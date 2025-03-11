package storage

import (
	"context"
	"sync"

	"github.com/google/uuid"
	"go.datum.net/iam/internal/grpc/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Provides an in-memory implementation of the storage interface for working
// with resources that don't need to be persisted beyond the application's
// runtime.
//
// Do NOT use in a production setting.
type InMemory[R Resource] struct {
	resources map[string]R
	setup     sync.Once

	// Represents a zero value of the underlying type being stored. This is here
	// for convinence when returning the zero value from a function when an error
	// is encountered. The zero value should never be used in any non-error
	// scenario.
	zero R
}

// GetResource will retrieve the resource from the underlying postgres database
// table. An gRPC NotFound error will be returned if the requested resource does
// not exist.
func (r *InMemory[R]) GetResource(ctx context.Context, req *GetResourceRequest) (R, error) {
	resource, exists := r.resources[req.Name]
	if !exists {
		return r.zero, errors.New(codes.NotFound, "resource not found").Err()
	}
	return resource, nil
}

// ListResources will list a page of resources from the underlying postgres
// database.
//
// TODO: Support pagination.
func (r *InMemory[R]) ListResources(ctx context.Context, req *ListResourcesRequest) (*ListResourcesResponse[R], error) {
	resources := make([]R, 0, len(r.resources))
	deleteField := r.zero.ProtoReflect().Descriptor().Fields().ByName("delete_time")
	parentField := r.zero.ProtoReflect().Descriptor().Fields().ByName("parent")

	for _, resource := range r.resources {
		deleteTime := resource.ProtoReflect().Get(deleteField).Message().Interface().(*timestamppb.Timestamp)
		// Only include non-deleted resources.
		if deleteField != nil && deleteTime.IsValid() {
			continue
		}

		// Only include resources with the same parent.
		if parentField != nil && resource.ProtoReflect().Get(parentField).String() != req.Parent {
			continue
		}

		resources = append(resources, resource)
	}
	return &ListResourcesResponse[R]{
		Resources: resources,
	}, nil
}

// CreateResource will create a new resource in the underlying storage
// implementation.
func (r *InMemory[R]) CreateResource(ctx context.Context, req *CreateResourceRequest[R]) (R, error) {
	r.init()

	resource := proto.Clone(req.Resource).(R)
	resourceReflector := resource.ProtoReflect()
	resourceFields := resourceReflector.Descriptor().Fields()

	// Set the UID of the resource when the user hasn't provided one. This is
	// there for convenience so callers don't always have to create a new UUID.
	uidField := resourceFields.ByName("uid")
	resource.ProtoReflect().Set(uidField, protoreflect.ValueOfString(uuid.NewString()))

	// Add the resource to storage.
	r.resources[req.Name] = resource

	return req.Resource, nil
}

// UpdateResource will update an existing resource in the underlying // database.
func (r *InMemory[R]) UpdateResource(ctx context.Context, req *UpdateResourceRequest[R]) (R, error) {
	r.init()

	resource, exists := r.resources[req.Name]
	if !exists {
		return r.zero, errors.New(codes.NotFound, "resource not found").Err()
	}

	updatedResource, err := req.Updater(resource)
	if err != nil {
		return r.zero, err
	}

	r.resources[req.Name] = updatedResource

	return updatedResource, nil
}

// DeleteResource will soft-delete a resource from the underlying database. The
// etag of the resource can be provided to ensure that the latest version of the
// resource was retrieved before deletion.
//
// TODO: Enforce etag validation.
func (r *InMemory[R]) DeleteResource(ctx context.Context, req *DeleteResourceRequest) (R, error) {
	r.init()

	resource, exists := r.resources[req.Name]
	if !exists {
		return r.zero, errors.New(codes.NotFound, "resource not found").Err()
	}

	resourceReflector := resource.ProtoReflect()
	resourceFields := resourceReflector.Descriptor().Fields()

	deleteTimeField := resourceFields.ByName("delete_time")
	if deleteTimeField != nil {
		resource.ProtoReflect().Set(deleteTimeField, protoreflect.ValueOf(timestamppb.Now()))
	} else {
		delete(r.resources, req.Name)
	}

	return resource, nil
}

func (r *InMemory[R]) UndeleteResource(ctx context.Context, req *UndeleteResourceRequest) (R, error) {
	r.init()

	resource, exists := r.resources[req.Name]
	if !exists {
		return r.zero, errors.New(codes.NotFound, "resource not found").Err()
	}

	resourceReflector := resource.ProtoReflect()
	resourceFields := resourceReflector.Descriptor().Fields()

	deleteTimeField := resourceFields.ByName("delete_time")
	resource.ProtoReflect().Clear(deleteTimeField)

	return resource, nil
}

func (r *InMemory[R]) PurgeResource(ctx context.Context, req *PurgeResourceRequest) (R, error) {
	return r.DeleteResource(ctx, &DeleteResourceRequest{
		Name: req.Name,
	})
}

func (r *InMemory[R]) init() {
	r.setup.Do(func() {
		r.resources = make(map[string]R)
	})
}
