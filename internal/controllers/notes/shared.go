package notes

import (
	"context"
	"fmt"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	noteReadyConditionType   = "Ready"
	noteReadyConditionReason = "Reconciled"

	noteManagedByLabel = "notes.miloapis.com/managed-by"
	noteManagedByValue = "note-controller"
	noteNameLabel      = "notes.miloapis.com/note-name"
	noteNamespaceLabel = "notes.miloapis.com/note-namespace"
	noteUIDLabel       = "notes.miloapis.com/note-uid"
	noteFinalizer      = "notes.miloapis.com/policybinding-cleanup"
)

// NoteResource is an interface that both Note and ClusterNote implement
type NoteResource interface {
	client.Object
	GetCreatorRef() iamv1alpha1.UserReference
	GetNoteKind() string
}

// ensureCreatorEditorPolicyBinding creates or checks a PolicyBinding for the note creator
func ensureCreatorEditorPolicyBinding(
	ctx context.Context,
	c client.Client,
	scheme *runtime.Scheme,
	noteResource NoteResource,
	creator *iamv1alpha1.User,
	creatorEditorRoleName string,
	creatorEditorRoleNamespace string,
) (bool, string, error) {
	log := log.FromContext(ctx)

	bindingName := fmt.Sprintf("note-creator-editor-%s", noteResource.GetName())
	if noteResource.GetNamespace() != "" {
		bindingName = fmt.Sprintf("note-creator-editor-%s-%s", noteResource.GetNamespace(), noteResource.GetName())
	}

	var existing iamv1alpha1.PolicyBinding
	if err := c.Get(ctx, types.NamespacedName{Name: bindingName, Namespace: creatorEditorRoleNamespace}, &existing); err == nil {
		return isPolicyBindingReady(&existing)
	} else if !apierrors.IsNotFound(err) {
		return false, "", fmt.Errorf("failed to get existing creator PolicyBinding: %w", err)
	}

	// Build labels to track which Note owns this PolicyBinding
	labels := map[string]string{
		noteManagedByLabel: noteManagedByValue,
		noteNameLabel:      noteResource.GetName(),
		noteUIDLabel:       string(noteResource.GetUID()),
	}
	if noteResource.GetNamespace() != "" {
		labels[noteNamespaceLabel] = noteResource.GetNamespace()
	}

	policyBinding := &iamv1alpha1.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: creatorEditorRoleNamespace,
			Labels:    labels,
		},
		Spec: iamv1alpha1.PolicyBindingSpec{
			RoleRef: iamv1alpha1.RoleReference{
				Name:      creatorEditorRoleName,
				Namespace: creatorEditorRoleNamespace,
			},
			Subjects: []iamv1alpha1.Subject{
				{
					Kind: "User",
					Name: creator.Name,
					UID:  string(creator.UID),
				},
			},
			ResourceSelector: iamv1alpha1.ResourceSelector{
				ResourceRef: &iamv1alpha1.ResourceReference{
					APIGroup: "notes.miloapis.com",
					Kind:     noteResource.GetNoteKind(),
					Name:     noteResource.GetName(),
					UID:      string(noteResource.GetUID()),
				},
			},
		},
	}

	// Note: We don't set owner reference here because PolicyBindings are in milo-system
	// and Notes can be in any namespace. Kubernetes doesn't allow cross-namespace owner references.
	// Instead, we use labels to track ownership and clean up manually via finalizers.

	log.Info("Creating creator PolicyBinding", "policyBinding", bindingName, "user", creator.Name)
	if err := c.Create(ctx, policyBinding); err != nil {
		return false, "", fmt.Errorf("failed to create creator PolicyBinding: %w", err)
	}

	return false, "Waiting for PolicyBinding to become ready", nil
}

// isPolicyBindingReady checks if a PolicyBinding is ready
func isPolicyBindingReady(binding *iamv1alpha1.PolicyBinding) (bool, string, error) {
	for _, condition := range binding.Status.Conditions {
		if condition.Type == "Ready" {
			if condition.Status == metav1.ConditionTrue {
				return true, "", nil
			}
			return false, fmt.Sprintf("PolicyBinding not ready: %s", condition.Message), nil
		}
	}
	return false, "Waiting for PolicyBinding to be reconciled", nil
}

// cleanupPolicyBindings deletes PolicyBindings associated with a Note
func cleanupPolicyBindings(
	ctx context.Context,
	c client.Client,
	noteResource NoteResource,
	creatorEditorRoleNamespace string,
) error {
	log := log.FromContext(ctx)

	// List PolicyBindings with labels matching this Note
	labelSelector := client.MatchingLabels{
		noteManagedByLabel: noteManagedByValue,
		noteNameLabel:      noteResource.GetName(),
		noteUIDLabel:       string(noteResource.GetUID()),
	}

	var policyBindings iamv1alpha1.PolicyBindingList
	if err := c.List(ctx, &policyBindings, client.InNamespace(creatorEditorRoleNamespace), labelSelector); err != nil {
		return fmt.Errorf("failed to list PolicyBindings: %w", err)
	}

	// Delete each PolicyBinding
	for i := range policyBindings.Items {
		pb := &policyBindings.Items[i]
		log.Info("Deleting PolicyBinding", "name", pb.Name, "namespace", pb.Namespace)
		if err := c.Delete(ctx, pb); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed to delete PolicyBinding %s: %w", pb.Name, err)
		}
	}

	return nil
}

// containsString checks if a string is present in a slice
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// removeString removes a string from a slice
func removeString(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}
