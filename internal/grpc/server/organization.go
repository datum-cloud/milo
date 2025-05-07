package server

import (
	"context"
	"fmt"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/google/uuid"
	"go.datum.net/iam/internal/grpc/longrunning"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	resourcemanagerpb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/resourcemanager/v1alpha"
	"github.com/mennanov/fmutils"
	"go.datum.net/iam/internal/grpc/validation"
	"go.datum.net/iam/internal/storage"
)

func (s *Server) GetOrganization(ctx context.Context, req *resourcemanagerpb.GetOrganizationRequest) (*resourcemanagerpb.Organization, error) {
	return s.OrganizationStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Name,
	})
}

func (s *Server) CreateOrganization(ctx context.Context, req *resourcemanagerpb.CreateOrganizationRequest) (*longrunningpb.Operation, error) {
	organization := req.Organization
	organization.OrganizationId = req.OrganizationId

	if errs := validation.ValidateOrganization(organization); len(errs) > 0 {
		return nil, errs.GRPCStatus().Err()
	}

	organization.Uid = uuid.New().String()

	var organizationIdentifier string
	if organization.OrganizationId != "" {
		organizationIdentifier = organization.OrganizationId
	} else {
		organizationIdentifier = organization.Uid
	}
	organization.Name = fmt.Sprintf("organizations/%s", organizationIdentifier)

	if req.ValidateOnly {
		return longrunning.ResponseOperation(&resourcemanagerpb.CreateOrganizationMetadata{}, organization, true)
	}

	now := timestamppb.Now()
	organization.CreateTime = now
	organization.UpdateTime = now

	userId, err := s.SubjectExtractor(ctx)
	if err != nil {
		return nil, err
	}

	userName := fmt.Sprintf("users/%s", userId)
	user, err := s.UserStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: userName,
	})
	if err != nil {
		return nil, err
	}

	createdOrganization, err := s.OrganizationStorage.CreateResource(ctx, &storage.CreateResourceRequest[*resourcemanagerpb.Organization]{
		Resource: organization,
		Name:     organization.Name,
	})
	if err != nil {
		return nil, err
	}

	policy := &iampb.SetIamPolicyRequest{
		Policy: &iampb.Policy{
			Name: fmt.Sprintf("iam.datumapis.com/%s", organization.Name),
			Spec: &iampb.PolicySpec{
				Bindings: []*iampb.Binding{{
					Role:    "services/iam.datumapis.com/roles/organizationManager",
					Members: []string{fmt.Sprintf("user:%s", user.Spec.Email)},
				}},
			},
		},
	}

	_, err = s.SetIamPolicy(ctx, policy)
	if err != nil {
		return nil, err
	}

	return longrunning.ResponseOperation(&resourcemanagerpb.CreateOrganizationMetadata{}, createdOrganization, true)
}

func (s *Server) DeleteOrganization(ctx context.Context, req *resourcemanagerpb.DeleteOrganizationRequest) (*longrunningpb.Operation, error) {
	if req.ValidateOnly {
		organization, err := s.OrganizationStorage.GetResource(ctx, &storage.GetResourceRequest{
			Name: req.Name,
		})
		if err != nil {
			return nil, err
		}

		return longrunning.ResponseOperation(&resourcemanagerpb.DeleteOrganizationMetadata{}, organization, true)
	}

	deletedOrganization, err := s.OrganizationStorage.DeleteResource(ctx, &storage.DeleteResourceRequest{
		Name: req.Name,
	})
	if err != nil {
		return nil, err
	}

	return longrunning.ResponseOperation(&resourcemanagerpb.DeleteOrganizationMetadata{}, deletedOrganization, true)
}

func (s *Server) UpdateOrganization(ctx context.Context, req *resourcemanagerpb.UpdateOrganizationRequest) (*longrunningpb.Operation, error) {
	updaterOrganizationFunc := func(existing *resourcemanagerpb.Organization) (new *resourcemanagerpb.Organization, err error) {
		existingOrganizationCopy := proto.Clone(existing).(*resourcemanagerpb.Organization)

		fmutils.Overwrite(req.Organization, existing, req.UpdateMask.Paths)
		if errs := validation.AssertImmutableFieldsUnchanged(req.UpdateMask.Paths, existingOrganizationCopy, existing); len(errs) > 0 {
			return nil, errs.GRPCStatus().Err()
		}
		if errs := validation.ValidateOrganization(existing); len(errs) > 0 {
			return nil, errs.GRPCStatus().Err()
		}

		return existing, nil
	}

	if req.ValidateOnly {
		existing, err := s.OrganizationStorage.GetResource(ctx, &storage.GetResourceRequest{
			Name: req.Organization.Name,
		})
		if err != nil {
			return nil, err
		}

		updatedOrganization, err := updaterOrganizationFunc(existing)
		if err != nil {
			return nil, err
		}
		existing.UpdateTime = timestamppb.Now()

		return longrunning.ResponseOperation(&resourcemanagerpb.UpdateOrganizationMetadata{}, updatedOrganization, true)
	}

	updatedOrganization, err := s.OrganizationStorage.UpdateResource(ctx, &storage.UpdateResourceRequest[*resourcemanagerpb.Organization]{
		Name:    req.Organization.Name,
		Updater: updaterOrganizationFunc,
	})
	if err != nil {
		return nil, err
	}

	return longrunning.ResponseOperation(&resourcemanagerpb.DeleteOrganizationMetadata{}, updatedOrganization, true)
}
