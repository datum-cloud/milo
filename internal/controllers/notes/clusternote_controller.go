package notes

import (
	"context"
	"fmt"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	notesv1alpha1 "go.miloapis.com/milo/pkg/apis/notes/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type ClusterNoteController struct {
	Client client.Client

	CreatorEditorRoleName      string
	CreatorEditorRoleNamespace string
}

// +kubebuilder:rbac:groups=notes.miloapis.com,resources=clusternotes,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=notes.miloapis.com,resources=clusternotes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=notes.miloapis.com,resources=clusternotes/finalizers,verbs=update
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users,verbs=get;list;watch
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=policybindings,verbs=get;list;create;delete

func (r *ClusterNoteController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("clusternote-controller").WithValues("clusternote", req.Name)

	clusterNote := &notesv1alpha1.ClusterNote{}
	if err := r.Client.Get(ctx, req.NamespacedName, clusterNote); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling ClusterNote", "clusternote", clusterNote.Name)

	// Handle deletion
	if !clusterNote.DeletionTimestamp.IsZero() {
		if containsString(clusterNote.Finalizers, noteFinalizer) {
			log.Info("ClusterNote is being deleted, cleaning up PolicyBindings", "clusternote", clusterNote.Name)

			// Clean up PolicyBindings
			if err := cleanupPolicyBindings(ctx, r.Client, clusterNote, r.CreatorEditorRoleNamespace); err != nil {
				log.Error(err, "failed to cleanup PolicyBindings")
				return ctrl.Result{}, fmt.Errorf("failed to cleanup PolicyBindings: %w", err)
			}

			// Remove finalizer
			clusterNote.Finalizers = removeString(clusterNote.Finalizers, noteFinalizer)
			if err := r.Client.Update(ctx, clusterNote); err != nil {
				log.Error(err, "failed to remove finalizer")
				return ctrl.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}
		return ctrl.Result{}, nil
	}

	// Add finalizer if not present
	if !containsString(clusterNote.Finalizers, noteFinalizer) {
		clusterNote.Finalizers = append(clusterNote.Finalizers, noteFinalizer)
		if err := r.Client.Update(ctx, clusterNote); err != nil {
			log.Error(err, "failed to add finalizer")
			return ctrl.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
		return ctrl.Result{Requeue: true}, nil
	}

	noteCreator := &iamv1alpha1.User{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: clusterNote.Spec.CreatorRef.Name}, noteCreator); err != nil {
		if apierrors.IsNotFound(err) {
			log.Info("User referenced in CreatorRef not found, status.CreatedBy will not be updated", "user", clusterNote.Spec.CreatorRef.Name)
			return ctrl.Result{}, nil
		}
		log.Error(err, "failed to get User", "user", clusterNote.Spec.CreatorRef.Name)
		return ctrl.Result{}, fmt.Errorf("failed to get User: %w", err)
	}

	policyBindingReady, policyBindingMessage, err := ensureCreatorEditorPolicyBinding(ctx, r.Client, r.Client.Scheme(), clusterNote, noteCreator, r.CreatorEditorRoleName, r.CreatorEditorRoleNamespace)
	if err != nil {
		log.Error(err, "failed to ensure creator PolicyBinding")
		return ctrl.Result{}, fmt.Errorf("failed to ensure creator PolicyBinding: %w", err)
	}

	oldNoteStatus := clusterNote.Status.DeepCopy()

	clusterNote.Status.CreatedBy = noteCreator.Spec.Email

	if policyBindingReady {
		meta.SetStatusCondition(&clusterNote.Status.Conditions, metav1.Condition{
			Type:               noteReadyConditionType,
			Status:             metav1.ConditionTrue,
			Reason:             noteReadyConditionReason,
			Message:            "Reconciled successfully",
			LastTransitionTime: metav1.Now(),
		})
	} else {
		meta.SetStatusCondition(&clusterNote.Status.Conditions, metav1.Condition{
			Type:               noteReadyConditionType,
			Status:             metav1.ConditionFalse,
			Reason:             "PolicyBindingNotReady",
			Message:            policyBindingMessage,
			LastTransitionTime: metav1.Now(),
		})
	}

	if !equality.Semantic.DeepEqual(oldNoteStatus, &clusterNote.Status) {
		log.Info("Updating ClusterNote status")
		if err := r.Client.Status().Update(ctx, clusterNote); err != nil {
			log.Error(err, "Failed to update ClusterNote status")
			return ctrl.Result{}, fmt.Errorf("failed to update ClusterNote status: %w", err)
		}
	} else {
		log.Info("ClusterNote status unchanged, skipping update")
	}

	if !policyBindingReady {
		log.Info("PolicyBinding not ready, will retry", "message", policyBindingMessage)
		return ctrl.Result{}, fmt.Errorf("waiting for PolicyBinding to become ready: %s", policyBindingMessage)
	}

	return ctrl.Result{}, nil
}

func (r *ClusterNoteController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&notesv1alpha1.ClusterNote{}).
		Named("clusternote").
		Complete(r)
}
