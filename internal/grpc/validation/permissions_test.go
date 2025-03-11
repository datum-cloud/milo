package validation_test

import (
	"context"
	"testing"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"go.datum.net/iam/internal/grpc/validation"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/validation/field"
	"google.golang.org/protobuf/encoding/protojson"
)

type serviceGetterFunc func(ctx context.Context, req *storage.GetResourceRequest) (*iampb.Service, error)

func (s serviceGetterFunc) GetResource(ctx context.Context, req *storage.GetResourceRequest) (*iampb.Service, error) {
	return s(ctx, req)
}

func TestPermissionValidation(t *testing.T) {
	validator := validation.NewPermissionValidator(serviceGetterFunc(func(ctx context.Context, req *storage.GetResourceRequest) (*iampb.Service, error) {
		return &iampb.Service{
			Name:      "services/library.example.com",
			ServiceId: "library.example.com",
			Spec: &iampb.ServiceSpec{
				Resources: []*iampb.Resource{
					{
						Type:     "library.example.com/Branch",
						Singular: "branch",
						Plural:   "branches",
						ResourceNamePatterns: []string{
							"branches/{branch}",
						},
						Permissions: []string{"create"},
					},
				},
			},
		}, nil
	}))

	testCases := []struct {
		desc               string
		permission         string
		valid              bool
		expectedViolations []*field.Error
	}{
		{
			desc:       "Valid permission",
			permission: "library.example.com/branches.create",
			valid:      true,
		},
		{
			desc:       "invalid permission format",
			permission: "my-permission",
			valid:      false,
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {
			errs := validator(field.NewPath("test"), tC.permission)
			if tC.valid {
				if len(errs) > 0 {
					t.Errorf("expected permission '%s' to be valid, but got errs %s", tC.permission, protojson.Format(errs.GRPCStatus().Proto()))
				}
			} else if !tC.valid {
				if len(errs) == 0 {
					t.Errorf("expected permission '%s' to not be valid but no errors were returned", tC.permission)
				}
			}
		})
	}
}
