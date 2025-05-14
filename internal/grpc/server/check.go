package server

import (
	"context"
	"fmt"
	"log/slog"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/subject"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	// schema "go.datum.net/iam/internal/schema" // No longer needed if s.SchemaRegistry is used directly and types are compatible
)

func (s *Server) CheckAccess(ctx context.Context, req *iampb.CheckAccessRequest) (*iampb.CheckAccessResponse, error) {
	resolveCtx, span := otel.Tracer("").Start(ctx, "datum.server.CheckAccess.resolveParents")
	defer span.End()

	subjectID, _, err := subject.Parse(req.Subject)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subject: %w", err)
	}

	subjectReference, err := s.SubjectResolver(resolveCtx, subjectID)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve subject: %w", err)
	}

	// The subject ID within the authorization engine is the resource name of
	// the subject so it's guaranteed to be unique within the authorization
	// engine.
	req.Subject = subjectReference.ResourceName

	// Use schema.Registry to resolve the resource URL into its components including type.
	schemaRef, err := s.SchemaRegistry.ResolveResource(resolveCtx, req.Resource)
	if err != nil {
		slog.ErrorContext(resolveCtx, "Failed to resolve resource using schema registry", "error", err, "resource", req.Resource)
		span.SetStatus(codes.Error, "Failed to resolve resource via schema registry")
		return nil, fmt.Errorf("failed to resolve resource '%s' via schema registry: %w", req.Resource, err)
	}

	currentResourceRef := &storage.ResourceReference{
		Name:     storage.ResourceName(schemaRef.Name), // Cast string to storage.ResourceName type if it exists and is used by ParentResolver
		Type:     schemaRef.Type,
		SelfLink: schemaRef.SelfLink, // This should be equivalent to req.Resource
	}

	slog.InfoContext(resolveCtx, "Attempting to resolve parents for", slog.Any("resource", currentResourceRef))

	var resolvedParentRelationships []*iampb.ParentRelationship
	loopResource := currentResourceRef
	for {
		parentStorageRef, err := s.ParentResolver.ResolveParent(resolveCtx, loopResource)
		if err != nil {
			span.SetStatus(codes.Error, err.Error())
			slog.ErrorContext(resolveCtx, "Error resolving parent", slog.Any("resource", loopResource), "error", err)
			return nil, fmt.Errorf("failed to resolve parent for %s: %w", loopResource.SelfLink, err)
		} else if parentStorageRef == nil {
			slog.DebugContext(resolveCtx, "Resource does not have a parent or no more parents", slog.Any("resource", loopResource))
			break
		}

		slog.DebugContext(resolveCtx, "Found parent", slog.Any("parent", parentStorageRef), slog.Any("child", loopResource))
		resolvedParentRelationships = append(resolvedParentRelationships, &iampb.ParentRelationship{
			ParentResource: parentStorageRef.SelfLink,
			ChildResource:  loopResource.SelfLink,
		})

		loopResource = parentStorageRef
	}
	span.SetAttributes(attribute.Int("resolved_parent_count", len(resolvedParentRelationships)))

	if len(resolvedParentRelationships) > 0 {
		for _, p := range resolvedParentRelationships {
			req.Context = append(req.Context, &iampb.CheckContext{
				ContextType: &iampb.CheckContext_ParentRelationship{
					ParentRelationship: p,
				},
			})
		}
		slog.InfoContext(resolveCtx, "Augmented CheckAccessRequest with parent relationships", slog.Int("count", len(resolvedParentRelationships)))
	}

	return s.AccessChecker(ctx, req)
}
