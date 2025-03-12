package openfga

import (
	"context"
	"fmt"
	"strings"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	openfgav1 "github.com/openfga/api/proto/openfga/v1"
	"go.datum.net/iam/internal/schema"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func AccessChecker(registry *schema.Registry, client openfgav1.OpenFGAServiceClient, storeID string) func(ctx context.Context, req *iampb.CheckAccessRequest) (*iampb.CheckAccessResponse, error) {
	return func(ctx context.Context, req *iampb.CheckAccessRequest) (*iampb.CheckAccessResponse, error) {
		resourceReference, err := registry.ResolveResource(ctx, req.Resource)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "Could not resolve resource reference for resource '%s': %s", req.Resource, err)
		}

		checkReq := &openfgav1.CheckRequest{
			StoreId: storeID,
			TupleKey: &openfgav1.CheckRequestTupleKey{
				User:     "iam.datumapis.com/InternalUser:" + strings.TrimPrefix(req.Subject, "user:"),
				Relation: hashPermission(req.Permission),
				Object:   resourceReference.Type + ":" + resourceReference.Name,
			},
		}

		contextualTuples := []*openfgav1.TupleKey{}
		if resourceReference.Type != "iam.datumapis.com/Root" {
			// All resources will have the iam.datumspis.com/Root resource as its
			// parent. This allows permissions to be bound to the root of a resource
			// type to grant permissions across all resources.
			contextualTuples = append(contextualTuples, &openfgav1.TupleKey{
				User:     "iam.datumapis.com/Root:root/" + resourceReference.Type,
				Relation: "parent",
				Object:   resourceReference.Type + ":" + resourceReference.Name,
			})
		}

		for _, checkContext := range req.Context {
			// Parent and child relationships defined by the user are added as
			// contextual tuples so they can be used when evaluating the subjects access
			// to the resource. Permissions bound to a subject on a parent resource will
			// be inherited on on child resources.
			//
			// NOTE: WE could store these relationship tuples, to make it easier on the
			// caller, but this approach avoids services from informating the IAM system
			// of every resource's parent / child relationship as resources are created
			// and deleted.
			if checkContext.GetParentRelationship() != nil {
				parentResource, err := registry.ResolveResource(ctx, checkContext.GetParentRelationship().GetParentResource())
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "could not resolve parent resource '%s': %s", checkContext.GetParentRelationship().GetParentResource(), err)
				}

				childResource, err := registry.ResolveResource(ctx, checkContext.GetParentRelationship().GetChildResource())
				if err != nil {
					return nil, status.Errorf(codes.InvalidArgument, "could not resolve child resource '%s': %s", checkContext.GetParentRelationship().GetChildResource(), err)
				}

				contextualTuples = append(
					contextualTuples,
					&openfgav1.TupleKey{
						User:     parentResource.Type + ":" + parentResource.Name,
						Relation: "parent",
						Object:   childResource.Type + ":" + childResource.Name,
					},
					// Add another parent relationship for the root resource type of the
					// parent resource.
					&openfgav1.TupleKey{
						User:     "iam.datumapis.com/Root:root/" + parentResource.Type,
						Relation: "parent",
						Object:   parentResource.Type + ":" + parentResource.Name,
					},
				)
			}
		}

		if len(contextualTuples) > 0 {
			checkReq.ContextualTuples = &openfgav1.ContextualTupleKeys{
				TupleKeys: contextualTuples,
			}
		}

		resp, err := client.Check(ctx, checkReq)
		if err != nil {
			return nil, fmt.Errorf("failed to check access in OpenFGA: %s", err)
		}

		return &iampb.CheckAccessResponse{
			Allowed: resp.Allowed,
		}, nil
	}
}
