package templating

import (
	"fmt"
	"net/url"

	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

func ValidateEmailVariables(email *notificationv1alpha1.Email, emailTemplate *notificationv1alpha1.EmailTemplate) field.ErrorList {
	errs := field.ErrorList{}

	if email == nil || emailTemplate == nil {
		return append(errs, field.InternalError(field.NewPath("spec"), fmt.Errorf("email or template is nil")))
	}

	ev := email.Spec.Variables
	etv := emailTemplate.Spec.Variables

	// Map template declared variables for quick lookup
	declared := make(map[string]notificationv1alpha1.TemplateVariable, len(etv))
	required := make(map[string]bool) // track whether required vars were supplied
	for _, v := range etv {
		declared[v.Name] = v
		if v.Required {
			required[v.Name] = false // false means the variable is not provided. We will check for this and update it to true if it is provided.
		}
	}

	// Keep track of duplicates in the Email variables list
	seen := make(map[string]struct{})

	variablesPath := field.NewPath("spec").Child("variables")

	for i, v := range ev {
		idxPath := variablesPath.Index(i)

		// Duplicate name detection
		if _, exists := seen[v.Name]; exists {
			errs = append(errs, field.Duplicate(idxPath.Child("name"), v.Name))
			continue
		}
		seen[v.Name] = struct{}{}

		tmplVar, ok := declared[v.Name]
		if !ok {
			// Variable not declared in the template
			errs = append(errs, field.NotSupported(idxPath.Child("name"), v.Name, []string{"declared variables"}))
			continue
		}

		// Mark required as provided
		if _, req := required[v.Name]; req {
			required[v.Name] = true
		}

		// Type-specific validation
		switch tmplVar.Type {
		case notificationv1alpha1.EmailTemplateVariableTypeURL:
			// Validate that value is a well-formed HTTPS URL
			if parsed, err := url.ParseRequestURI(v.Value); err != nil {
				errs = append(errs, field.Invalid(idxPath.Child("value"), v.Value, "must be a valid URL"))
			} else if parsed.Scheme != "https" {
				errs = append(errs, field.Invalid(idxPath.Child("value"), v.Value, "URL must use https scheme"))
			}
		case notificationv1alpha1.EmailTemplateVariableTypeString:
			// No extra validation for plain strings (non-empty already ensured by CRD validation)
		default:
			// Unknown type – treat as internal error to surface misconfiguration
			errs = append(errs, field.InternalError(idxPath.Child("type"), fmt.Errorf("unsupported template variable type %q", tmplVar.Type)))
		}
	}

	// Check for any required variables missing from the Email
	for name, provided := range required {
		if !provided {
			errs = append(errs, field.Required(variablesPath, fmt.Sprintf("required variable %q is missing", name)))
		}
	}

	return errs
}
