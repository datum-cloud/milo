package templating

import (
	"fmt"
	"text/template"

	notificationv1alpha1 "go.miloapis.com/milo/pkg/apis/notification/v1alpha1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateTextTemplate validates the provided textBody string ensuring:
// 1. It compiles as a Go text/template.
// 2. Every referenced variable is declared in spec.variables.
// 3. All declared variables marked Required=true are referenced in the template.
//
// It returns a field.ErrorList suitable for propagating through Kubernetes admission errors.
func ValidateTextTemplate(textBody string, declaredVars []notificationv1alpha1.TemplateVariable) field.ErrorList {
	errs := field.ErrorList{}
	textBodyPath := field.NewPath("spec").Child("textBody")

	// Compile the template first to catch syntax errors early.
	if _, err := template.New("email").Parse(textBody); err != nil {
		errs = append(errs, field.Invalid(textBodyPath, textBody, fmt.Sprintf("textBody is not a valid Go template: %v", err)))
		// Continue with variable checks to surface any additional issues even if the template fails to parse.
	}

	// Map declared variables for quick lookup and track required ones.
	declared := make(map[string]notificationv1alpha1.TemplateVariable, len(declaredVars))
	requiredUsed := make(map[string]bool)
	for _, v := range declaredVars {
		declared[v.Name] = v
		if v.Required {
			requiredUsed[v.Name] = false
		}
	}

	// Find variable occurrences inside the template body.
	matches := templateVarRegexp.FindAllStringSubmatch(textBody, -1)
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		name := m[1]
		if _, ok := declared[name]; !ok {
			// Variable referenced but not declared.
			errs = append(errs, field.NotSupported(textBodyPath, fmt.Sprintf("{{.%s}}", name), []string{"declared variables"}))
		} else {
			if _, isRequired := requiredUsed[name]; isRequired {
				requiredUsed[name] = true
			}
		}
	}

	// Ensure all required variables were actually used in the template body.
	for name, used := range requiredUsed {
		if !used {
			errs = append(errs, field.Required(textBodyPath, fmt.Sprintf("required variable %q is not referenced in textBody", name)))
		}
	}

	return errs
}
