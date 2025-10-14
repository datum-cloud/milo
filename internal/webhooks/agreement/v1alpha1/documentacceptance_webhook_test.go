package v1alpha1

import (
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agreementv1alpha1 "go.miloapis.com/milo/pkg/apis/agreement/v1alpha1"
	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
)

func TestDocumentAcceptanceValidator_ValidateCreate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = agreementv1alpha1.AddToScheme(scheme)
	_ = documentationv1alpha1.AddToScheme(scheme)
	_ = iamv1alpha1.AddToScheme(scheme)

	now := metav1.Now()

	// Base resources reused across tests
	baseRevision := &documentationv1alpha1.DocumentRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tos-v1",
			Namespace: "default",
		},
		Spec: documentationv1alpha1.DocumentRevisionSpec{
			DocumentRef: documentationv1alpha1.DocumentReference{
				Name:      "tos",
				Namespace: "default",
			},
			Version:       "v1.0.0",
			EffectiveDate: metav1.Time{Time: now.Add(24 * time.Hour)},
			Content: documentationv1alpha1.DocumentRevisionContent{
				Format: "markdown",
				Data:   "lorem ipsum",
			},
			ChangesSummary:        "initial version",
			ExpectedSubjectKinds:  []documentationv1alpha1.DocumentRevisionExpectedSubjectKind{{APIGroup: "resourcemanager.miloapis.com", Kind: "Organization"}},
			ExpectedAccepterKinds: []documentationv1alpha1.DocumentRevisionExpectedAccepterKind{{APIGroup: "iam.miloapis.com", Kind: "User"}},
		},
	}

	baseUser := &iamv1alpha1.User{
		ObjectMeta: metav1.ObjectMeta{
			Name: "alice",
		},
		Spec: iamv1alpha1.UserSpec{Email: "alice@example.com"},
	}

	validAcceptance := &agreementv1alpha1.DocumentAcceptance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tos-acceptance",
			Namespace: "default",
		},
		Spec: agreementv1alpha1.DocumentAcceptanceSpec{
			DocumentRevisionRef: documentationv1alpha1.DocumentRevisionReference{
				Name:      baseRevision.Name,
				Namespace: baseRevision.Namespace,
				Version:   baseRevision.Spec.Version,
			},
			SubjectRef: agreementv1alpha1.ResourceReference{
				APIGroup: "resourcemanager.miloapis.com",
				Kind:     "Organization",
				Name:     "acme",
			},
			AccepterRef: agreementv1alpha1.ResourceReference{
				APIGroup: "iam.miloapis.com",
				Kind:     "User",
				Name:     baseUser.Name,
			},
			AcceptanceContext: agreementv1alpha1.DocumentAcceptanceContext{Method: "web"},
			Signature:         agreementv1alpha1.DocumentAcceptanceSignature{Type: "checkbox", Timestamp: now},
		},
	}

	tests := []struct {
		name      string
		objects   []runtime.Object
		da        *agreementv1alpha1.DocumentAcceptance
		wantError bool
	}{
		{
			name:      "valid acceptance",
			objects:   []runtime.Object{baseRevision.DeepCopy(), baseUser.DeepCopy()},
			da:        validAcceptance.DeepCopy(),
			wantError: false,
		},
		{
			name:      "document revision not found",
			objects:   []runtime.Object{baseUser.DeepCopy()},
			da:        validAcceptance.DeepCopy(),
			wantError: true,
		},
		{
			name:    "version mismatch",
			objects: []runtime.Object{baseRevision.DeepCopy(), baseUser.DeepCopy()},
			da: func() *agreementv1alpha1.DocumentAcceptance {
				v := validAcceptance.DeepCopy()
				v.Spec.DocumentRevisionRef.Version = "v0.9.0"
				return v
			}(),
			wantError: true,
		},
		{
			name:    "unexpected subject kind",
			objects: []runtime.Object{baseRevision.DeepCopy(), baseUser.DeepCopy()},
			da: func() *agreementv1alpha1.DocumentAcceptance {
				v := validAcceptance.DeepCopy()
				v.Spec.SubjectRef.Kind = "Project"
				return v
			}(),
			wantError: true,
		},
		{
			name:    "unexpected accepter kind",
			objects: []runtime.Object{baseRevision.DeepCopy(), baseUser.DeepCopy()},
			da: func() *agreementv1alpha1.DocumentAcceptance {
				v := validAcceptance.DeepCopy()
				v.Spec.AccepterRef.Kind = "MachineAccount"
				return v
			}(),
			wantError: true,
		},
		{
			name:      "accepter object not found",
			objects:   []runtime.Object{baseRevision.DeepCopy()},
			da:        validAcceptance.DeepCopy(),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objects...).Build()
			v := &DocumentAcceptanceValidator{Client: c}
			_, err := v.ValidateCreate(context.TODO(), tt.da)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !apierrors.IsInvalid(err) {
					t.Fatalf("expected admission invalid error, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
