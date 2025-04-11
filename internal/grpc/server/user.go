package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/timestamppb"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"go.datum.net/iam/internal/grpc/longrunning"
	"go.datum.net/iam/internal/grpc/validation"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/subject"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
