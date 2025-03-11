package validation

import (
	"strings"

	iampb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/iam/v1alpha"
	"go.datum.net/iam/internal/validation/field"
)

func ValidateService(service *iampb.Service) field.ErrorList {
	errs := field.ErrorList{}
	errs = append(errs, validateServiceSpec(service.ServiceId, service.Spec)...)

	return errs
}

func validateServiceSpec(serviceID string, spec *iampb.ServiceSpec) field.ErrorList {
	fieldPath := field.NewPath("spec")
	if spec == nil {
		return field.ErrorList{field.Required(fieldPath, "")}
	}
	errs := field.ErrorList{}
	if len(spec.Resources) == 0 {
		errs = append(errs, field.Required(fieldPath.Child("resources"), ""))
	} else {
		for index, resource := range spec.Resources {
			errs = append(errs, validateServiceResource(fieldPath.Child("resources").Index(index), serviceID, resource)...)
		}
	}

	return errs
}

func validateServiceResource(fieldPath *field.Path, serviceID string, resource *iampb.Resource) field.ErrorList {
	errs := field.ErrorList{}
	if resource.Type == "" {
		errs = append(errs, field.Required(fieldPath.Child("type"), ""))
	} else {
		// Resource types must be in the format
		// "{service_name}/{resource_type}".
		parts := strings.Split(resource.Type, "/")
		if len(parts) != 2 {
			errs = append(errs, field.Invalid(fieldPath.Child("type"), resource.Type, "expected resource type to be in the format '{service_id}/{resource_type}'"))
		} else {
			if parts[0] != serviceID {
				errs = append(errs, field.Invalid(fieldPath.Child("type"), resource.Type, "first portion of the resource type must be the service ID"))
			}
		}
	}

	if len(resource.Permissions) == 0 {
		errs = append(errs, field.Required(fieldPath.Child("permissions"), ""))
	} else {
		// TODO: add validation
	}

	if resource.Plural == "" {
		errs = append(errs, field.Required(fieldPath.Child("plural"), ""))
	} else {
		// TODO: add validation
	}

	if resource.Singular == "" {
		errs = append(errs, field.Required(fieldPath.Child("singular"), ""))
	} else {
		// TODO: add validation
	}

	if len(resource.ResourceNamePatterns) == 0 {
		errs = append(errs, field.Required(fieldPath.Child("resource_name_patterns"), ""))
	} else {
		// TODO: add validation
	}

	return errs
}
