package templating

import (
	"fmt"
	"html/template"

	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateHTMLTemplate validates the provided htmlBody string ensuring:
// 1. It compiles as a Go html/template.
// 2. Every referenced variable is declared in spec.variables.
// 3. All declared variables marked Required=true are referenced in the template.
//
// It returns a field.ErrorList suitable for propagating through Kubernetes admission errors.
func ValidateHTMLTemplate(htmlBody string, declaredVars []notificationv1alpha1.TemplateVariable) field.ErrorList {
	errs := field.ErrorList{}
	htmlBodyPath := field.NewPath("spec").Child("htmlBody")

	// Compile template first.
	if _, err := template.New("email").Parse(htmlBody); err != nil {
		errs = append(errs, field.Invalid(htmlBodyPath, htmlBody, fmt.Sprintf("htmlBody is not a valid Go template: %v", err)))
		// We still continue variable checks to accumulate more errors, as parse error might
		// not prevent regexp extraction of simple variable names.
	}

	// Map declared variables for quick lookup and track required ones
	declared := make(map[string]notificationv1alpha1.TemplateVariable, len(declaredVars))
	requiredUsed := make(map[string]bool)
	for _, v := range declaredVars {
		declared[v.Name] = v
		if v.Required {
			requiredUsed[v.Name] = false
		}
	}

	// Find variable occurrences.
	matches := templateVarRegexp.FindAllStringSubmatch(htmlBody, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		name := m[1]
		if _, ok := declared[name]; !ok {
			errs = append(errs, field.NotSupported(htmlBodyPath, fmt.Sprintf("{{.%s}}", name), []string{"declared variables"}))
		} else {
			if _, req := requiredUsed[name]; req {
				requiredUsed[name] = true
			}
		}
	}

	// Check for required variables missing in htmlBody.
	for name, used := range requiredUsed {
		if !used {
			errs = append(errs, field.Required(htmlBodyPath, fmt.Sprintf("required variable %q is not referenced in htmlBody", name)))
		}
	}

	return errs
}
