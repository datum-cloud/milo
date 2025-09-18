package engine

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"go.miloapis.com/milo/internal/quota/validation"
	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

// CELEngine provides CEL expression evaluation capabilities for quota operations.
type CELEngine interface {
	// ValidateConditions validates CEL expressions in trigger conditions.
	ValidateConditions(conditions []quotav1alpha1.ConditionExpression) error

	// ValidateNameExpression validates a CEL expression that should return a string.
	ValidateNameExpression(expression string) error

	// EvaluateConditions evaluates all trigger conditions against a resource object.
	EvaluateConditions(conditions []quotav1alpha1.ConditionExpression, obj *unstructured.Unstructured) (bool, error)

	// EvaluateNameExpression evaluates a name expression against a resource object.
	EvaluateNameExpression(expression string, obj *unstructured.Unstructured) (string, error)
}

// celEngine implements CELEngine by delegating to the validation package.
type celEngine struct {
	validator *validation.CELValidator
}

// NewCELEngine creates a new CEL engine using the validation package.
func NewCELEngine() (CELEngine, error) {
	validator, err := validation.NewCELValidator()
	if err != nil {
		return nil, err
	}

	return &celEngine{validator: validator}, nil
}

// ValidateConditions validates CEL expressions in trigger conditions.
func (e *celEngine) ValidateConditions(conditions []quotav1alpha1.ConditionExpression) error {
	return e.validator.ValidateConditions(conditions)
}

// ValidateNameExpression validates a CEL expression that should return a string.
func (e *celEngine) ValidateNameExpression(expression string) error {
	return e.validator.ValidateNameExpression(expression)
}

// EvaluateConditions evaluates all trigger conditions against a resource object.
func (e *celEngine) EvaluateConditions(conditions []quotav1alpha1.ConditionExpression, obj *unstructured.Unstructured) (bool, error) {
	return e.validator.EvaluateConditions(conditions, obj)
}

// EvaluateNameExpression evaluates a name expression against a resource object.
func (e *celEngine) EvaluateNameExpression(expression string, obj *unstructured.Unstructured) (string, error) {
	return e.validator.EvaluateNameExpression(expression, obj)
}
