package validation

import (
	"fmt"
	"strings"

	resourcemanagerpb "buf.build/gen/go/datum-cloud/iam/protocolbuffers/go/datum/resourcemanager/v1alpha"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Errors accumulates validation errors.
// Assuming this type (or similar) exists in your validation package,
// based on its usage in organization.go (errs.GRPCStatus().Err()).
// If it doesn't, this part would need to be adapted to your error handling.
type Errors []*errdetails.BadRequest_FieldViolation

// Add appends a new field violation.
func (e *Errors) Add(field, description string) {
	*e = append(*e, &errdetails.BadRequest_FieldViolation{
		Field:       field,
		Description: description,
	})
}

// GRPCStatus converts accumulated errors to a gRPC status.
func (e Errors) GRPCStatus() *status.Status {
	if len(e) == 0 {
		return status.New(codes.OK, "")
	}
	st := status.New(codes.InvalidArgument, "request validation failed")
	br := &errdetails.BadRequest{}
	br.FieldViolations = e
	st, _ = st.WithDetails(br)
	return st
}

// ValidateProject checks the validity of a Project resource.
func ValidateProject(project *resourcemanagerpb.Project) Errors {
	var errs Errors

	if strings.TrimSpace(project.GetDisplayName()) == "" {
		errs.Add("project.display_name", "display_name is required and cannot be empty")
	}

	parent := project.GetParent()
	if parent == "" {
		errs.Add("project.parent", "parent is required")
	} else {
		if !strings.HasPrefix(parent, "organizations/") && !strings.HasPrefix(parent, "folders/") {
			errs.Add("project.parent", "parent must be in the format 'organizations/{organization_id}' or 'folders/{folder_id}'")
		}
		// Further validation could check if the organization/folder ID part is valid.
	}

	name := project.GetName()
	if name != "" { // Name is usually set by the system, but if provided, validate its format.
		parts := strings.Split(name, "/")
		if len(parts) != 2 || parts[0] != "projects" || strings.TrimSpace(parts[1]) == "" {
			errs.Add("project.name", "name must be in the format 'projects/{project_id}' and project_id cannot be empty")
		}
	}

	// Example: Validate Project ID character set if needed (assuming project_id comes from name or a separate field)
	// projectID := project.GetProjectId() // Assuming ProjectId field exists if not derived from Name
	// if projectID != "" && !IsValidResourceID(projectID) {
	// 	errs.Add("project.project_id", "project_id contains invalid characters or format")
	// }

	// Add other project-specific validations as needed.
	// For example, labels, annotations, etc.

	return errs
}

// AssertProjectImmutableFieldsUnchanged checks if any immutable fields of a project
// have been changed during an update operation.
// This is a placeholder and needs to be adapted based on actual immutable fields for Project.
func AssertProjectImmutableFieldsUnchanged(updateMaskPaths []string, original, updated *resourcemanagerpb.Project) Errors {
	var errs Errors
	immutableFields := map[string]bool{
		"name":        true,
		"project_id":  true, // Assuming ProjectId is a field or derived from name
		"create_time": true,
		// "parent": true, // Parent is changed via MoveProject, not UpdateProject
	}

	for _, path := range updateMaskPaths {
		if immutableFields[path] {
			// This is a simplified check. A proper check would involve reflection or specific field comparisons.
			// For example, if original.GetName() != updated.GetName() for path "name".
			// The fmutils.Overwrite in the server handler applies changes first,
			// so 'updated' already has new values. We compare 'original' (before Overwrite)
			// with 'updated' (after Overwrite) for paths in the mask that are immutable.
			// This requires passing the state *before* fmutils.Overwrite for 'original'.
			// The current updaterFunc pattern in projects_service.go clones existingProject *before* Overwrite,
			// which can be used as 'original'. 'projectToUpdate' after Overwrite is 'updated'.

			// This placeholder needs actual comparison logic.
			// Example for 'name':
			// if path == "name" && original.GetName() != updated.GetName() {
			//    errs.Add(path, fmt.Sprintf("%s is immutable and cannot be changed", path))
			// }
			errs.Add(path, fmt.Sprintf("field '%s' is immutable and part of update_mask", path))
		}
	}
	return errs
}

// IsValidResourceID is a helper function to validate characters in a resource ID.
// This is an example, adapt as needed.
// func IsValidResourceID(id string) bool {
// 	if len(id) < 1 || len(id) > 63 {
// 		return false
// 	}
// 	match, _ := regexp.MatchString("^[a-z]([-a-z0-9]{0,61}[a-z0-9])?$", id)
// 	return match
// }
