package validation

import (
	"context"
	"fmt"
	"regexp"
	"slices"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"go.datum.net/iam/internal/storage"
	"go.datum.net/iam/internal/validation/field"
)

// PermissionValidator will validate the provided permission and return any
// validation errors that are encountered. The provided field path should be the
// path to the permission in the request being validated.
type PermissionValidator func(fieldPath *field.Path, permission string) field.ErrorList

var permissionMatcher *regexp.Regexp

func init() {
	permissionMatcher = regexp.MustCompile(`([a-zA-Z0-9\.\-]+)/([a-zA-Z\.\-]+)\.([a-zA-Z]+)`)
}

func NewPermissionValidator(services storage.ResourceGetter[*iampb.Service]) PermissionValidator {
	return func(fieldPath *field.Path, permission string) field.ErrorList {
		errs := field.ErrorList{}
		matches := permissionMatcher.FindStringSubmatch(permission)
		// There should only ever be a single match on the permission and we
		// should have 3 capture groups on the first match.
		if len(matches) != 4 {
			errs = append(errs, field.Invalid(fieldPath, permission, "permission must be in the format `{service_name}/{resource_type_plural}.{action}"))
			return errs
		}

		serviceName := matches[1]
		resourceType := matches[2]
		action := matches[3]

		service, err := services.GetResource(context.Background(), &storage.GetResourceRequest{
			Name: "services/" + serviceName,
		})
		if err != nil {
			errs = append(errs, field.InternalError(fieldPath, fmt.Errorf("internal error when validating permission")))
		}

		found := false
		for _, resource := range service.GetSpec().GetResources() {
			if resource.Plural == resourceType {
				if slices.Contains(resource.Permissions, action) {
					found = true
				}
			}
		}

		if !found {
			errs = append(errs, field.NotFound(fieldPath, permission))
		}

		return errs
	}
}
