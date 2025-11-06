package iam

import (
	"context"
	"fmt"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/finalizer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	groupFinalizerKey = "iam.miloapis.com/group"
)

// GroupReconciler reconciles a Group object
type GroupController struct {
	Client     client.Client
	Finalizers finalizer.Finalizers
}

// groupFinalizer implements the finalizer.Finalizer interface for Group cleanup.
// This is used to clean up associated GroupMemberships and PolicyBindings when a Group is deleted.
type groupFinalizer struct {
	K8sClient client.Client
}

// UpdateResourcesParams holds the parameters for updating/deleting associated resources
type UpdateResourcesParams struct {
	Group        *iamv1alpha1.Group
	List         client.ObjectList
	MatchField   string
	ResourceType string
}

// PolicyBindingUpdate contains a PolicyBinding and its updated subjects
type PolicyBindingUpdate struct {
	PolicyBinding *iamv1alpha1.PolicyBinding
	Subjects      []iamv1alpha1.Subject
}

// deleteGroupMemberships deletes all GroupMembership objects associated with the given group
func (f *groupFinalizer) deleteGroupMemberships(ctx context.Context, group *iamv1alpha1.Group) error {
	log := logf.FromContext(ctx)
	log.Info("Deleting GroupMemberships associated with group", "groupName", group.Name)

	var gmList iamv1alpha1.GroupMembershipList
	if err := f.K8sClient.List(ctx, &gmList, client.InNamespace(group.Namespace)); err != nil {
		log.Error(err, "Failed to list GroupMemberships", "groupName", group.Name)
		return err
	}

	for _, gm := range gmList.Items {
		if gm.Spec.GroupRef.Name != group.Name {
			continue
		}

		if err := f.K8sClient.Delete(ctx, &gm); err != nil {
			if !errors.IsNotFound(err) {
				log.Error(err, "Failed to delete GroupMembership", "name", gm.Name)
				return err
			}
		} else {
			log.Info("Deleted GroupMembership associated with group", "groupName", group.Name, "groupMembershipName", gm.Name)
		}
	}

	log.Info("Successfully deleted GroupMemberships associated with group", "groupName", group.Name)
	return nil
}

// updatePolicyBindings updates/deletes all PolicyBinding objects associated with the given group.
// If a PolicyBinding has no subjects left, it will be deleted.
// Otherwise, the PolicyBinding will be updated with the remaining subjects.
func (f *groupFinalizer) updatePolicyBindings(ctx context.Context, group *iamv1alpha1.Group) error {
	log := logf.FromContext(ctx)
	log.Info("Updating PolicyBindings associated with group", "groupName", group.Name)

	var pbList iamv1alpha1.PolicyBindingList
	if err := f.K8sClient.List(ctx, &pbList, client.InNamespace(group.Namespace)); err != nil {
		log.Error(err, "Failed to list PolicyBindings", "groupName", group.Name)
		return err
	}

	// Update/delete all PolicyBindings that reference this group
	for _, pb := range pbList.Items {
		var updatedSubjects []iamv1alpha1.Subject
		hasGroupRef := false

		// Keep only subjects that don't reference this group
		for _, subject := range pb.Spec.Subjects {
			if subject.Kind == "Group" && subject.Name == group.Name {
				hasGroupRef = true
				continue
			}
			updatedSubjects = append(updatedSubjects, subject)
		}

		if !hasGroupRef {
			continue
		}

		// If there are no subjects left, delete the PolicyBinding
		if len(updatedSubjects) == 0 {
			if err := f.K8sClient.Delete(ctx, &pb); err != nil {
				if !errors.IsNotFound(err) {
					log.Error(err, "Failed to delete PolicyBinding", "name", pb.Name)
					return err
				}
			} else {
				log.Info("Deleted PolicyBinding associated with group", "groupName", group.Name, "policyBindingName", pb.Name)
			}
			continue
		}

		// Otherwise update the PolicyBinding with the remaining subjects
		pbCopy := pb.DeepCopy()
		pbCopy.Spec.Subjects = updatedSubjects
		if err := f.K8sClient.Update(ctx, pbCopy); err != nil {
			log.Error(err, "Failed to update PolicyBinding", "name", pbCopy.Name)
			return err
		}
		log.Info("Updated PolicyBinding associated with group", "groupName", group.Name, "policyBindingName", pbCopy.Name)
	}

	log.Info("Successfully updated PolicyBindings associated with group", "groupName", group.Name)
	return nil
}

func (f *groupFinalizer) Finalize(ctx context.Context, obj client.Object) (finalizer.Result, error) {
	log := logf.FromContext(ctx)

	// Type assertion to check if the object is a Group
	group, ok := obj.(*iamv1alpha1.Group)
	if !ok {
		log.Error(fmt.Errorf("unexpected object type %T, expected Group", obj), "Failed to finalize Group")
		return finalizer.Result{}, fmt.Errorf("unexpected object type %T, expected Group", obj)
	}
	log.Info("Finalizing Group", "groupName", obj.GetName())

	// Update associated PolicyBindings
	log.Info("Deleting PolicyBindings from Group", "groupName", group.Name)
	if err := f.updatePolicyBindings(ctx, group); err != nil {
		return finalizer.Result{}, err
	}

	// Delete associated GroupMemberships
	log.Info("Deleting GroupMemberships from Group", "groupName", group.Name)
	if err := f.deleteGroupMemberships(ctx, group); err != nil {
		return finalizer.Result{}, err
	}

	log.Info("Successfully finalized Group (clenaed up PolicyBindings and GroupMemberships)")
	return finalizer.Result{}, nil
}

// +kubebuilder:rbac:groups=iam.miloapis.com,resources=groups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=groups/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=groups/finalizers,verbs=update
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=groupmemberships,verbs=list;delete
// +kubebuilder:rbac:groups=iam.miloapis.com,resources=policybindings,verbs=list;update;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Group object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/reconcile
func (r *GroupController) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Get the Group resource
	group := &iamv1alpha1.Group{}
	err := r.Client.Get(ctx, req.NamespacedName, group)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("Group resource not found. Ignoring since object must be deleted")
			return ctrl.Result{}, nil
		}
		log.Error(err, "Failed to get Group")
		return ctrl.Result{}, err
	}

	log.Info("Reconciling Group", "groupName", group.Name)

	// Run finalizers
	finalizeResult, err := r.Finalizers.Finalize(ctx, group)
	if err != nil {
		log.Error(err, "Failed to run finalizers for Group")
		return ctrl.Result{}, fmt.Errorf("failed to run finalizers for Group: %w", err)
	}

	if finalizeResult.Updated {
		log.Info("finalizer updated the group object, updating API server")
		if updateErr := r.Client.Update(ctx, group); updateErr != nil {
			log.Error(updateErr, "Failed to update Group after finalizer update")
			return ctrl.Result{}, updateErr
		}
		return ctrl.Result{}, nil
	}

	if group.GetDeletionTimestamp() != nil {
		log.Info("Group is marked for deletion, stopping reconciliation")
		return ctrl.Result{}, nil
	}

	// Update status if needed
	originalStatus := group.Status.DeepCopy()

	groupCondition := metav1.Condition{
		Type:    "Ready",
		Status:  metav1.ConditionTrue,
		Reason:  "Reconciled",
		Message: "Group successfully reconciled",
	}
	meta.SetStatusCondition(&group.Status.Conditions, groupCondition)

	// Only update if status actually changed
	if !equality.Semantic.DeepEqual(&group.Status, originalStatus) {
		if err := r.Client.Status().Update(ctx, group); err != nil {
			log.Error(err, "Failed to update Group status")
			return ctrl.Result{}, err
		}
		log.Info("Group status updated")
	}

	// Return success
	log.Info("Successfully reconciled Group", "groupName", group.Name)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *GroupController) SetupWithManager(mgr ctrl.Manager) error {
	r.Finalizers = finalizer.NewFinalizers()
	if err := r.Finalizers.Register(groupFinalizerKey, &groupFinalizer{
		K8sClient: r.Client,
	}); err != nil {
		return fmt.Errorf("failed to register group finalizer: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&iamv1alpha1.Group{}).
		Named("group").
		Complete(r)
}
