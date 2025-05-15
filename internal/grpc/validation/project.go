package validation

import (
	"strings"

	resourcemanagerpb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/resourcemanager/v1alpha"
	"go.datum.net/iam/internal/validation/field"
	"go.datum.net/iam/internal/validation/meta"
)

// ValidateProject checks the validity of a Project resource.
func ValidateProject(project *resourcemanagerpb.Project) field.ErrorList {
	var errs field.ErrorList

	if strings.TrimSpace(project.GetDisplayName()) == "" {
		errs = append(errs, field.Required(field.NewPath("display_name"), "display_name is required and cannot be empty"))
	}

	parent := project.GetParent()
	if parent == "" {
		errs = append(errs, field.Required(field.NewPath("parent"), "parent is required"))
	} else if !strings.HasPrefix(parent, "organizations/") {
		errs = append(errs, field.Invalid(field.NewPath("parent"), parent, "parent must be in the format 'organizations/{organization_id}'"))
	}

	if project.ProjectId != "" {
		if errs := meta.ValidateResourceID(field.NewPath("project_id"), project.ProjectId); len(errs) > 0 {
			errs = append(errs, errs...)
		}
	}

	errs = append(errs, meta.ValidateAnnotations(field.NewPath("annotations"), project.Annotations)...)
	errs = append(errs, meta.ValidateLabels(field.NewPath("labels"), project.Labels)...)
	errs = append(errs, meta.ValidateResourceDisplayName(field.NewPath("display_name"), project.DisplayName)...)
	errs = append(errs, meta.ValidateResourceDescription(field.NewPath("description"), project.Description)...)

	return errs
}
