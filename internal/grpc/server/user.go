package server

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/mennanov/fmutils"
	"go.datum.net/iam/internal/grpc/longrunning"
	"go.datum.net/iam/internal/grpc/validation"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/subject"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func (s *Server) CreateUser(ctx context.Context, req *iampb.CreateUserRequest) (*longrunningpb.Operation, error) {
	user := req.User
	user.UserId = req.UserId

	if errs := validation.ValidateUser(user); len(errs) > 0 {
		return nil, errs.GRPCStatus().Err()
	}

	user.Uid = uuid.New().String()

	var userIdentifier string
	// If there is an userId, it had been validated previously
	if user.UserId != "" {
		userIdentifier = user.UserId
	} else {
		userIdentifier = user.Uid
	}
	user.Name = fmt.Sprintf("users/%s", userIdentifier)

	if user.DisplayName == "" {
		user.DisplayName = fmt.Sprintf("%s %s", user.Spec.GivenName, user.Spec.FamilyName)
	}

	now := timestamppb.Now()
	user.CreateTime = now
	user.UpdateTime = now

	// Check if the user already exists
	sub, err := s.SubjectResolver(ctx, subject.UserKind, user.Spec.Email)
	if err != nil {
		// If error is not "not found", don't return it an continue
		if !errors.Is(err, subject.ErrSubjectNotFound) {
			return nil, err
		}
	}
	if len(sub) > 0 {
		return nil, status.Errorf(codes.AlreadyExists, "user with email %s already exists", user.Spec.Email)
	}

	if req.ValidateOnly {
		return longrunning.ResponseOperation(&iampb.CreateUserMetadata{}, user, true)
	}

	createdUser, err := s.UserStorage.CreateResource(ctx, &storage.CreateResourceRequest[*iampb.User]{
		Resource: user,
		Name:     user.Name,
	})

	if err != nil {
		return nil, err
	}

	policy := &iampb.SetIamPolicyRequest{
		Policy: &iampb.Policy{
			Name: fmt.Sprintf("iam.datumapis.com/%s", user.Name),
			Spec: &iampb.PolicySpec{
				Bindings: []*iampb.Binding{{
					Role:    "services/iam.datumapis.com/roles/userSelfManage",
					Members: []string{fmt.Sprintf("user:%s", user.Spec.Email)},
				}},
			},
		},
	}

	_, err = s.SetIamPolicy(ctx, policy)
	if err != nil {
		return nil, err
	}

	return longrunning.ResponseOperation(&iampb.CreateUserMetadata{}, createdUser, true)
}

func (s *Server) GetUser(ctx context.Context, req *iampb.GetUserRequest) (*iampb.User, error) {
	return s.UserStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Name,
	})
}

func (s *Server) SetUserProviderId(ctx context.Context, req *iampb.SetUserProviderIdRequest) (*iampb.SetUserProviderIdResponse, error) {
	userEmail := strings.TrimPrefix(req.Name, "users/")
	// Create an update mask with the "annotations" path
	updateMask := &fieldmaskpb.FieldMask{
		Paths: []string{"annotations"},
	}

	userUpdates := &iampb.User{
		Annotations: map[string]string{
			// TODO: refactor to get the provider key from a config
			validation.UsersAnnotationValidator.GetProviderKey(): req.ProviderId,
		},
	}

	// Getting the user uid from the email
	sub, err := s.SubjectResolver(ctx, subject.UserKind, userEmail)
	if err != nil {
		if errors.Is(err, subject.ErrSubjectNotFound) {
			return nil, status.Errorf(codes.NotFound, "user with email %s not found", userEmail)
		}
		return nil, err
	}

	resourceName := fmt.Sprintf("users/%s", sub)

	if req.ValidateOnly {
		existing, err := s.UserStorage.GetResource(ctx, &storage.GetResourceRequest{
			Name: resourceName,
		})
		if err != nil {
			return nil, err
		}

		fmutils.Overwrite(userUpdates, existing, updateMask.Paths)

		if errs := validation.ValidateUser(existing); len(errs) > 0 {
			return nil, errs.GRPCStatus().Err()
		}

		existing.UpdateTime = timestamppb.Now()

		return &iampb.SetUserProviderIdResponse{User: existing}, nil
	}

	// Update the user
	updatedUser, err := s.UserStorage.UpdateResource(ctx, &storage.UpdateResourceRequest[*iampb.User]{
		Name: resourceName,
		Updater: func(existing *iampb.User) (new *iampb.User, err error) {
			fmutils.Overwrite(userUpdates, existing, updateMask.Paths)

			if errs := validation.ValidateUser(existing); len(errs) > 0 {
				return nil, errs.GRPCStatus().Err()
			}

			return existing, nil
		},
	})
	if err != nil {
		return nil, err
	}

	return &iampb.SetUserProviderIdResponse{User: updatedUser}, nil
}

func (s *Server) UpdateUser(ctx context.Context, req *iampb.UpdateUserRequest) (*longrunningpb.Operation, error) {
	userUpdates := req.User
	updateMask := req.UpdateMask
	// TODO: refactor to get the provider key from a config
	providerKey := validation.UsersAnnotationValidator.GetProviderKey()

	// TODO: refactor to only set the mutable paths, and on app initialization,
	// retrieve the immutable paths from the from the *iampb.User resource
	immutablePaths := []string{
		"name",
		"user_id",
		"uid",
		"spec.email",
		"create_time",
		"update_time",
		"delete_time",
		"reconciling",
	}

	updaterUserFunc := func(existing *iampb.User) (new *iampb.User, err error) {
		providerId := existing.Annotations[providerKey]
		existingUserCopy := proto.Clone(existing).(*iampb.User)

		fmutils.Overwrite(userUpdates, existing, updateMask.Paths)

		if errs := validation.ValidateUserUpdate(immutablePaths, existingUserCopy, existing, req); len(errs) > 0 {
			return nil, errs.GRPCStatus().Err()
		}

		// Reassign the providerId to the user in case there was no annotation path
		existing.Annotations[providerKey] = providerId

		if errs := validation.ValidateUser(existing); len(errs) > 0 {
			return nil, errs.GRPCStatus().Err()
		}

		return existing, nil
	}

	resourceName := req.User.Name

	if req.ValidateOnly {
		existing, err := s.UserStorage.GetResource(ctx, &storage.GetResourceRequest{
			Name: resourceName,
		})
		if err != nil {
			return nil, err
		}

		updatedUser, err := updaterUserFunc(existing)
		if err != nil {
			return nil, err
		}

		existing.UpdateTime = timestamppb.Now()

		return longrunning.ResponseOperation(&iampb.CreateUserMetadata{}, updatedUser, true)
	}

	// Update the user
	updatedUser, err := s.UserStorage.UpdateResource(ctx, &storage.UpdateResourceRequest[*iampb.User]{
		Name:    resourceName,
		Updater: updaterUserFunc,
	})
	if err != nil {
		return nil, err
	}

	return longrunning.ResponseOperation(&iampb.UpdateUserMetadata{}, updatedUser, true)
}

func (s *Server) ListUsers(ctx context.Context, req *iampb.ListUsersRequest) (*iampb.ListUsersResponse, error) {
	if errs := validation.ValidateListUsersRequest(req); len(errs) > 0 {
		return nil, errs.GRPCStatus().Err()
	}

	users, err := s.UserStorage.ListResources(ctx, &storage.ListResourcesRequest{
		PageSize:       req.PageSize,
		PageToken:      req.PageToken,
		Filter:         req.Filter,
		IncludeDeleted: req.ShowDeleted,
	})
	if err != nil {
		return nil, err
	}

	return &iampb.ListUsersResponse{
		Users:         users.Resources,
		NextPageToken: users.NextPageToken,
	}, nil
}

func (s *Server) DeleteUser(ctx context.Context, req *iampb.DeleteUserRequest) (*longrunningpb.Operation, error) {
	deletedUser, err := s.UserStorage.DeleteResource(ctx, &storage.DeleteResourceRequest{
		Name: req.Name,
	})
	if err != nil {
		return nil, err
	}

	err = s.AuthenticationProvider.DeleteUser(ctx, deletedUser)
	if err != nil {
		return nil, err
	}

	return longrunning.ResponseOperation(&iampb.UpdateUserMetadata{}, deletedUser, true)
}
