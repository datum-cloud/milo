package quota

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/endpoints/request"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// CELEngine evaluates CEL expressions with proper context.
type CELEngine interface {
	// EvaluateExpression evaluates a CEL expression with the given context.
	EvaluateExpression(ctx context.Context, expression string, evalContext *EvaluationContext) (interface{}, error)
	// ValidateExpression validates that a CEL expression is syntactically correct.
	ValidateExpression(expression string, expectedType cel.Type) error
}

// EvaluationContext provides context variables for CEL expression evaluation.
type EvaluationContext struct {
	// Object is the Kubernetes object being created.
	Object *unstructured.Unstructured
	// User contains information about the user making the request.
	User UserContext
	// RequestInfo contains information about the request parsed from the HTTP request.
	RequestInfo *request.RequestInfo
	// Namespace is the namespace where the object is being created.
	Namespace string
	// GVK is the GroupVersionKind of the object.
	GVK schema.GroupVersionKind
}

// UserContext provides user information for CEL expressions.
type UserContext struct {
	// Name is the username.
	Name string
	// UID is the user's unique identifier.
	UID string
	// Groups are the groups the user belongs to.
	Groups []string
	// Extra contains additional user attributes.
	Extra map[string][]string
}


// celEngine implements CELEngine using Google CEL-Go.
type celEngine struct {
	env    *cel.Env
	logger logr.Logger
}

// NewCELEngine creates a new CEL expression evaluation engine.
func NewCELEngine(logger logr.Logger) (CELEngine, error) {
	// Create CEL environment with standard macros and functions
	env, err := cel.NewEnv(
		// Object-related variables
		cel.Variable("object", cel.DynType),
		// User-related variables
		cel.Variable("user", cel.ObjectType("user")),
		cel.Variable("user.name", cel.StringType),
		cel.Variable("user.uid", cel.StringType),
		cel.Variable("user.groups", cel.ListType(cel.StringType)),
		cel.Variable("user.extra", cel.MapType(cel.StringType, cel.ListType(cel.StringType))),
		// RequestInfo-related variables
		cel.Variable("requestInfo", cel.ObjectType("requestInfo")),
		cel.Variable("requestInfo.verb", cel.StringType),
		cel.Variable("requestInfo.resource", cel.StringType),
		cel.Variable("requestInfo.subresource", cel.StringType),
		cel.Variable("requestInfo.name", cel.StringType),
		cel.Variable("requestInfo.namespace", cel.StringType),
		cel.Variable("requestInfo.apiGroup", cel.StringType),
		cel.Variable("requestInfo.apiVersion", cel.StringType),
		// Context variables
		cel.Variable("namespace", cel.StringType),
		cel.Variable("gvk", cel.ObjectType("gvk")),
		cel.Variable("gvk.group", cel.StringType),
		cel.Variable("gvk.version", cel.StringType),
		cel.Variable("gvk.kind", cel.StringType),
		// Enable standard macros like has(), size(), etc.
		cel.Lib(&StandardLibrary{}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	return &celEngine{
		env:    env,
		logger: logger.WithName("cel-engine"),
	}, nil
}

// EvaluateExpression evaluates a CEL expression with the given context.
func (e *celEngine) EvaluateExpression(ctx context.Context, expression string, evalCtx *EvaluationContext) (interface{}, error) {
	if expression == "" {
		return nil, fmt.Errorf("expression cannot be empty")
	}

	// Parse the expression
	ast, issues := e.env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to parse CEL expression '%s': %w", expression, issues.Err())
	}

	// Type-check the expression
	checked, issues := e.env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return nil, fmt.Errorf("failed to type-check CEL expression '%s': %w", expression, issues.Err())
	}

	// Create program
	program, err := e.env.Program(checked)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL program for expression '%s': %w", expression, err)
	}

	// Prepare evaluation context
	vars, err := e.buildVariableMap(evalCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to build variable map for expression '%s': %w", expression, err)
	}

	// Evaluate the expression
	result, _, err := program.Eval(vars)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate CEL expression '%s': %w", expression, err)
	}

	// Convert CEL result to Go value
	return e.convertCELValue(result)
}

// ValidateExpression validates that a CEL expression is syntactically correct.
func (e *celEngine) ValidateExpression(expression string, expectedType cel.Type) error {
	if expression == "" {
		return fmt.Errorf("expression cannot be empty")
	}

	// Parse the expression
	ast, issues := e.env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("failed to parse CEL expression '%s': %w", expression, issues.Err())
	}

	// Type-check the expression
	_, issues = e.env.Check(ast)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("failed to type-check CEL expression '%s': %w", expression, issues.Err())
	}

	// Check if the result type matches expected type (if specified)
	// Note: Type checking is handled by CEL's type checker above
	_ = expectedType // unused for now

	return nil
}

// buildVariableMap creates a map of variables for CEL evaluation.
func (e *celEngine) buildVariableMap(evalCtx *EvaluationContext) (map[string]interface{}, error) {
	vars := make(map[string]interface{})

	// Object variable - convert to map for easier access
	if evalCtx.Object != nil {
		vars["object"] = evalCtx.Object.Object
	}

	// User variables (as individual top-level variables)
	vars["user.name"] = evalCtx.User.Name
	vars["user.uid"] = evalCtx.User.UID
	vars["user.groups"] = evalCtx.User.Groups
	vars["user.extra"] = evalCtx.User.Extra

	// RequestInfo variables (as individual top-level variables)
	if evalCtx.RequestInfo != nil {
		vars["requestInfo.verb"] = evalCtx.RequestInfo.Verb
		vars["requestInfo.resource"] = evalCtx.RequestInfo.Resource
		vars["requestInfo.subresource"] = evalCtx.RequestInfo.Subresource
		vars["requestInfo.name"] = evalCtx.RequestInfo.Name
		vars["requestInfo.namespace"] = evalCtx.RequestInfo.Namespace
		vars["requestInfo.apiGroup"] = evalCtx.RequestInfo.APIGroup
		vars["requestInfo.apiVersion"] = evalCtx.RequestInfo.APIVersion
	}

	// Context variables
	vars["namespace"] = evalCtx.Namespace
	vars["gvk.group"] = evalCtx.GVK.Group
	vars["gvk.version"] = evalCtx.GVK.Version
	vars["gvk.kind"] = evalCtx.GVK.Kind

	return vars, nil
}

// convertCELValue converts a CEL ref.Val to a Go interface{}.
func (e *celEngine) convertCELValue(val ref.Val) (interface{}, error) {
	// Handle different CEL types
	switch val.Type() {
	case types.BoolType:
		return val.Value().(bool), nil
	case types.IntType:
		return val.Value().(int64), nil
	case types.UintType:
		return val.Value().(uint64), nil
	case types.DoubleType:
		return val.Value().(float64), nil
	case types.StringType:
		return val.Value().(string), nil
	case types.ListType:
		return val.Value(), nil
	case types.MapType:
		return val.Value(), nil
	default:
		// Try to get the underlying value
		if val.Value() != nil {
			return val.Value(), nil
		}
		return nil, fmt.Errorf("unsupported CEL value type: %s", val.Type())
	}
}

// BuildEvaluationContextFromAdmission builds an EvaluationContext from admission request.
func BuildEvaluationContextFromAdmission(req admission.Request, obj *unstructured.Unstructured) *EvaluationContext {
	// Convert Extra map from ExtraValue to []string
	extra := make(map[string][]string)
	for key, values := range req.UserInfo.Extra {
		extra[key] = []string(values)
	}

	user := UserContext{
		Name:   req.UserInfo.Username,
		UID:    req.UserInfo.UID,
		Groups: req.UserInfo.Groups,
		Extra:  extra,
	}

	// Build RequestInfo from admission request data
	// Map admission operation to HTTP verb equivalents
	verb := strings.ToLower(string(req.Operation))
	
	requestInfo := &request.RequestInfo{
		IsResourceRequest: true,
		Verb:             verb,
		APIGroup:         req.Kind.Group,
		APIVersion:       req.Kind.Version,
		Namespace:        req.Namespace,
		Resource:         strings.ToLower(req.Kind.Kind) + "s", // Pluralize kind for resource
		Subresource:      req.SubResource,
		Name:             req.Name,
	}

	return &EvaluationContext{
		Object:      obj,
		User:        user,
		RequestInfo: requestInfo,
		Namespace:   req.Namespace,
		GVK: schema.GroupVersionKind{
			Group:   req.Kind.Group,
			Version: req.Kind.Version,
			Kind:    req.Kind.Kind,
		},
	}
}

// StandardLibrary provides additional CEL functions and macros.
type StandardLibrary struct{}

func (lib *StandardLibrary) CompileOptions() []cel.EnvOption {
	return []cel.EnvOption{
		cel.Function("contains",
			cel.MemberOverload("list_contains", []*cel.Type{cel.ListType(cel.StringType), cel.StringType}, cel.BoolType,
				cel.FunctionBinding(func(args ...ref.Val) ref.Val {
					if len(args) != 2 {
						return types.NewErr("contains requires exactly 2 arguments")
					}
					list := args[0]
					item := args[1]

					listVal, ok := list.Value().([]interface{})
					if !ok {
						return types.NewErr("first argument must be a list")
					}

					itemVal := item.Value()
					for _, listItem := range listVal {
						if reflect.DeepEqual(listItem, itemVal) {
							return types.True
						}
					}
					return types.False
				})),
		),
	}
}

func (lib *StandardLibrary) ProgramOptions() []cel.ProgramOption {
	return []cel.ProgramOption{}
}
