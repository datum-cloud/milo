package validation

import (
	"fmt"
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// extractCELExpressions extracts CEL expressions from {{ }} delimiters in a template string.
// Uses a simplified parser that handles string literals but not nested braces for better performance.
func extractCELExpressions(templateStr string) []string {
	expressions := make([]string, 0)
	runes := []rune(templateStr)

	for i := 0; i < len(runes)-1; {
		if runes[i] == '{' && runes[i+1] == '{' {
			// Skip triple braces - not valid CEL syntax
			if i < len(runes)-2 && runes[i+2] == '{' {
				// Skip past all consecutive braces to avoid processing overlapping {{
				for i < len(runes) && runes[i] == '{' {
					i++
				}
				continue
			}

			// Find closing delimiter
			if end := findClosingDelimiter(runes, i+2); end != -1 {
				expr := strings.TrimSpace(string(runes[i+2 : end]))
				if expr != "" {
					expressions = append(expressions, expr)
				}
				i = end + 2
			} else {
				i++
			}
		} else {
			i++
		}
	}

	return expressions
}

// findClosingDelimiter finds the closing }} delimiter, handling string literals.
func findClosingDelimiter(runes []rune, start int) int {
	for i := start; i < len(runes)-1; i++ {
		// Handle string literals to avoid }} inside strings
		if runes[i] == '"' || runes[i] == '\'' {
			i = skipString(runes, i)
			if i == -1 {
				return -1 // Malformed string
			}
			continue
		}
		if runes[i] == '}' && runes[i+1] == '}' {
			return i
		}
	}
	return -1
}

// skipString skips over a string literal, handling escape sequences.
func skipString(runes []rune, start int) int {
	if start >= len(runes) {
		return -1
	}

	delimiter := runes[start]
	for i := start + 1; i < len(runes); i++ {
		if runes[i] == delimiter {
			// Check if it's escaped
			backslashCount := 0
			for j := i - 1; j >= start && runes[j] == '\\'; j-- {
				backslashCount++
			}
			// If even number of backslashes, the quote is not escaped
			if backslashCount%2 == 0 {
				return i
			}
		}
	}
	return -1 // Unterminated string
}

// extractVariablesFromCEL extracts variable references from a CEL expression.
// e.g., trigger.metadata.name -> "trigger"
func extractVariablesFromCEL(expression string) []string {
	re := regexp.MustCompile(`\b(trigger|user|requestInfo)\b`)
	matches := re.FindAllString(expression, -1)

	var variables []string
	seen := make(map[string]bool)
	for _, match := range matches {
		if !seen[match] {
			variables = append(variables, match)
			seen[match] = true
		}
	}

	return variables
}

// containsCELExpressions checks if a string contains CEL template expressions.
func containsCELExpressions(str string) bool {
	return strings.Contains(str, "{{") && strings.Contains(str, "}}")
}

// validateCELTemplate validates a string that contains CEL expressions with the given allowed variables.
func validateCELTemplate(templateStr string, allowedVariables []string, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if templateStr == "" {
		return allErrs
	}

	celValidator, err := NewCELValidator()
	if err != nil {
		allErrs = append(allErrs, field.InternalError(fldPath, fmt.Errorf("failed to create CEL validator: %w", err)))
		return allErrs
	}

	expressions := extractCELExpressions(templateStr)
	for i, expr := range expressions {
		if errs := validateCELExpression(expr, allowedVariables, celValidator, fldPath.Index(i)); len(errs) > 0 {
			allErrs = append(allErrs, errs...)
		}
	}

	return allErrs
}

// validateCELExpression validates a single CEL expression with the given allowed variables.
func validateCELExpression(expression string, allowedVariables []string, celValidator *CELValidator, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if err := celValidator.ValidateTemplateExpression(expression); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, expression, fmt.Sprintf("CEL syntax validation failed: %v", err)))
	}

	variables := extractVariablesFromCEL(expression)
	allowedSet := make(map[string]bool)
	for _, v := range allowedVariables {
		allowedSet[v] = true
	}

	for _, variable := range variables {
		if !allowedSet[variable] {
			allErrs = append(allErrs, field.Invalid(fldPath, variable, fmt.Sprintf("invalid template variable '%s', valid variables are: %v", variable, allowedVariables)))
		}
	}

	return allErrs
}

// validateTemplateOrKubernetesName validates a string that either contains CEL expressions OR is a literal Kubernetes name.
func validateTemplateOrKubernetesName(str string, allowedVariables []string, allowEmpty bool, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if strings.TrimSpace(str) == "" {
		if !allowEmpty {
			allErrs = append(allErrs, field.Required(fldPath, "value cannot be empty"))
		}
		return allErrs
	}

	// If it contains CEL expressions, validate as template
	if containsCELExpressions(str) {
		allErrs = append(allErrs, validateCELTemplate(str, allowedVariables, fldPath)...)
	} else {
		// Otherwise validate as Kubernetes name using official validation
		if errs := validation.IsDNS1123Subdomain(str); len(errs) > 0 {
			allErrs = append(allErrs, field.Invalid(fldPath, str, fmt.Sprintf("invalid Kubernetes name: %s", strings.Join(errs, "; "))))
		}
	}

	return allErrs
}

// validateTemplateOrGenerateName validates values intended for metadata.generateName.
// Literal generateName values must end with '-' and have a DNS-compliant prefix.
func validateTemplateOrGenerateName(str string, allowedVariables []string, allowEmpty bool, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if strings.TrimSpace(str) == "" {
		if !allowEmpty {
			allErrs = append(allErrs, field.Required(fldPath, "value cannot be empty"))
		}
		return allErrs
	}

	if containsCELExpressions(str) {
		allErrs = append(allErrs, validateCELTemplate(str, allowedVariables, fldPath)...)
		return allErrs
	}

	if !strings.HasSuffix(str, "-") {
		allErrs = append(allErrs, field.Invalid(fldPath, str, "generateName must end with '-'"))
		return allErrs
	}

	prefix := strings.TrimSpace(str[:len(str)-1])
	if prefix == "" {
		allErrs = append(allErrs, field.Invalid(fldPath, str, "generateName prefix cannot be empty"))
		return allErrs
	}

	if errs := validation.IsDNS1123Subdomain(prefix); len(errs) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, str, fmt.Sprintf("invalid Kubernetes generateName prefix: %s", strings.Join(errs, "; "))))
	}

	return allErrs
}

// validateTemplateOrLiteral validates strings that allow arbitrary literal values when not templated.
func validateTemplateOrLiteral(str string, allowedVariables []string, allowEmpty bool, fldPath *field.Path) field.ErrorList {
	var allErrs field.ErrorList

	if strings.TrimSpace(str) == "" {
		if !allowEmpty {
			allErrs = append(allErrs, field.Required(fldPath, "value cannot be empty"))
		}
		return allErrs
	}

	if containsCELExpressions(str) {
		allErrs = append(allErrs, validateCELTemplate(str, allowedVariables, fldPath)...)
	}

	return allErrs
}
