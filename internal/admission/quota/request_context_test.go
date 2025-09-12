package quota

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/request"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	quotav1alpha1 "go.miloapis.com/milo/pkg/apis/quota/v1alpha1"
)

func TestRequestContextIntegration(t *testing.T) {
	tests := []struct {
		name             string
		policy           *quotav1alpha1.ClaimCreationPolicy
		obj              *unstructured.Unstructured
		user             user.Info
		operation        admission.Operation
		subResource      string
		dryRun           bool
		expectedRequests int
		validateContext  func(t *testing.T, ctx *EvaluationContext)
	}{
		{
			name: "basic request context populated",
			policy: &quotav1alpha1.ClaimCreationPolicy{
				Spec: quotav1alpha1.ClaimCreationPolicySpec{
					TargetResource: quotav1alpha1.TargetResource{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					Enabled: boolPtr(true),
					ResourceClaimTemplate: quotav1alpha1.ResourceClaimTemplateSpec{
						Requests: []quotav1alpha1.ResourceRequestTemplate{
							{
								ResourceType: "apps/Deployment",
								Amount:       int64Ptr(1),
							},
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
						"labels": map[string]interface{}{
							"tier": "premium",
							"org":  "acme-corp",
						},
					},
					"spec": map[string]interface{}{
						"replicas": int64(3),
					},
				},
			},
			user: &user.DefaultInfo{
				Name:   "test-user",
				UID:    "test-uid-123",
				Groups: []string{"premium-users", "developers"},
				Extra: map[string][]string{
					"department": {"engineering"},
				},
			},
			operation:        admission.Create,
			subResource:      "",
			dryRun:           false,
			expectedRequests: 1,
			validateContext: func(t *testing.T, ctx *EvaluationContext) {
				// Validate user context
				if ctx.User.Name != "test-user" {
					t.Errorf("Expected user name 'test-user', got '%s'", ctx.User.Name)
				}
				if ctx.User.UID != "test-uid-123" {
					t.Errorf("Expected user UID 'test-uid-123', got '%s'", ctx.User.UID)
				}
				if len(ctx.User.Groups) != 2 {
					t.Errorf("Expected 2 user groups, got %d", len(ctx.User.Groups))
				}

				// Validate RequestInfo context
				if ctx.RequestInfo.Verb != "create" {
					t.Errorf("Expected verb 'create', got '%s'", ctx.RequestInfo.Verb)
				}
				if ctx.RequestInfo.Subresource != "" {
					t.Errorf("Expected empty subresource, got '%s'", ctx.RequestInfo.Subresource)
				}
				if ctx.RequestInfo.APIGroup != "apps" {
					t.Errorf("Expected API group 'apps', got '%s'", ctx.RequestInfo.APIGroup)
				}

				// Validate object context
				if ctx.Object == nil {
					t.Error("Expected object context to be populated")
					return
				}
				if ctx.Object.GetName() != "test-deployment" {
					t.Errorf("Expected object name 'test-deployment', got '%s'", ctx.Object.GetName())
				}

				// Validate GVK context
				if ctx.GVK.Kind != "Deployment" {
					t.Errorf("Expected kind 'Deployment', got '%s'", ctx.GVK.Kind)
				}
			},
		},
		{
			name: "subresource and dry run context",
			policy: &quotav1alpha1.ClaimCreationPolicy{
				Spec: quotav1alpha1.ClaimCreationPolicySpec{
					TargetResource: quotav1alpha1.TargetResource{
						APIVersion: "apps/v1",
						Kind:       "Deployment",
					},
					Enabled: boolPtr(true),
					ResourceClaimTemplate: quotav1alpha1.ResourceClaimTemplateSpec{
						Requests: []quotav1alpha1.ResourceRequestTemplate{
							{
								ResourceType: "apps/Deployment",
								Amount:       int64Ptr(1),
							},
						},
					},
				},
			},
			obj: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "apps/v1",
					"kind":       "Deployment",
					"metadata": map[string]interface{}{
						"name":      "test-deployment",
						"namespace": "default",
					},
				},
			},
			user: &user.DefaultInfo{
				Name: "test-user",
			},
			operation:        admission.Update,
			subResource:      "scale",
			dryRun:           true,
			expectedRequests: 0, // Should skip due to dry run
			validateContext: func(t *testing.T, ctx *EvaluationContext) {
				if ctx.RequestInfo.Verb != "update" {
					t.Errorf("Expected verb 'update', got '%s'", ctx.RequestInfo.Verb)
				}
				if ctx.RequestInfo.Subresource != "scale" {
					t.Errorf("Expected subresource 'scale', got '%s'", ctx.RequestInfo.Subresource)
				}
				if ctx.RequestInfo.APIGroup != "apps" {
					t.Errorf("Expected API group 'apps', got '%s'", ctx.RequestInfo.APIGroup)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create logger
			logger := zap.New(zap.UseDevMode(true))

			// Create plugin
			plugin := &ClaimCreationPlugin{
				Handler: admission.NewHandler(admission.Create, admission.Update),
				logger:  logger.WithName("plugin"),
			}

			// Create test admission attributes
			attrs := &testAdmissionAttributes{
				operation:   tt.operation,
				object:      tt.obj,
				gvk:         schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
				name:        tt.obj.GetName(),
				namespace:   tt.obj.GetNamespace(),
				userInfo:    tt.user,
				dryRun:      tt.dryRun,
				subResource: tt.subResource,
			}

			// Call buildEvaluationContext directly to test context building
			evalContext := plugin.buildEvaluationContext(attrs, tt.obj)

			// Validate the context
			if tt.validateContext != nil {
				tt.validateContext(t, evalContext)
			}
		})
	}
}

func TestCELExpressionWithRequestContext(t *testing.T) {
	// Helper function to create complete context
	createTestContext := func(requestInfo *request.RequestInfo, user UserContext) *EvaluationContext {
		return &EvaluationContext{
			Object:      &unstructured.Unstructured{},
			User:        user,
			RequestInfo: requestInfo,
			Namespace:   "default",
			GVK:         schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"},
		}
	}

	tests := []struct {
		name       string
		expression string
		context    *EvaluationContext
		expected   interface{}
		expectErr  bool
	}{
		{
			name:       "access request verb",
			expression: "requestInfo.verb",
			context: createTestContext(
				&request.RequestInfo{Verb: "create", Resource: "deployments", Subresource: ""},
				UserContext{Name: "test-user", UID: "test-uid", Groups: []string{}, Extra: map[string][]string{}},
			),
			expected: "create",
		},
		{
			name:       "access request subresource",
			expression: "requestInfo.subresource",
			context: createTestContext(
				&request.RequestInfo{Verb: "update", Resource: "deployments", Subresource: "scale"},
				UserContext{Name: "test-user", UID: "test-uid", Groups: []string{}, Extra: map[string][]string{}},
			),
			expected: "scale",
		},
		{
			name:       "access request resource",
			expression: "requestInfo.resource",
			context: createTestContext(
				&request.RequestInfo{Verb: "create", Resource: "deployments", Subresource: ""},
				UserContext{Name: "test-user", UID: "test-uid", Groups: []string{}, Extra: map[string][]string{}},
			),
			expected: "deployments",
		},
		{
			name:       "conditional based on verb",
			expression: "requestInfo.verb == 'create' ? 'new-resource' : 'existing-resource'",
			context: createTestContext(
				&request.RequestInfo{Verb: "create", Resource: "deployments", Subresource: ""},
				UserContext{Name: "test-user", UID: "test-uid", Groups: []string{}, Extra: map[string][]string{}},
			),
			expected: "new-resource",
		},
		{
			name:       "check if subresource is empty",
			expression: "requestInfo.subresource == ''",
			context: createTestContext(
				&request.RequestInfo{Verb: "create", Resource: "deployments", Subresource: ""},
				UserContext{Name: "test-user", UID: "test-uid", Groups: []string{}, Extra: map[string][]string{}},
			),
			expected: true,
		},
		{
			name:       "combine user groups and request verb",
			expression: "'premium' in user.groups && requestInfo.verb == 'create'",
			context: createTestContext(
				&request.RequestInfo{Verb: "create", Resource: "deployments", Subresource: ""},
				UserContext{Name: "test-user", UID: "test-uid", Groups: []string{"premium", "developers"}, Extra: map[string][]string{}},
			),
			expected: true,
		},
		{
			name:       "verify all context variables work",
			expression: "requestInfo.verb + ' by ' + user.name",
			context: createTestContext(
				&request.RequestInfo{Verb: "create", Resource: "deployments", Subresource: ""},
				UserContext{Name: "test-user", UID: "test-uid", Groups: []string{"premium"}, Extra: map[string][]string{}},
			),
			expected: "create by test-user",
		},
	}

	// Create CEL engine
	logger := zap.New(zap.UseDevMode(true))
	celEngine, err := NewCELEngine(logger.WithName("cel"))
	if err != nil {
		t.Fatalf("Failed to create CEL engine: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := celEngine.EvaluateExpression(context.Background(), tt.expression, tt.context)

			if tt.expectErr && err == nil {
				t.Error("Expected error but got none")
				return
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if !tt.expectErr && result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}
