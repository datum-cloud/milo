package openfga

import (
	"context"
	"fmt"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"

	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"go.datum.net/iam/internal/storage"
)

type RoleReconciler struct {
	StoreID     string
	Client      openfgav1.OpenFGAServiceClient
	RoleStorage storage.ResourceServer[*iampb.Role]
}

func (r *RoleReconciler) getAllPermissions(ctx context.Context, role *iampb.Role, visited map[string]struct{}) ([]string, error) {
	if visited == nil {
		visited = make(map[string]struct{})
	}
	if _, ok := visited[role.Name]; ok {
		return nil, nil // Prevent cycles
	}
	visited[role.Name] = struct{}{}

	permissions := append([]string{}, role.Spec.IncludedPermissions...)

	for _, inheritedRoleName := range role.Spec.InheritedRoles {

		inheritedRole, err := r.RoleStorage.GetResource(ctx, &storage.GetResourceRequest{
			Name: inheritedRoleName,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get inherited role %s: %w", inheritedRoleName, err)
		}

		inheritedPerms, err := r.getAllPermissions(ctx, inheritedRole, visited)
		if err != nil {
			return nil, err
		}

		permissions = append(permissions, inheritedPerms...)
	}

	return permissions, nil
}

func (r *RoleReconciler) ReconcileRole(ctx context.Context, role *iampb.Role) error {
	var expectedTuples []*openfgav1.TupleKey

	existingTupleKeys, err := getTupleKeys(ctx, r.StoreID, r.Client, &openfgav1.ReadRequestTupleKey{
		Object: "iam.datumapis.com/InternalRole:" + role.Name,
	})
	if err != nil {
		return fmt.Errorf("failed to get existing tuples: %w", err)
	}

	allPermissions, err := r.getAllPermissions(ctx, role, nil)
	if err != nil {
		return fmt.Errorf("failed to collect permissions: %w", err)
	}

	for _, permission := range allPermissions {
		expectedTuples = append(
			expectedTuples,
			&openfgav1.TupleKey{
				User:     "iam.datumapis.com/InternalUser:*",
				Relation: hashPermission(permission),
				Object:   "iam.datumapis.com/InternalRole:" + role.Name,
			},
		)
	}

	added, removed := diffTuples(existingTupleKeys, expectedTuples)

	// Don't do anything if there's no changes to make.
	if len(added) == 0 && len(removed) == 0 {
		return nil
	}

	req := &openfgav1.WriteRequest{
		StoreId: r.StoreID,
	}

	if len(removed) > 0 {
		req.Deletes = &openfgav1.WriteRequestDeletes{
			TupleKeys: convertTuplesForDelete(removed),
		}
	}

	if len(added) > 0 {
		req.Writes = &openfgav1.WriteRequestWrites{
			TupleKeys: added,
		}
	}

	_, err = r.Client.Write(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to reconcile roles: %w", err)
	}

	return nil
}

func (r *RoleReconciler) DeleteRole(ctx context.Context, role *iampb.Role) error {
	existingTupleKeys, err := getTupleKeys(ctx, r.StoreID, r.Client, &openfgav1.ReadRequestTupleKey{
		Object: "iam.datumapis.com/InternalRole:" + role.Name,
	})
	if err != nil {
		return fmt.Errorf("failed to get existing tuples: %w", err)
	}

	if len(existingTupleKeys) == 0 {
		return nil
	}

	_, err = r.Client.Write(ctx, &openfgav1.WriteRequest{
		StoreId: r.StoreID,
		Deletes: &openfgav1.WriteRequestDeletes{
			TupleKeys: convertTuplesForDelete(existingTupleKeys),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete role: %w", err)
	}
	return nil
}
