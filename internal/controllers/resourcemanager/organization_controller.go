package resourcemanager

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	resourcemanagerv1alpha "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

// OrganizationController reconciles an Organization object
type OrganizationController struct {
	Client client.Client
}

// +kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=organizations,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=namespaces,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=authorization.k8s.io,resources=subjectaccessreviews,verbs=create

func (r *OrganizationController) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	var organization resourcemanagerv1alpha.Organization
	if err := r.Client.Get(ctx, req.NamespacedName, &organization); apierrors.IsNotFound(err) {
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get organization: %w", err)
	}

	// Don't need to continue if the organization is being deleted from the cluster.
	if !organization.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	logger.Info("reconciling organization")
	defer logger.Info("reconcile complete")

	// Find the namespace for this organization
	namespaceName := fmt.Sprintf("organization-%s", organization.Name)
	var namespace corev1.Namespace
	if err := r.Client.Get(ctx, types.NamespacedName{Name: namespaceName}, &namespace); apierrors.IsNotFound(err) {
		// Namespace doesn't exist, nothing to do
		logger.Info("organization namespace not found", "namespace", namespaceName)
		return ctrl.Result{}, nil
	} else if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get organization namespace: %w", err)
	}

	// Check if the organization is already set as the controller owner reference
	hasOwnerRef, err := controllerutil.HasOwnerReference(namespace.OwnerReferences, &organization, r.Client.Scheme())
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to check if organization is owner reference: %w", err)
	} else if hasOwnerRef {
		return ctrl.Result{}, nil
	}

	logger.Info("adding organization as owner reference to namespace", "namespace", namespaceName)

	// Set the organization as the controller owner reference for the namespace
	if err := controllerutil.SetControllerReference(&organization, &namespace, r.Client.Scheme()); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Update the namespace with the owner reference
	if err := r.Client.Update(ctx, &namespace); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update namespace owner references: %w", err)
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *OrganizationController) SetupWithManager(mgr ctrl.Manager) error {
	r.Client = mgr.GetClient()

	return ctrl.NewControllerManagedBy(mgr).
		For(&resourcemanagerv1alpha.Organization{}).
		Named("organization").
		Complete(r)
}
