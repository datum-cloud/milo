package server

import (
	"context"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"github.com/google/uuid"
	"go.datum.net/iam/internal/grpc/validation"
	"go.datum.net/iam/internal/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Server) SetIamPolicy(ctx context.Context, req *iampb.SetIamPolicyRequest) (*iampb.Policy, error) {
	policy := req.Policy
	policy.UpdateTime = timestamppb.Now()

	if errs := validation.ValidatePolicy(policy, validation.PolicyValidatorOptions{
		Context:         ctx,
		RoleResolver:    s.RoleResolver,
		SubjectResolver: s.SubjectResolver,
	}); len(errs) > 0 {
		return nil, errs.GRPCStatus().Err()
	}

	policyExists := false
	_, err := s.PolicyStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Policy.Name,
	})
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, err
	} else if err == nil {
		policyExists = true
	}

	if !policyExists {
		policy.Uid = uuid.NewString()
		policy, err = s.PolicyStorage.CreateResource(ctx, &storage.CreateResourceRequest[*iampb.Policy]{
			Name:     req.Policy.Name,
			Resource: policy,
		})
	} else {
		policy, err = s.PolicyStorage.UpdateResource(ctx, &storage.UpdateResourceRequest[*iampb.Policy]{
			Name: req.Policy.Name,
			Updater: func(existing *iampb.Policy) (new *iampb.Policy, err error) {
				return policy, nil
			},
		})
	}
	if err != nil {
		return nil, err
	}

	if err := s.PolicyReconciler.ReconcilePolicy(ctx, req.Policy.Name, policy); err != nil {
		return nil, err
	}

	return policy, nil
}

func (s *Server) GetIamPolicy(ctx context.Context, req *iampb.GetIamPolicyRequest) (*iampb.Policy, error) {
	existing, err := s.PolicyStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Name,
	})

	if status.Code(err) == codes.NotFound {
		// We intentionally return an empty policy here if one doesn't already
		// exist for the resource.
		return &iampb.Policy{}, nil
	} else if err != nil {
		return nil, err
	}

	return existing, nil
}
