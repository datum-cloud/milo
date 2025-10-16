package v1alpha1

import (
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	documentationv1alpha1 "go.miloapis.com/milo/pkg/apis/documentation/v1alpha1"
)

func TestDocumentValidator_ValidateDelete(t *testing.T) {
	ctx := context.TODO()
	validator := &DocumentValidator{}

	t.Run("allowed when no latest revision", func(t *testing.T) {
		doc := &documentationv1alpha1.Document{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Document",
				APIVersion: "documentation.miloapis.com/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "terms-of-service",
				Namespace: "default",
			},
			Status: documentationv1alpha1.DocumentStatus{}, // LatestRevisionRef is nil
		}

		if _, err := validator.ValidateDelete(ctx, doc); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("denied when latest revision exists", func(t *testing.T) {
		doc := &documentationv1alpha1.Document{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Document",
				APIVersion: "documentation.miloapis.com/v1alpha1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "privacy-policy",
				Namespace: "default",
			},
			Status: documentationv1alpha1.DocumentStatus{
				LatestRevisionRef: &documentationv1alpha1.LatestRevisionRef{
					Name:        "privacy-policy-v1.0.0",
					Namespace:   "default",
					Version:     documentationv1alpha1.DocumentVersion("v1.0.0"),
					PublishedAt: metav1.Now(),
				},
			},
		}

		if _, err := validator.ValidateDelete(ctx, doc); err == nil {
			t.Fatalf("expected error, got nil")
		} else if !apierrors.IsBadRequest(err) {
			t.Fatalf("expected BadRequest error, got %v", err)
		}
	})
}
