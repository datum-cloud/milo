package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/genproto/googleapis/api/annotations"
	grpcCodes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

// This error will be returned from the parent resolver registry when a request
// is received to resolve the parent of a resource that has not been registered.
var ErrTypeNotRegistered = errors.New("parent resolver not registered for type")

// ParentResolver supports resolving the parent information of resource defined
// by services.
type ParentResolver interface {
	// ResolveParent returns a reference to the parent of the resource that was
	// requested. An error will be returned if the resource does not exist. A
	// resource reference will only be returned if a parent exists for the
	// resource.
	ResolveParent(ctx context.Context, resource *ResourceReference) (*ResourceReference, error)
}

type ParentResolverRegistry struct {
	resolvers map[string]ParentResolver

	once sync.Once
}

// ResourceReference provides all of the information necessary to reference a
// resource defined by a service.
type ResourceReference struct {
	// The fully qualified type reference for the resource that's registered
	// (e.g. resourcemanager.datumapis.com/Project).
	Type string

	// The resource name of the resource.
	//
	// e.g. projects/my-example-project
	Name string

	// The system-generated unique ID generated for a resource.
	UID string

	// Provides the fully qualified name of the resource, including the service
	// name that provides the resource.
	//
	// e.g. resourcemanager.datumapis.com/projects/my-example-project
	SelfLink string
}

// Register a new parent resolver for a resource. An error will be returned if
// a parent resolver is already registered for the resource.
func (p *ParentResolverRegistry) RegisterResolver(resource Resource, resolver ParentResolver) error {
	resourceDescriptor := proto.GetExtension(resource.ProtoReflect().Descriptor().Options(), annotations.E_Resource).(*annotations.ResourceDescriptor)

	if _, existing := p.resolvers[resourceDescriptor.Type]; existing {
		return fmt.Errorf("resolver already registered for type '%s'", resourceDescriptor.Type)
	}

	p.once.Do(func() {
		p.resolvers = make(map[string]ParentResolver)
	})

	p.resolvers[resourceDescriptor.Type] = TraceParentResolver(resolver)
	return nil
}

func (p *ParentResolverRegistry) ResolveParent(ctx context.Context, resource *ResourceReference) (*ResourceReference, error) {
	resolver, exists := p.resolvers[resource.Type]
	if !exists {
		return nil, nil
	}

	return resolver.ResolveParent(ctx, resource)
}

func ResourceParentResolver[R Resource](getter ResourceGetter[R]) ParentResolver {
	return ParentResolverFunc(func(ctx context.Context, resourceRef *ResourceReference) (*ResourceReference, error) {
		resource, err := getter.GetResource(ctx, &GetResourceRequest{
			Name: resourceRef.Name,
		})
		if err != nil {
			return nil, err
		}

		parentField := resource.ProtoReflect().Descriptor().Fields().ByName("parent")
		if parentField == nil {
			return nil, nil
		}

		if !proto.HasExtension(parentField.Options(), annotations.E_ResourceReference) {
			return nil, fmt.Errorf("resource '%s' does not have the required `google.api.resource_reference` annotation on the `parent` field", resource.ProtoReflect().Descriptor().FullName())
		}

		parentResourceRef := proto.GetExtension(parentField.Options(), annotations.E_ResourceReference).(*annotations.ResourceReference)

		slog.InfoContext(
			ctx,
			"Resolved parent type",
			slog.String("parent_type", string(resource.ProtoReflect().Descriptor().FullName())),
			slog.String("parent_type_ref", parentResourceRef.Type),
		)

		if parentResourceRef.Type == "*" {
			// TODO: figure out how to support resolution of multi-parent resources
			return nil, errors.New("parent resolver cannot resolve parent when type is set to '*'")
		}

		parentResourceName := resource.ProtoReflect().Get(parentField).String()

		return &ResourceReference{
			Type:     parentResourceRef.Type,
			Name:     parentResourceName,
			SelfLink: ServiceName(parentResourceRef.Type) + "/" + parentResourceName,
		}, nil
	})
}

type ParentResolverFunc func(context.Context, *ResourceReference) (*ResourceReference, error)

func (f ParentResolverFunc) ResolveParent(ctx context.Context, resource *ResourceReference) (*ResourceReference, error) {
	return f(ctx, resource)
}

func TraceParentResolver(resolver ParentResolver) ParentResolverFunc {
	tracer := otel.Tracer("")

	return func(ctx context.Context, rr *ResourceReference) (*ResourceReference, error) {
		ctx, span := tracer.Start(ctx, "datum.resources.ResolveParent", trace.WithAttributes(
			attribute.String("resources.datumapis.com/resource_type", rr.Type),
			attribute.String("resources.datumapis.com/resource_name", rr.Name),
		))

		parent, err := resolver.ResolveParent(ctx, rr)
		if err != nil && status.Code(err) != grpcCodes.NotFound {
			span.SetStatus(codes.Error, err.Error())
		}

		hasParent := parent != nil

		span.SetAttributes(attribute.Bool("resources.datumapis.com/has_parent", hasParent))

		if hasParent {
			span.SetAttributes(
				attribute.String("resources.datumapis.com/parent_resource_name", parent.Name),
				attribute.String("resources.datumapis.com/parent_resource_type", parent.Type),
			)
		}

		if status.Code(err) == grpcCodes.NotFound {
			return nil, nil
		}

		span.End()
		return parent, err
	}
}
