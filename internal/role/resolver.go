package role

import (
	"context"

	"buf.build/gen/go/datum-cloud/iam/grpc/go/datum/iam/v1alpha/iamv1alphagrpc"
	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"go.datum.net/iam/internal/subject"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RoleResolver will validate that a role can be used within an IAM policy.
type Resolver func(ctx context.Context, roleName string) error

// IAMUseRoleResolver creates a role resolver that confirms the authorized user
// in the context has access to use an IAM role.
func IAMUseRoleResolver(client iamv1alphagrpc.AccessCheckClient, subjectExtractor subject.Extractor) Resolver {
	return func(ctx context.Context, roleName string) error {
		// TODO: Replace by having the access check endpoint determine the subject
		//       from the authenticated user.
		subject, err := subjectExtractor(ctx)
		if err != nil {
			return err
		}

		resp, err := client.CheckAccess(ctx, &iampb.CheckAccessRequest{
			Subject:    subject,
			Resource:   "iam.datumapis.com/" + roleName,
			Permission: "iam.datumapis.com/roles.use",
		})
		if err != nil {
			return err
		} else if !resp.Allowed {
			return status.Newf(codes.PermissionDenied, "user does not have permission to use role '%s'", roleName).Err()
		}
		return nil
	}
}
