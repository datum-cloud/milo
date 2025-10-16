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

	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
)

func TestDocumentRevisionValidator_ValidateCreate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = documentationv1alpha1.AddToScheme(scheme)

	now := time.Now()
	future := metav1.NewTime(now.Add(24 * time.Hour))
	past := metav1.NewTime(now.Add(-1 * time.Hour))

	baseDoc := &documentationv1alpha1.Document{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "doc",
			Namespace: "default",
		},
		Status: documentationv1alpha1.DocumentStatus{
			LatestRevisionRef: &documentationv1alpha1.LatestRevisionRef{
				Version: "v1.0.0",
			},
		},
	}

	tests := []struct {
		name      string
		objects   []runtime.Object
		dr        *documentationv1alpha1.DocumentRevision
		wantError bool
	}{
		{
			name:    "valid revision",
			objects: []runtime.Object{baseDoc.DeepCopy()},
			dr: &documentationv1alpha1.DocumentRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rev1",
					Namespace: "default",
				},
				Spec: documentationv1alpha1.DocumentRevisionSpec{
					DocumentRef: documentationv1alpha1.DocumentReference{
						Name:      "doc",
						Namespace: "default",
					},
					Version:       "v1.0.1",
					EffectiveDate: future,
					Content: documentationv1alpha1.DocumentRevisionContent{
						Format: "markdown",
						Data:   "some data",
					},
					ChangesSummary:        "changes",
					ExpectedSubjectKinds:  []documentationv1alpha1.DocumentRevisionExpectedSubjectKind{{APIGroup: "test", Kind: "Kind"}},
					ExpectedAccepterKinds: []documentationv1alpha1.DocumentRevisionExpectedAccepterKind{{APIGroup: "test", Kind: "Kind"}},
				},
			},
			wantError: false,
		},
		{
			name:    "document not found",
			objects: []runtime.Object{},
			dr: &documentationv1alpha1.DocumentRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rev2",
					Namespace: "default",
				},
				Spec: documentationv1alpha1.DocumentRevisionSpec{
					DocumentRef: documentationv1alpha1.DocumentReference{
						Name:      "doc",
						Namespace: "default",
					},
					Version:               "v1.0.1",
					EffectiveDate:         future,
					Content:               documentationv1alpha1.DocumentRevisionContent{Format: "markdown", Data: "x"},
					ChangesSummary:        "changes",
					ExpectedSubjectKinds:  []documentationv1alpha1.DocumentRevisionExpectedSubjectKind{{APIGroup: "test", Kind: "Kind"}},
					ExpectedAccepterKinds: []documentationv1alpha1.DocumentRevisionExpectedAccepterKind{{APIGroup: "test", Kind: "Kind"}},
				},
			},
			wantError: true,
		},
		{
			name:    "version not higher",
			objects: []runtime.Object{baseDoc.DeepCopy()},
			dr: &documentationv1alpha1.DocumentRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rev3",
					Namespace: "default",
				},
				Spec: documentationv1alpha1.DocumentRevisionSpec{
					DocumentRef:           documentationv1alpha1.DocumentReference{Name: "doc", Namespace: "default"},
					Version:               "v0.9.0",
					EffectiveDate:         future,
					Content:               documentationv1alpha1.DocumentRevisionContent{Format: "markdown", Data: "x"},
					ChangesSummary:        "changes",
					ExpectedSubjectKinds:  []documentationv1alpha1.DocumentRevisionExpectedSubjectKind{{APIGroup: "test", Kind: "Kind"}},
					ExpectedAccepterKinds: []documentationv1alpha1.DocumentRevisionExpectedAccepterKind{{APIGroup: "test", Kind: "Kind"}},
				},
			},
			wantError: true,
		},
		{
			name:    "effective date not future",
			objects: []runtime.Object{baseDoc.DeepCopy()},
			dr: &documentationv1alpha1.DocumentRevision{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "rev4",
					Namespace: "default",
				},
				Spec: documentationv1alpha1.DocumentRevisionSpec{
					DocumentRef:           documentationv1alpha1.DocumentReference{Name: "doc", Namespace: "default"},
					Version:               "v1.0.1",
					EffectiveDate:         past,
					Content:               documentationv1alpha1.DocumentRevisionContent{Format: "markdown", Data: "x"},
					ChangesSummary:        "changes",
					ExpectedSubjectKinds:  []documentationv1alpha1.DocumentRevisionExpectedSubjectKind{{APIGroup: "test", Kind: "Kind"}},
					ExpectedAccepterKinds: []documentationv1alpha1.DocumentRevisionExpectedAccepterKind{{APIGroup: "test", Kind: "Kind"}},
				},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tt.objects...).Build()
			v := &DocumentRevisionValidator{Client: c}
			_, err := v.ValidateCreate(context.TODO(), tt.dr)
			if tt.wantError {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if !apierrors.IsInvalid(err) && !apierrors.IsNotFound(err) {
					t.Fatalf("expected admission invalid/notfound error, got %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
