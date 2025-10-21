package documents

import (
	"context"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"

	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
)

func TestDocumentController_Reconcile_LatestRevisionRefUpdated(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add client-go scheme: %v", err)
	}
	if err := documentationv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add documentation scheme: %v", err)
	}

	// Build fake client that supports status subresource updates
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&documentationv1alpha1.Document{}).
		WithIndex(&documentationv1alpha1.DocumentRevision{}, documentRefNamespacedKey, func(obj client.Object) []string {
			dr, ok := obj.(*documentationv1alpha1.DocumentRevision)
			if !ok {
				return nil
			}
			return []string{buildDocumentRevisionByDocumentIndexKey(dr.Spec.DocumentRef)}
		}).
		Build()

	ctx := context.TODO()

	// Create a sample Document in the fake cluster
	doc := &documentationv1alpha1.Document{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-document",
			Namespace: "default",
		},
		Spec: documentationv1alpha1.DocumentSpec{
			Title:        "TOS",
			Description:  "Terms of Service",
			DocumentType: "tos",
		},
		Metadata: documentationv1alpha1.DocumentMetadata{
			Category:     "legal",
			Jurisdiction: "us",
		},
	}
	if err := fakeClient.Create(ctx, doc); err != nil {
		t.Fatalf("failed to create document: %v", err)
	}

	r := &DocumentController{
		Client:     fakeClient,
		Finalizers: finalizer.NewFinalizers(),
	}

	req := ctrl.Request{NamespacedName: client.ObjectKey{Name: doc.Name, Namespace: doc.Namespace}}

	// First reconcile: no revisions exist yet -> LatestRevisionRef should stay nil
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if err := fakeClient.Get(ctx, req.NamespacedName, doc); err != nil {
		t.Fatalf("failed to get document after reconcile: %v", err)
	}
	if doc.Status.LatestRevisionRef != nil {
		t.Fatalf("expected LatestRevisionRef to be nil, got %v", doc.Status.LatestRevisionRef)
	}

	// Verify Ready condition is present and set to True with reason Reconciled
	cond := meta.FindStatusCondition(doc.Status.Conditions, "Ready")
	if cond == nil {
		t.Fatalf("expected Ready condition to be present")
	}
	if cond.Status != metav1.ConditionTrue || cond.Reason != "Reconciled" {
		t.Fatalf("unexpected Ready condition: %+v", cond)
	}

	// Create first DocumentRevision v1.0.0
	rev1 := &documentationv1alpha1.DocumentRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-document-v1",
			Namespace: "default",
		},
		Spec: documentationv1alpha1.DocumentRevisionSpec{
			DocumentRef:           documentationv1alpha1.DocumentReference{Name: doc.Name, Namespace: doc.Namespace},
			Version:               "v1.0.0",
			EffectiveDate:         metav1.Time{Time: time.Now()},
			Content:               documentationv1alpha1.DocumentRevisionContent{Format: "markdown", Data: "initial content"},
			ChangesSummary:        "initial version",
			ExpectedSubjectKinds:  []documentationv1alpha1.DocumentRevisionExpectedSubjectKind{{APIGroup: "", Kind: ""}},
			ExpectedAccepterKinds: []documentationv1alpha1.DocumentRevisionExpectedAccepterKind{{APIGroup: "iam.miloapis.com", Kind: "User"}},
		},
	}
	if err := fakeClient.Create(ctx, rev1); err != nil {
		t.Fatalf("failed to create document revision 1: %v", err)
	}

	// Reconcile again -> LatestRevisionRef should be updated to v1.0.0
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if err := fakeClient.Get(ctx, req.NamespacedName, doc); err != nil {
		t.Fatalf("failed to get document after reconcile 2: %v", err)
	}
	if doc.Status.LatestRevisionRef == nil || string(doc.Status.LatestRevisionRef.Version) != "v1.0.0" {
		t.Fatalf("expected LatestRevisionRef version v1.0.0, got %v", doc.Status.LatestRevisionRef)
	}

	// Create second DocumentRevision v1.1.0 (higher)
	rev2 := &documentationv1alpha1.DocumentRevision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-document-v2",
			Namespace: "default",
		},
		Spec: documentationv1alpha1.DocumentRevisionSpec{
			DocumentRef:           documentationv1alpha1.DocumentReference{Name: doc.Name, Namespace: doc.Namespace},
			Version:               "v1.1.0",
			EffectiveDate:         metav1.Time{Time: time.Now()},
			Content:               documentationv1alpha1.DocumentRevisionContent{Format: "markdown", Data: "updated content"},
			ChangesSummary:        "update",
			ExpectedSubjectKinds:  []documentationv1alpha1.DocumentRevisionExpectedSubjectKind{{APIGroup: "", Kind: ""}},
			ExpectedAccepterKinds: []documentationv1alpha1.DocumentRevisionExpectedAccepterKind{{APIGroup: "iam.miloapis.com", Kind: "User"}},
		},
	}
	if err := fakeClient.Create(ctx, rev2); err != nil {
		t.Fatalf("failed to create document revision 2: %v", err)
	}

	// Reconcile again -> LatestRevisionRef should be updated to v1.1.0
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if err := fakeClient.Get(ctx, req.NamespacedName, doc); err != nil {
		t.Fatalf("failed to get document after reconcile 3: %v", err)
	}
	if doc.Status.LatestRevisionRef == nil || string(doc.Status.LatestRevisionRef.Version) != "v1.1.0" {
		t.Fatalf("expected LatestRevisionRef version v1.1.0, got %v", doc.Status.LatestRevisionRef.Version)
	}
}
