package v1alpha1

import (
	"context"
	"testing"
	"time"

	"regexp"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agreementv1alpha1 "go.miloapis.com/milo/pkg/apis/agreement/v1alpha1"
	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

func TestDocumentAcceptanceValidator_ValidateCreate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = agreementv1alpha1.AddToScheme(scheme)
	_ = documentationv1alpha1.AddToScheme(scheme)
	_ = iamv1alpha1.AddToScheme(scheme)
	_ = resourcemanagerv1alpha1.AddToScheme(scheme)

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

	baseOrg := &resourcemanagerv1alpha1.Organization{
		ObjectMeta: metav1.ObjectMeta{
			Name: "acme",
		},
		Spec: resourcemanagerv1alpha1.OrganizationSpec{
			Type: "Standard",
		},
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
		errRegex  string
	}{
		{
			name:      "valid acceptance",
			objects:   []runtime.Object{baseRevision.DeepCopy(), baseUser.DeepCopy(), baseOrg.DeepCopy()},
			da:        validAcceptance.DeepCopy(),
			wantError: false,
		},
		{
			name:      "document revision not found",
			objects:   []runtime.Object{baseUser.DeepCopy(), baseOrg.DeepCopy()},
			da:        validAcceptance.DeepCopy(),
			wantError: true,
			errRegex:  "spec.documentRevisionRef",
		},
		{
			name:    "version mismatch",
			objects: []runtime.Object{baseRevision.DeepCopy(), baseUser.DeepCopy(), baseOrg.DeepCopy()},
			da: func() *agreementv1alpha1.DocumentAcceptance {
				v := validAcceptance.DeepCopy()
				v.Spec.DocumentRevisionRef.Version = "v0.9.0"
				return v
			}(),
			wantError: true,
			errRegex:  "spec.documentRevisionRef.version",
		},
		{
			name:    "unexpected subject kind",
			objects: []runtime.Object{baseRevision.DeepCopy(), baseUser.DeepCopy(), baseOrg.DeepCopy()},
			da: func() *agreementv1alpha1.DocumentAcceptance {
				v := validAcceptance.DeepCopy()
				v.Spec.SubjectRef.Kind = "Project"
				return v
			}(),
			wantError: true,
			errRegex:  "spec.subjectRef",
		},
		{
			name:    "unexpected accepter kind",
			objects: []runtime.Object{baseRevision.DeepCopy(), baseUser.DeepCopy(), baseOrg.DeepCopy()},
			da: func() *agreementv1alpha1.DocumentAcceptance {
				v := validAcceptance.DeepCopy()
				v.Spec.AccepterRef.Kind = "MachineAccount"
				return v
			}(),
			wantError: true,
			errRegex:  "spec.accepterRef",
		},
		{
			name: "duplicate acceptance",
			objects: func() []runtime.Object {
				existing := validAcceptance.DeepCopy()
				existing.ObjectMeta = metav1.ObjectMeta{ // ensure distinct name but same spec
					Name:      "tos-acceptance-existing",
					Namespace: "default",
				}
				return []runtime.Object{baseRevision.DeepCopy(), baseUser.DeepCopy(), baseOrg.DeepCopy(), existing}
			}(),
			da: func() *agreementv1alpha1.DocumentAcceptance {
				dup := validAcceptance.DeepCopy()
				dup.ObjectMeta = metav1.ObjectMeta{
					Name:      "tos-acceptance-dup",
					Namespace: "default",
				}
				return dup
			}(),
			wantError: true,
			errRegex:  "same documentRevisionRef and subjectRef already exists",
		},
		{
			name:      "accepter object not found",
			objects:   []runtime.Object{baseRevision.DeepCopy(), baseOrg.DeepCopy()},
			da:        validAcceptance.DeepCopy(),
			wantError: true,
			errRegex:  "spec.accepterRef.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			builder := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objects...)
			// register the same index defined in production code
			builder = builder.WithIndex(&agreementv1alpha1.DocumentAcceptance{}, daIndexKey, func(obj client.Object) []string {
				da := obj.(*agreementv1alpha1.DocumentAcceptance)
				return []string{buildDaIndexKey(*da)}
			})
			c := builder.Build()
			v := &DocumentAcceptanceValidator{Client: c}
			_, err := v.ValidateCreate(context.TODO(), tt.da)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !apierrors.IsInvalid(err) {
					t.Fatalf("expected admission invalid error, got %v", err)
				}
				if tt.errRegex != "" {
					if !regexp.MustCompile(tt.errRegex).MatchString(err.Error()) {
						t.Fatalf("error message %q did not match %q", err.Error(), tt.errRegex)
					}
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
