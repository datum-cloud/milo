package server

import (
	"context"
	"fmt"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	resourcemanagerpb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/resourcemanager/v1alpha"
	"go.datum.net/iam/internal/grpc/longrunning"
	"go.datum.net/iam/internal/grpc/validation"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/subject"
	"go.datum.net/iam/internal/validation/field"

	"cloud.google.com/go/longrunning/autogen/longrunningpb"
	"github.com/google/uuid"
	"github.com/mennanov/fmutils"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (s *Server) CreateInvitation(ctx context.Context, req *resourcemanagerpb.CreateInvitationRequest) (*longrunningpb.Operation, error) {
	invitation := req.Invitation
	invitation.Uid = uuid.New().String()
	invitation.InvitationId = req.InvitationId
	invitation.Spec.Parent = req.Parent

	invitationRecipientSub, err := s.SubjectResolver(ctx, subject.UserKind, invitation.Spec.RecipientEmailAddress)
	if err != nil {
		return nil, err
	}

	// Check if the user is already a member of the organization
	resp, err := s.CheckAccess(ctx, &iampb.CheckAccessRequest{
		Resource:   fmt.Sprintf("resourcemanager.datumapis.com/%s", req.Parent),
		Permission: "resourcemanager.datumapis.com/organizations.get",
		Subject:    invitationRecipientSub,
	})
	if err != nil {
		return nil, err
	}
	if resp.Allowed {
		return nil, status.Errorf(codes.InvalidArgument, "user %s is already a member of the organization %s", invitation.Spec.RecipientEmailAddress, req.Parent)
	}

	if invitation.Spec.ExpirationTime == nil {
		// Make expiration time to be in 14 days from now
		invitation.Spec.ExpirationTime = &timestamppb.Timestamp{
			Seconds: timestamppb.Now().GetSeconds() + DEFAULT_INVITATION_EXPIRATION_TIME,
		}
	}

	if errs := validation.ValidateInvitation(invitation, req.Parent); len(errs) > 0 {
		return nil, errs.GRPCStatus().Err()
	}

	var invitationIdentifier string
	if invitation.InvitationId != "" {
		invitationIdentifier = invitation.InvitationId
	} else {
		invitationIdentifier = invitation.Uid
	}
	invitation.Name = fmt.Sprintf("%s/invitations/%s", req.Parent, invitationIdentifier)

	// Check that the roles exists
	errors := field.ErrorList{}
	for _, role := range invitation.Spec.Roles {
		_, err := s.RoleStorage.GetResource(ctx, &storage.GetResourceRequest{
			Name: role,
		})
		if err != nil {
			errors = append(errors, field.NotFound(field.NewPath("spec").Child("roles"), role))
		}
	}
	if len(errors) > 0 {
		return nil, errors.GRPCStatus().Err()
	}

	now := timestamppb.Now()
	invitation.CreateTime = now
	invitation.UpdateTime = now

	if req.ValidateOnly {
		return longrunning.ResponseOperation(&resourcemanagerpb.CreateInvitationMetadata{}, invitation, true)
	}

	// Create invitation
	_, err = s.InvitationStorage.CreateResource(ctx, &storage.CreateResourceRequest[*resourcemanagerpb.Invitation]{
		Resource: invitation,
		Name:     invitation.Name,
		Parent:   req.Parent,
	})
	if err != nil {
		return nil, err
	}

	// Set the IAM policy to the user so they can accept the invitation
	policy := &iampb.SetIamPolicyRequest{
		Policy: &iampb.Policy{
			Name: fmt.Sprintf("resourcemanager.datumapis.com/%s", invitation.Name),
			Spec: &iampb.PolicySpec{
				Bindings: []*iampb.Binding{{
					Role:    "services/resourcemanager.datumapis.com/roles/invitationResponder",
					Members: []string{fmt.Sprintf("user:%s", invitation.Spec.RecipientEmailAddress)},
				}},
			},
		},
	}

	_, err = s.SetIamPolicy(ctx, policy)
	if err != nil {
		return nil, err
	}

	// Set invitation state to SENT
	updatedInvitation, err := s.InvitationStorage.UpdateResource(ctx, &storage.UpdateResourceRequest[*resourcemanagerpb.Invitation]{
		Name: invitation.Name,
		Updater: func(existing *resourcemanagerpb.Invitation) (new *resourcemanagerpb.Invitation, err error) {
			invitationUpdates := &resourcemanagerpb.Invitation{
				State: resourcemanagerpb.InvitationState_INVITATION_SENT,
			}
			updateMask := &fieldmaskpb.FieldMask{
				Paths: []string{"state"},
			}
			fmutils.Overwrite(invitationUpdates, existing, updateMask.Paths)

			return existing, nil
		},
	})
	if err != nil {
		return nil, err
	}

	return longrunning.ResponseOperation(&resourcemanagerpb.CreateInvitationMetadata{}, updatedInvitation, true)
}

func (s *Server) GetInvitation(ctx context.Context, req *resourcemanagerpb.GetInvitationRequest) (*resourcemanagerpb.Invitation, error) {
	return s.InvitationStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Name,
	})
}

func (s *Server) AcceptInvitation(ctx context.Context, req *resourcemanagerpb.AcceptInvitationRequest) (*resourcemanagerpb.AcceptInvitationResponse, error) {
	// Getting the invitation
	invitation, err := s.InvitationStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: req.Name,
	})
	if err != nil {
		return nil, err
	}

	// Check if invitation can be accepted
	if invitation.State != resourcemanagerpb.InvitationState_INVITATION_SENT {
		return nil, status.Errorf(codes.InvalidArgument, "invitation %s cannot be accepted. Current state is %s", req.Name, invitation.State)
	}

	// Check if invitation is expired
	if invitation.Spec.ExpirationTime.AsTime().Before(timestamppb.Now().AsTime()) {
		return nil, status.Errorf(codes.InvalidArgument, "invitation %s is expired", req.Name)
	}

	organization, err := s.OrganizationStorage.GetResource(ctx, &storage.GetResourceRequest{
		Name: invitation.Spec.Parent,
	})
	if err != nil {
		return nil, err
	}

	if req.ValidateOnly {
		return &resourcemanagerpb.AcceptInvitationResponse{Organization: organization}, nil
	}

	// Setting the IAM policies to the user
	for _, role := range invitation.Spec.Roles {
		policy := &iampb.SetIamPolicyRequest{
			Policy: &iampb.Policy{
				Name: fmt.Sprintf("resourcemanager.datumapis.com/%s", organization.Name),
				Spec: &iampb.PolicySpec{
					Bindings: []*iampb.Binding{{
						Role:    role,
						Members: []string{fmt.Sprintf("user:%s", invitation.Spec.RecipientEmailAddress)},
					}},
				},
			},
		}

		_, err = s.SetIamPolicy(ctx, policy)
		if err != nil {
			return nil, err
		}
	}

	// Set invitation state to ACCEPTED
	_, err = s.InvitationStorage.UpdateResource(ctx, &storage.UpdateResourceRequest[*resourcemanagerpb.Invitation]{
		Name: invitation.Name,
		Updater: func(existing *resourcemanagerpb.Invitation) (new *resourcemanagerpb.Invitation, err error) {
			invitationUpdates := &resourcemanagerpb.Invitation{
				State: resourcemanagerpb.InvitationState_INVITATION_ACCEPTED,
			}
			updateMask := &fieldmaskpb.FieldMask{
				Paths: []string{"state"},
			}
			fmutils.Overwrite(invitationUpdates, existing, updateMask.Paths)

			return existing, nil
		},
	})
	if err != nil {
		return nil, err
	}

	return &resourcemanagerpb.AcceptInvitationResponse{Organization: organization}, nil
}
