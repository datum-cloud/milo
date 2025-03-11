package storage

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Resource interface {
	// The resource name provides the immutable and unique human-readable name of
	// the resource in the system. Resource names may be reused after a resource
	// has been purged from storage.
	GetName() string

	// Provides the system-generated unique ID of the resource.
	GetUid() string

	// Get the time the resource was created in the system.
	GetCreateTime() *timestamppb.Timestamp

	// The etag is computed by the service bassed on the contents of the resource.
	GetEtag() string

	// All resources are expected to be defined as protobuf messages.
	proto.Message
}

type ResourceGetter[T Resource] interface {
	GetResource(context.Context, *GetResourceRequest) (T, error)
}

type ResourceLister[T Resource] interface {
	ListResources(context.Context, *ListResourcesRequest) (*ListResourcesResponse[T], error)
}

type ResourceCreator[T Resource] interface {
	CreateResource(context.Context, *CreateResourceRequest[T]) (T, error)
}

type ResourceUpdater[T Resource] interface {
	UpdateResource(context.Context, *UpdateResourceRequest[T]) (T, error)
}

type ResourceDeleter[T Resource] interface {
	DeleteResource(context.Context, *DeleteResourceRequest) (T, error)
}

type ResourceUndeleter[T Resource] interface {
	UndeleteResource(context.Context, *UndeleteResourceRequest) (T, error)
}

type ResourcePurger[T Resource] interface {
	// PurgeResource will hard-delete a resource from the storage layer. A gRPC
	// NotFound error will be returned if the resource that's referenced does not
	// exist. The last state of the resource before it was purged will be
	// returned.
	PurgeResource(context.Context, *PurgeResourceRequest) (T, error)
}

// Resource defines a generic storage CRUD interface for working with protobuf
// resources.
type ResourceServer[T Resource] interface {
	// ResourceGetter supports retrieving a resource. See the ResourceGetter
	// interface for more detail.
	ResourceGetter[T]
	ResourceLister[T]
	ResourceCreator[T]
	ResourceUpdater[T]
	ResourceDeleter[T]
	ResourceUndeleter[T]
	ResourcePurger[T]
}

type CreateResourceRequest[T Resource] struct {
	Name     string
	Parent   string
	Resource T
}

type GetResourceRequest struct {
	Name string
}

type ListResourcesRequest struct {
	// The parent that should be searched by.
	Parent string

	// The max number of results per page that should be returned. If the number
	// of available results is larger than `page_size`, a `next_page_token` is
	// returned which can be used to get the next page of results in subsequent
	// requests. Acceptable values are 0 to 500, inclusive. (Default: 10)
	// The default value is used when a page_size of 0 is provided.
	PageSize int32

	// Specifies a page token to use. Set this to the nextPageToken returned by
	// previous list requests to get the next page of results.
	PageToken string

	// A filter that should be used to retrieve a subset of the resources.
	Filter string

	// Indicates whether soft-deleted resources should be returned when listing
	// resources.
	IncludeDeleted bool
}

type ListResourcesResponse[T Resource] struct {
	// A list of resources that were retrieved from storage for a given request.
	Resources []T

	// The cursor token that can be used to retrieve the next page of resources.
	// Must be used with the same page size and filter values.
	NextPageToken string
}

type UpdateResourceRequest[T Resource] struct {
	// The fully qualified name of the resource that should be retrieved from
	// storage.
	//
	// e.g. `projects/{project}`
	Name string
	// Updater will get executed during the update of the resource so the caller
	// can modify the existing resource before it's stored in the database. This
	// function should be used to apply any changes the caller wants to make to
	// the resource. Changes can be applied directly to the provided resource.
	//
	// The etag of the resource that's returned from this function will be
	// compared to the etag of the existing resource. If the etags are not equal
	// the request will be denied with a Conflict gRPC error. This behvaior can be
	// disabled by setting the etag of the resource to an empty string in the
	// updater function.
	//
	// The Updater function may return an error directly to prevent updates to the
	// resource.
	Updater func(existing T) (new T, err error)
}

type DeleteResourceRequest struct {
	Name string
	Etag string
}

type UndeleteResourceRequest struct {
	Name string
}

type PurgeResourceRequest struct {
	Name string
}

func getResourceDescriptor(resource Resource) (*annotations.ResourceDescriptor, error) {
	resourceDescriptor := proto.GetExtension(resource.ProtoReflect().Descriptor().Options(), annotations.E_Resource)
	if resourceDescriptor == nil {
		return nil, fmt.Errorf("resource '%s' does not have the required 'google.api.resource' protobuf annotation", resource.ProtoReflect().Descriptor().FullName())
	}

	return resourceDescriptor.(*annotations.ResourceDescriptor), nil
}

func ResourceType(resource Resource) string {
	descriptor, err := getResourceDescriptor(resource)
	if err != nil {
		return ""
	}

	return descriptor.Type
}

func ResourceNameMatches(resource Resource, name string) bool {
	resourceDescriptor := proto.GetExtension(resource.ProtoReflect().Descriptor().Options(), annotations.E_Resource).(*annotations.ResourceDescriptor)

	if IsResourceURL(name) {
		name = strings.SplitN(name, "/", 2)[1]
	}

	for _, pattern := range resourceDescriptor.Pattern {
		// Convert pattern to regex by replacing `{placeholder}` with `[^/]+`
		re := regexp.MustCompile(`\{[^/]+\}`)
		regexPattern := "^" + re.ReplaceAllString(pattern, `[^/]+`) + "$"

		// Compile the final regex pattern
		matcher := regexp.MustCompile(regexPattern)

		// Check if the path matches the pattern
		if matcher.MatchString(name) {
			return true
		}
	}

	return false
}

func IsResourceURL(name string) bool {
	parts := strings.SplitN(name, "/", -1)
	if len(parts) <= 2 {
		return false
	}

	return strings.Contains(parts[0], ".")
}

// Converts a resource URL to a resource name, removing the service prefix. If
// the resource name is not a resource URL, the original resource name will be
// returned. This does not validate that the resource name is correct or
// actually exists in the system.
func ResourceName(resourceURL string) string {
	if !IsResourceURL(resourceURL) {
		return resourceURL
	}

	return strings.SplitN(resourceURL, "/", 2)[1]
}

// Converts a resource type reference into the service name of the resource
// type.
func ServiceName(resourceType string) string {
	return strings.SplitN(resourceType, "/", 2)[0]
}
