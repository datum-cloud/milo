package otelstorage

import (
	"context"

	"go.datum.net/iam/internal/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

func WithTracing[R storage.Resource](srv storage.ResourceServer[R]) storage.ResourceServer[R] {
	return &tracer[R]{
		srv: srv,
	}
}

type tracer[R storage.Resource] struct {
	srv  storage.ResourceServer[R]
	zero R
}

func (t *tracer[R]) CreateResource(ctx context.Context, req *storage.CreateResourceRequest[R]) (R, error) {
	ctx, span := otel.Tracer("").Start(ctx, "datum.storage.CreateResource", trace.WithAttributes(
		t.resourceAttributes(
			attribute.String("storage.resources.datumapis.go/resource_name", req.Name),
			attribute.String("storage.resources.datumapis.go/resource_parent", req.Parent),
		)...,
	))
	defer span.End()

	resource, err := t.srv.CreateResource(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return resource, err
}

func (t *tracer[R]) resourceAttributes(additional ...attribute.KeyValue) []attribute.KeyValue {
	return append(
		additional,
		// Must be the resource name (e.g. resourcemanager.datumapis.com/Project)
		attribute.String("storage.resources.datumapis.go/resource_type", storage.ResourceType(t.zero)),
		// Must be the fully qualified name of the resource in protobuf
		attribute.String("storage.resources.datumapis.go/protobuf_resource_type", string(t.zero.ProtoReflect().Descriptor().FullName())),
	)
}

func (t *tracer[R]) GetResource(ctx context.Context, req *storage.GetResourceRequest) (R, error) {
	ctx, span := otel.Tracer("").Start(ctx, "datum.storage.GetResource", trace.WithAttributes(
		t.resourceAttributes(
			attribute.String("storage.resources.datumapis.go/resource_name", req.Name),
		)...,
	))
	defer span.End()

	resource, err := t.srv.GetResource(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return resource, err
}

func (t *tracer[R]) ListResources(ctx context.Context, req *storage.ListResourcesRequest) (*storage.ListResourcesResponse[R], error) {
	ctx, span := otel.Tracer("").Start(ctx, "datum.storage.ListResources", trace.WithAttributes(
		t.resourceAttributes(
			attribute.String("storage.resources.datumapis.go/resource_parent", req.Parent),
		)...,
	))
	defer span.End()

	resources, err := t.srv.ListResources(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return resources, err
}

func (t *tracer[R]) UpdateResource(ctx context.Context, req *storage.UpdateResourceRequest[R]) (R, error) {
	ctx, span := otel.Tracer("").Start(ctx, "datum.storage.UpdateResource", trace.WithAttributes(
		t.resourceAttributes(
			attribute.String("storage.resources.datumapis.go/resource_name", req.Name),
		)...,
	))
	defer span.End()

	resource, err := t.srv.UpdateResource(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return resource, err
}

func (t *tracer[R]) DeleteResource(ctx context.Context, req *storage.DeleteResourceRequest) (R, error) {
	ctx, span := otel.Tracer("").Start(ctx, "datum.storage.DeleteResource", trace.WithAttributes(
		t.resourceAttributes(
			attribute.String("storage.resources.datumapis.go/resource_name", req.Name),
		)...,
	))
	defer span.End()

	resource, err := t.srv.DeleteResource(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return resource, err
}

func (t *tracer[R]) UndeleteResource(ctx context.Context, req *storage.UndeleteResourceRequest) (R, error) {
	ctx, span := otel.Tracer("").Start(ctx, "datum.storage.UndeleteResource")
	defer span.End()

	resource, err := t.srv.UndeleteResource(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return resource, err
}

func (t *tracer[R]) PurgeResource(ctx context.Context, req *storage.PurgeResourceRequest) (R, error) {
	ctx, span := otel.Tracer("").Start(ctx, "datum.storage.PurgeResource")
	defer span.End()

	resource, err := t.srv.PurgeResource(ctx, req)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
	}
	return resource, err
}
