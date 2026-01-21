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
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	clusterNoteReadyConditionType   = "Ready"
	clusterNoteReadyConditionReason = "Reconciled"

	clusterNoteManagedByLabel = "notes.miloapis.com/managed-by"
	clusterNoteManagedByValue = "clusternote-controller"
)

type ClusterNoteController struct {
	Client client.Client

	CreatorEditorRoleName      string
	CreatorEditorRoleNamespace string
}

// +kubebuilder:rbac:groups=notes.miloapis.com,resources=clusternotes,verbs=get;list;watch;update;patch;delete
// +kubebuilder:rbac:groups=notes.miloapis.com,resources=clusternotes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=users,verbs=get;list;watch
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=policybindings,verbs=get;create

func (r *ClusterNoteController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithName("clusternote-controller").WithValues("clusternote", req.Name)

	clusterNote := &notesv1alpha1.ClusterNote{}
	if err := r.Client.Get(ctx, req.NamespacedName, clusterNote); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("reconciling ClusterNote", "clusternote", clusterNote.Name)

	if !clusterNote.DeletionTimestamp.IsZero() {
		log.Info("ClusterNote is being deleted, skipping reconciliation", "clusternote", clusterNote.Name)
		return ctrl.Result{}, nil
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

	policyBindingReady, policyBindingMessage, err := r.ensureCreatorEditorPolicyBinding(ctx, clusterNote, noteCreator)
	if err != nil {
		log.Error(err, "failed to ensure creator PolicyBinding")
		return ctrl.Result{}, fmt.Errorf("failed to ensure creator PolicyBinding: %w", err)
	}

	oldNoteStatus := clusterNote.Status.DeepCopy()

	clusterNote.Status.CreatedBy = noteCreator.Spec.Email

	if policyBindingReady {
		meta.SetStatusCondition(&clusterNote.Status.Conditions, metav1.Condition{
			Type:               clusterNoteReadyConditionType,
			Status:             metav1.ConditionTrue,
			Reason:             clusterNoteReadyConditionReason,
			Message:            "Reconciled successfully",
			LastTransitionTime: metav1.Now(),
		})
	} else {
		meta.SetStatusCondition(&clusterNote.Status.Conditions, metav1.Condition{
			Type:               clusterNoteReadyConditionType,
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

func (r *ClusterNoteController) ensureCreatorEditorPolicyBinding(ctx context.Context, clusterNote *notesv1alpha1.ClusterNote, creator *iamv1alpha1.User) (bool, string, error) {
	log := log.FromContext(ctx)

	bindingName := fmt.Sprintf("clusternote-creator-editor-%s", clusterNote.Name)

	var existing iamv1alpha1.PolicyBinding
	if err := r.Client.Get(ctx, types.NamespacedName{Name: bindingName, Namespace: r.CreatorEditorRoleNamespace}, &existing); err == nil {
		return r.isPolicyBindingReady(&existing)
	} else if !apierrors.IsNotFound(err) {
		return false, "", fmt.Errorf("failed to get existing creator PolicyBinding: %w", err)
	}

	policyBinding := &iamv1alpha1.PolicyBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      bindingName,
			Namespace: r.CreatorEditorRoleNamespace,
			Labels: map[string]string{
				clusterNoteManagedByLabel: clusterNoteManagedByValue,
			},
		},
		Spec: iamv1alpha1.PolicyBindingSpec{
			RoleRef: iamv1alpha1.RoleReference{
				Name:      r.CreatorEditorRoleName,
				Namespace: r.CreatorEditorRoleNamespace,
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
					Kind:     "ClusterNote",
					Name:     clusterNote.Name,
					UID:      string(clusterNote.UID),
				},
			},
		},
	}

	if err := controllerutil.SetOwnerReference(clusterNote, policyBinding, r.Client.Scheme()); err != nil {
		return false, "", fmt.Errorf("failed to set owner reference on PolicyBinding: %w", err)
	}

	log.Info("Creating creator PolicyBinding", "policyBinding", bindingName, "user", creator.Name)
	if err := r.Client.Create(ctx, policyBinding); err != nil {
		return false, "", fmt.Errorf("failed to create creator PolicyBinding: %w", err)
	}

	return false, "Waiting for PolicyBinding to become ready", nil
}

func (r *ClusterNoteController) isPolicyBindingReady(binding *iamv1alpha1.PolicyBinding) (bool, string, error) {
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

func (r *ClusterNoteController) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&notesv1alpha1.ClusterNote{}).
		Named("clusternote").
		Complete(r)
}
