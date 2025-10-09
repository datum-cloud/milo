package resourcemanager

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	vendorscontrollers "go.miloapis.com/milo/internal/controllers/vendors"
	vendorsv1alpha1 "go.miloapis.com/milo/pkg/apis/vendors/v1alpha1"
)

// VendorValidationReconciler reconciles Vendor objects to validate corporation types
type VendorValidationReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=vendors.miloapis.com,resources=vendors,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vendors.miloapis.com,resources=vendors/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vendors.miloapis.com,resources=vendortypedefinitions,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile validates vendor types against VendorTypeDefinition
func (r *VendorValidationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the Vendor
	var vendor vendorsv1alpha1.Vendor
	if err := r.Get(ctx, req.NamespacedName, &vendor); err != nil {
		logger.Error(err, "unable to fetch Vendor")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Skip validation if vendorType is not set
	if vendor.Spec.VendorType == "" {
		return ctrl.Result{}, nil
	}

	// Get all VendorTypeDefinitions
	var definitionList vendorsv1alpha1.VendorTypeDefinitionList
	if err := r.List(ctx, &definitionList); err != nil {
		logger.Error(err, "unable to list VendorTypeDefinitions")
		return ctrl.Result{}, err
	}

	// Validate the vendor type
	if err := vendorsv1alpha1.ValidateVendorTypeFromList(vendor.Spec.VendorType, definitionList.Items); err != nil {
		logger.Error(err, "invalid vendor type", "vendorType", vendor.Spec.VendorType)

		// Update vendor status with validation error
		// This is a simplified example - in practice you'd want more sophisticated status management
		return ctrl.Result{}, fmt.Errorf("invalid vendor type %q: %w", vendor.Spec.VendorType, err)
	}

	// Validate tax ID secret reference
	if err := vendorscontrollers.ValidateTaxIdSecret(ctx, r.Client, &vendor, vendor.Spec.TaxInfo.TaxIdRef); err != nil {
		logger.Error(err, "invalid tax ID secret reference", "secretName", vendor.Spec.TaxInfo.TaxIdRef.SecretName)
		return ctrl.Result{}, fmt.Errorf("invalid tax ID secret reference: %w", err)
	}

	logger.Info("vendor validated successfully", "vendorType", vendor.Spec.VendorType)
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *VendorValidationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vendorsv1alpha1.Vendor{}).
		Complete(r)
}
