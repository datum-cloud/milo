package validation

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// CELValidator provides validation for CEL expressions used in GrantCreationPolicy.
type CELValidator struct {
	env *cel.Env
}

// NewCELValidator creates a new CEL validator with the appropriate environment.
func NewCELValidator() (*CELValidator, error) {
	env, err := cel.NewEnv(
		// Add the 'trigger' variable that contains the trigger resource
		cel.Variable("trigger", cel.DynType),

		// Add custom functions for Kubernetes operations
		cel.Function("has",
			cel.MemberOverload("has_field", []*cel.Type{cel.DynType, cel.StringType}, cel.BoolType,
				cel.BinaryBinding(func(obj, field ref.Val) ref.Val {
					if objMap, ok := obj.Value().(map[string]interface{}); ok {
						fieldStr := field.Value().(string)
						_, exists := getNestedField(objMap, fieldStr)
						return types.Bool(exists)
					}
					return types.False
				}),
			),
		),
	)
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

// EvaluateConditions evaluates all trigger conditions against a resource object.
func (v *CELValidator) EvaluateConditions(conditions []quotav1alpha1.ConditionExpression, obj *unstructured.Unstructured) (bool, error) {
	if len(conditions) == 0 {
		return true, nil // No conditions means always match
	}

	// Convert unstructured object to a map for CEL evaluation
	objData := obj.Object

	for i, condition := range conditions {
		result, err := v.evaluateCondition(condition.Expression, objData)
		if err != nil {
			return false, fmt.Errorf("condition %d evaluation failed: %w", i, err)
		}

		if !result {
			return false, nil // At least one condition failed
		}
	}

	return true, nil // All conditions passed
}

// EvaluateNameExpression evaluates a name expression against a resource object.
func (v *CELValidator) EvaluateNameExpression(expression string, obj *unstructured.Unstructured) (string, error) {
	objData := obj.Object

	// Parse and compile the expression
	ast, issues := v.env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return "", fmt.Errorf("parse error: %w", issues.Err())
	}

	checked, issues := v.env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return "", fmt.Errorf("type check error: %w", issues.Err())
	}

	// Create program with optimizations
	program, err := v.env.Program(checked,
		cel.EvalOptions(cel.OptOptimize),
	)
	if err != nil {
		return "", fmt.Errorf("program creation failed: %w", err)
	}

	// Evaluate with the resource object
	vars := map[string]interface{}{
		"trigger": objData,
	}

	result, _, err := program.Eval(vars)
	if err != nil {
		return "", fmt.Errorf("evaluation failed: %w", err)
	}

	// Convert result to string
	if str, ok := result.Value().(string); ok {
		return str, nil
	}

	return "", fmt.Errorf("expression did not return a string value")
}

// evaluateCondition evaluates a single condition expression.
func (v *CELValidator) evaluateCondition(expression string, objData map[string]interface{}) (bool, error) {
	// Parse and compile the expression
	ast, issues := v.env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("parse error: %w", issues.Err())
	}

	checked, issues := v.env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return false, fmt.Errorf("type check error: %w", issues.Err())
	}

	// Create program with optimizations
	program, err := v.env.Program(checked,
		cel.EvalOptions(cel.OptOptimize),
	)
	if err != nil {
		return false, fmt.Errorf("program creation failed: %w", err)
	}

	// Evaluate with the resource object
	vars := map[string]interface{}{
		"trigger": objData,
	}

	result, _, err := program.Eval(vars)
	if err != nil {
		return false, fmt.Errorf("evaluation failed: %w", err)
	}

	// Convert result to boolean
	if b, ok := result.Value().(bool); ok {
		return b, nil
	}

	return false, fmt.Errorf("expression did not return a boolean value")
}

// getNestedField retrieves a nested field from a map using dot notation.
func getNestedField(obj map[string]interface{}, fieldPath string) (interface{}, bool) {
	parts := strings.Split(fieldPath, ".")
	current := obj

	for i, part := range parts {
		if current == nil {
			return nil, false
		}

		if i == len(parts)-1 {
			// Last part - return the value
			value, exists := current[part]
			return value, exists
		}

		// Intermediate part - must be a map
		if next, ok := current[part].(map[string]interface{}); ok {
			current = next
		} else {
			return nil, false
		}
	}

	return current, true
}
