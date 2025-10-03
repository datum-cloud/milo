package validation

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"

	quotacel "go.miloapis.com/milo/internal/quota/cel"
	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// CELValidator provides compile-time validation for CEL expressions.
// It validates syntax, type safety, and security constraints but does not execute expressions.
type CELValidator struct {
	env *cel.Env
}

// NewCELValidator creates a new CEL validator with the shared quota CEL environment.
func NewCELValidator() (*CELValidator, error) {
	env, err := quotacel.NewQuotaEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return &CELValidator{env: env}, nil
}

// ValidateConditions validates CEL expressions in trigger conditions.
func (v *CELValidator) ValidateConditions(conditions []quotav1alpha1.ConditionExpression) error {
	for i, condition := range conditions {
		if err := v.validateExpression(condition.Expression, cel.BoolType); err != nil {
			return fmt.Errorf("condition %d: %w", i, err)
		}
	}
	return nil
}

// ValidateNameExpression validates a CEL expression that should return a string.
func (v *CELValidator) ValidateNameExpression(expression string) error {
	return v.validateExpression(expression, cel.StringType)
}

// validateExpression validates that a CEL expression is syntactically correct and returns the expected type.
func (v *CELValidator) validateExpression(expression string, expectedType *cel.Type) error {
	if strings.TrimSpace(expression) == "" {
		return fmt.Errorf("expression cannot be empty")
	}

	// Parse the expression
	ast, issues := v.env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("parse error: %w", issues.Err())
	}

	// Type-check the expression
	checked, issues := v.env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("type check error: %w", issues.Err())
	}

	// Verify the return type matches expectations
	if !checked.OutputType().IsEquivalentType(expectedType) {
		return fmt.Errorf("expression must return %s, got %s", expectedType, checked.OutputType())
	}

	// Additional security checks
	if err := v.validateSecurity(expression); err != nil {
		return fmt.Errorf("security validation failed: %w", err)
	}

	return nil
}

// validateSecurity performs basic security validation on CEL expressions.
func (v *CELValidator) validateSecurity(expression string) error {
	// Prevent potentially dangerous operations
	forbidden := []string{
		"system",
		"exec",
		"eval",
		"import",
		"file",
		"network",
		"subprocess",
	}

	lowerExpr := strings.ToLower(expression)
	for _, term := range forbidden {
		if strings.Contains(lowerExpr, term) {
			return fmt.Errorf("expression contains forbidden term: %s", term)
		}
	}

	// Limit expression length to prevent DoS
	if len(expression) > 1024 {
		return fmt.Errorf("expression exceeds maximum length of 1024 characters")
	}

	return nil
}

