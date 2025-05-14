package server

import (
	"context"
	"fmt"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/google/uuid"
	"github.com/mennanov/fmutils"
	"go.datum.net/iam/internal/grpc/longrunning"
	"go.datum.net/iam/internal/grpc/validation"
	roleResolver "go.datum.net/iam/internal/role"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/validation/field"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Server) CreateRole(ctx context.Context, req *iampb.CreateRoleRequest) (*longrunningpb.Operation, error) {
	role := req.Role

	// Validate the role
	if errs := validation.ValidateRole(role, &validation.RoleValidatorOptions{
		PermissionValidator: validation.NewPermissionValidator(s.ServiceStorage),
		RoleValidator:       validation.NewRoleValidator(s.RoleStorage),
	}); len(errs) != 0 {
		return nil, errs.GRPCStatus().Err()
	}

	if req.ValidateOnly {
		return longrunning.ResponseOperation(&iampb.CreateRoleMetadata{}, role, true)
	}

	role.RoleId = req.RoleId
	role.Uid = uuid.New().String()
	role.CreateTime = timestamppb.Now()
	role.Parent = req.Parent
	role.Name = fmt.Sprintf("%s/roles/%s", req.Parent, req.RoleId)

	// TODO: Add support for additional parent types (e.g. Organizations and
	//       Projects)
	//
	// Verify that the service the role is being created in exists.
	if _, err := s.ServiceStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Parent,
	}); err != nil {
		return nil, err
	}

	createdRole, err := s.RoleStorage.CreateResource(ctx, &storage.CreateResourceRequest[*iampb.Role]{
		Name:     role.Name,
		Parent:   role.Parent,
		Resource: role,
	})
	if err != nil {
		return nil, err
	}

	if err := s.RoleReconciler.ReconcileRole(ctx, createdRole); err != nil {
		return nil, fmt.Errorf("failed to reconcile role: %w", err)
	}

	return longrunning.ResponseOperation(&iampb.CreateRoleMetadata{}, role, true)
}

func (s *Server) ListRoles(ctx context.Context, req *iampb.ListRolesRequest) (*iampb.ListRolesResponse, error) {
	roles, err := s.RoleStorage.ListResources(ctx, &storage.ListResourcesRequest{
		Parent:    req.Parent,
		PageSize:  req.PageSize,
		PageToken: req.PageToken,
		Filter:    req.Filter,
	})
	if err != nil {
		return nil, err
	}

	return &iampb.ListRolesResponse{
		Roles:         roles.Resources,
		NextPageToken: roles.NextPageToken,
	}, nil
}

func (s *Server) GetRole(ctx context.Context, req *iampb.GetRoleRequest) (*iampb.Role, error) {
	return s.RoleStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Name,
	})
}

func (s *Server) UpdateRole(ctx context.Context, req *iampb.UpdateRoleRequest) (*longrunningpb.Operation, error) {
	// TODO: Add support for allow_missing
	updatedRole, err := s.RoleStorage.UpdateResource(ctx, &storage.UpdateResourceRequest[*iampb.Role]{
		Name: req.Role.Name,
		Updater: func(existing *iampb.Role) (new *iampb.Role, err error) {
			// Apply the update to the existing role and only update paths that were
			// provided in the field mask.
			fmutils.Overwrite(req.Role, existing, req.UpdateMask.Paths)

			// Validate the updated role
			if errs := validation.ValidateRole(existing, &validation.RoleValidatorOptions{
				PermissionValidator: validation.NewPermissionValidator(s.ServiceStorage),
			}); len(errs) != 0 {
				return nil, errs.GRPCStatus().Err()
			}
			return existing, nil
		},
	})
	if err != nil {
		return nil, err
	}

	if err := s.RoleReconciler.ReconcileRole(ctx, updatedRole); err != nil {
		return nil, err
	}

	errs := s.reconcileInheritedRoles(ctx, updatedRole.Name)
	if len(errs) > 0 {
		return nil, errs.GRPCStatus().Err()
	}

	return longrunning.ResponseOperation(&iampb.UpdateRoleMetadata{}, updatedRole, true)
}

func (s *Server) DeleteRole(ctx context.Context, req *iampb.DeleteRoleRequest) (*longrunningpb.Operation, error) {
	if inUse, err := s.roleInUse(ctx, req.Name); err != nil {
		return nil, err
	} else if inUse {
		return nil, status.Errorf(codes.FailedPrecondition, "Role '%s' is still bound to subjects in IAM policies. Usage of role must be removed before the role can be deleted.", req.Name)
	}

	existingRole, err := s.RoleStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Name,
	})
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, err
	}

	if req.ValidateOnly {
		// TODO: Should we return the existing role here? Should probably also
		//       perform validation on the etag value if it's provided.
		return longrunning.ResponseOperation(&iampb.DeleteRoleMetadata{}, existingRole, true)
	}

	role, err := s.RoleStorage.DeleteResource(ctx, &storage.DeleteResourceRequest{
		Name: req.Name,
		Etag: req.Etag,
	})
	if status.Code(err) == codes.NotFound && req.AllowMissing {
		return longrunning.ResponseOperation(&iampb.DeleteRoleMetadata{}, existingRole, true)
	} else if err != nil {
		return nil, err
	}

	return longrunning.ResponseOperation(&iampb.DeleteRoleMetadata{}, role, true)
}

func (s *Server) roleInUse(ctx context.Context, role string) (bool, error) {
	rolesThatDependOnThisRole, err := s.DatabaseRoleResolver(ctx, roleResolver.InheritedRoleKind, role)
	if err != nil {
		return false, err
	}
	if len(rolesThatDependOnThisRole) > 0 {
		return true, nil
	}

	var pageToken string
	for {
		policies, err := s.PolicyStorage.ListResources(ctx, &storage.ListResourcesRequest{
			PageSize:  1000,
			PageToken: pageToken,
		})
		if err != nil {
			return false, err
		}

		for _, policy := range policies.Resources {
			for _, binding := range policy.GetSpec().GetBindings() {
				if binding.Role == role {
					return true, nil
				}
			}
		}

		// Check to see if there's any additional policies to retrieve.
		if policies.NextPageToken == "" {
			return false, nil
		}

		pageToken = policies.NextPageToken
	}
}

// reconcileInheritedRoles recursively finds all roles that inherit from the given role name (directly or indirectly),
// reconciles them, and collects all errors encountered during reconciliation.
func (s *Server) reconcileInheritedRoles(ctx context.Context, roleName string) field.ErrorList {
	errs := field.ErrorList{}
	visited := make(map[string]struct{})
	counter := 0
	const maxRecursion = 9999

	var process func(string)
	process = func(parentRole string) {
		counter++
		if counter >= maxRecursion {
			errs = append(errs, field.InternalError(field.NewPath("reconcileInheritedRoles"), fmt.Errorf("maximum recursion limit (%d) reached, possible cycle or excessive depth", maxRecursion)))
			return
		}

		// Use the DatabaseRoleResolver to find all roles that inherit from parentRole
		childRoles, err := s.DatabaseRoleResolver(ctx, roleResolver.InheritedRoleKind, parentRole)
		if err != nil {
			errs = append(errs, field.InternalError(field.NewPath("DatabaseRoleResolver"), err))
			return
		}

		for _, childRoleName := range childRoles {
			if _, ok := visited[childRoleName]; ok {
				continue // already processed this role name
			}
			visited[childRoleName] = struct{}{}

			// Fetch the full role object by name
			roleObj, err := s.RoleStorage.GetResource(ctx, &storage.GetResourceRequest{
				Name: childRoleName,
			})
			if err != nil {
				errs = append(errs, field.InternalError(field.NewPath("RoleStorage.GetResource").Child(childRoleName), err))
				continue
			}

			if recErr := s.RoleReconciler.ReconcileRole(ctx, roleObj); recErr != nil {
				errs = append(errs, field.InternalError(field.NewPath("RoleReconciler.ReconcileRole").Child(childRoleName), recErr))
			}

			// Recursively process roles that inherit from this child role
			process(childRoleName)
		}
	}

	// Mark the initial role as visited to avoid reprocessing if it appears as a child
	visited[roleName] = struct{}{}
	process(roleName)
	return errs
}
