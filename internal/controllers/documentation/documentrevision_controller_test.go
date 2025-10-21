package documents

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
	"go.miloapis.com/milo/pkg/util/hash"
)

func TestDocumentRevisionController_Reconcile_HashBehaviour(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add client-go scheme: %v", err)
	}
	if err := documentationv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add documentation scheme: %v", err)
	}

	// Build fake client that supports status subresource updates
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&documentationv1alpha1.DocumentRevision{}).
		Build()

	ctx := context.TODO()

	// Create a sample Document in the fake cluster (needed to satisfy reference, controller doesn't use it)
	doc := &documentationv1alpha1.Document{
		ObjectMeta: metav1.ObjectMeta{Name: "sample-document", Namespace: "default"},
		Spec: documentationv1alpha1.DocumentSpec{
			Title: "TOS", Description: "Terms", DocumentType: "tos",
		},
		Metadata: documentationv1alpha1.DocumentMetadata{Category: "legal", Jurisdiction: "us"},
	}
	if err := fakeClient.Create(ctx, doc); err != nil {
		t.Fatalf("failed to create document: %v", err)
	}

	// Initial content
	initialContent := "initial content"

	rev := &documentationv1alpha1.DocumentRevision{
		ObjectMeta: metav1.ObjectMeta{Name: "sample-document-v1", Namespace: "default"},
		Spec: documentationv1alpha1.DocumentRevisionSpec{
			DocumentRef:           documentationv1alpha1.DocumentReference{Name: doc.Name, Namespace: doc.Namespace},
			Version:               "v1.0.0",
			EffectiveDate:         metav1.Now(),
			Content:               documentationv1alpha1.DocumentRevisionContent{Format: "markdown", Data: initialContent},
			ChangesSummary:        "initial",
			ExpectedSubjectKinds:  []documentationv1alpha1.DocumentRevisionExpectedSubjectKind{{APIGroup: "", Kind: ""}},
			ExpectedAccepterKinds: []documentationv1alpha1.DocumentRevisionExpectedAccepterKind{{APIGroup: "iam.miloapis.com", Kind: "User"}},
		},
	}
	if err := fakeClient.Create(ctx, rev); err != nil {
		t.Fatalf("failed to create document revision: %v", err)
	}

	r := &DocumentRevisionController{Client: fakeClient}

	req := ctrl.Request{NamespacedName: client.ObjectKey{Name: rev.Name, Namespace: rev.Namespace}}

	// First reconcile should calculate and persist hash
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile failed: %v", err)
	}
	if err := fakeClient.Get(ctx, req.NamespacedName, rev); err != nil {
		t.Fatalf("failed to get revision after reconcile: %v", err)
	}
	expectedHash := hash.SHA256Hex(initialContent)
	if rev.Status.ContentHash != expectedHash {
		t.Fatalf("expected content hash %s, got %s", expectedHash, rev.Status.ContentHash)
	}

	// Modify spec content (controller should ignore because Ready=True)
	updatedContent := "modified content"
	rev.Spec.Content.Data = updatedContent
	if err := fakeClient.Update(ctx, rev); err != nil {
		t.Fatalf("failed to update revision content: %v", err)
	}

	// Reconcile again - hash should remain unchanged
	if _, err := r.Reconcile(ctx, req); err != nil {
		t.Fatalf("reconcile 2 failed: %v", err)
	}
	if err := fakeClient.Get(ctx, req.NamespacedName, rev); err != nil {
		t.Fatalf("failed to get revision after reconcile 2: %v", err)
	}
	if rev.Status.ContentHash != expectedHash {
		t.Fatalf("content hash changed unexpectedly: expected %s got %s", expectedHash, rev.Status.ContentHash)
	}
}
