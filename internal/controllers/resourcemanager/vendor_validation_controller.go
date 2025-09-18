package resourcemanager

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
)

// VendorValidationReconciler reconciles Vendor objects to validate corporation types
type VendorValidationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=vendors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=vendors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=resourcemanager.miloapis.com,resources=corporationtypeconfigs,verbs=get;list;watch

// Reconcile validates vendor corporation types against active CorporationTypeConfig
func (r *VendorValidationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Vendor
	var vendor resourcemanagerv1alpha1.Vendor
	if err := r.Get(ctx, req.NamespacedName, &vendor); err != nil {
		logger.Error(err, "unable to fetch Vendor")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip validation if corporationType is not set
	if vendor.Spec.CorporationType == "" {
		return ctrl.Result{}, nil
	}

	// Find the active CorporationTypeConfig
	var configList resourcemanagerv1alpha1.CorporationTypeConfigList
	if err := r.List(ctx, &configList); err != nil {
		logger.Error(err, "unable to list CorporationTypeConfigs")
		return ctrl.Result{}, err
	}

	var activeConfig *resourcemanagerv1alpha1.CorporationTypeConfig
	for _, config := range configList.Items {
		if config.Spec.Active {
			activeConfig = &config
			break
		}
	}

	if activeConfig == nil {
		logger.Info("no active CorporationTypeConfig found, skipping validation")
		return ctrl.Result{}, nil
	}

	// Validate the corporation type
	if err := resourcemanagerv1alpha1.ValidateCorporationType(vendor.Spec.CorporationType, activeConfig); err != nil {
		logger.Error(err, "invalid corporation type", "corporationType", vendor.Spec.CorporationType)

		// Update vendor status with validation error
		// This is a simplified example - in practice you'd want more sophisticated status management
		return ctrl.Result{}, fmt.Errorf("invalid corporation type %q: %w", vendor.Spec.CorporationType, err)
	}

	logger.Info("vendor corporation type validated successfully", "corporationType", vendor.Spec.CorporationType)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VendorValidationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&resourcemanagerv1alpha1.Vendor{}).
		Complete(r)
}
