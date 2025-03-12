package schema

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"go.datum.net/iam/internal/storage"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type Registry struct {
	// The registry will resolve resources based on services that are stored with
	// the IAM service.
	Services storage.ResourceGetter[*iampb.Service]

	//
	cache map[string][]*resourceNameMatcher

	setup sync.Once
}

type ResourceReference struct {
	// The fully qualified type reference for the resource that's registered
	// (e.g. resourcemanager.datumapis.com/Project).
	Type string

	Name string

	// Provides the fully qualified name of the resource, including the service
	// name that provides the resource.
	//
	// e.g. resourcemanager.datumapis.com/projects/my-example-project
	SelfLink string
}

type resourceNameMatcher struct {
	regex *regexp.Regexp

	// The original resource name pattern that was compiled into a regex.
	pattern string

	// The fully qualified type reference for the resource that's registered
	// (e.g. resourcemanager.datumapis.com/Project).
	resourceType string
}

func newResourceNameMatcher(resourceType, pattern string) (*resourceNameMatcher, error) {
	// Replace {param} with named capture group (?P<param>[^/]+)
	regexPattern := "^" + regexp.MustCompile(`\{([^/]+)\}`).ReplaceAllString(pattern, `(?P<$1>[^/]+)`) + "$"
	regex, err := regexp.Compile(regexPattern)
	if err != nil {
		return nil, fmt.Errorf("failed to compile resource name pattern, must be in format `resources/{resource}`: %w", err)
	}

	return &resourceNameMatcher{
		regex:        regex,
		pattern:      pattern,
		resourceType: resourceType,
	}, nil
}

func (r *Registry) init() {
	r.cache = make(map[string][]*resourceNameMatcher)
}

func (r *Registry) ResolveResource(ctx context.Context, resourceURL string) (*ResourceReference, error) {
	ctx, span := otel.Tracer("").Start(ctx, "iam.schema.ResolveResource", trace.WithAttributes(
		attribute.String("iam.datumapis.com/resource_url", resourceURL),
	))
	defer span.End()

	r.setup.Do(r.init)
	// Check to see if the resource is referencing the root resource in the IAM
	// system. The Root resource acts as the parent of any resource that doesn't
	// have a configured parent in the hierarchy so we can attach permissions for
	// managing top level resources.
	if strings.HasPrefix(resourceURL, "iam.datumapis.com/root") {
		return &ResourceReference{
			Type:     "iam.datumapis.com/Root",
			Name:     strings.TrimPrefix(resourceURL, "iam.datumapis.com/"),
			SelfLink: resourceURL,
		}, nil
	}

	serviceName, resourceName, err := ParseResourceURL(resourceURL)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	if matchers, exists := r.cache[serviceName]; exists {
		for _, matcher := range matchers {
			if matcher.regex.MatchString(resourceName) {
				span.SetAttributes(attribute.Bool("schema.iam.cached", true))
				return &ResourceReference{
					Type:     matcher.resourceType,
					Name:     resourceName,
					SelfLink: resourceURL,
				}, nil
			}
		}
	}

	span.SetAttributes(attribute.Bool("schema.iam.cached", false))

	service, err := r.Services.GetResource(ctx, &storage.GetResourceRequest{
		Name: "services/" + serviceName,
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	// Load the matchers based on the service name
	matchers, err := buildMatchers(service)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	r.cache[serviceName] = matchers

	for _, matcher := range matchers {
		if matcher.regex.MatchString(resourceName) {
			return &ResourceReference{
				Type:     matcher.resourceType,
				Name:     resourceName,
				SelfLink: resourceURL,
			}, nil
		}
	}

	err = fmt.Errorf("could not find a matching resource name pattern for '%s' in service '%s'", resourceName, serviceName)
	span.SetStatus(codes.Error, err.Error())

	return nil, err
}

func buildMatchers(service *iampb.Service) ([]*resourceNameMatcher, error) {
	var matchers []*resourceNameMatcher

	for _, resource := range service.GetSpec().GetResources() {
		for _, resourcePattern := range resource.GetResourceNamePatterns() {
			patternMatcher, err := newResourceNameMatcher(resource.Type, resourcePattern)
			if err != nil {
				return nil, fmt.Errorf("failed to create resource name matcher: %w", err)
			}
			matchers = append(matchers, patternMatcher)
		}
	}
	return matchers, nil
}

func ParseResourceURL(resourceURL string) (serviceName, resourceName string, err error) {
	// The fully qualified resource name is expected to be in the format:
	// `{service_name}/{resource_name}`.
	nameParts := strings.SplitN(resourceURL, "/", 2)
	if len(nameParts) != 2 {
		return "", "", fmt.Errorf("resource name must be in the format '{service_name}/{resource_name}'")
	}
	return nameParts[0], nameParts[1], nil
}
