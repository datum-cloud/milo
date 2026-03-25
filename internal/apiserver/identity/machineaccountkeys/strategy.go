package machineaccountkeys

import (
	"context"
	"time"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage/names"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	identityv1alpha1 "go.miloapis.com/milo/pkg/apis/identity/v1alpha1"
)

// machineAccountKeyStrategy implements rest.RESTCreateStrategy for MachineAccountKey.
type machineAccountKeyStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

// Strategy is the singleton strategy instance used by the REST handler's store.
// It uses legacyscheme.Scheme as the ObjectTyper so the store can identify objects
// by their registered GVK. The identity scheme must be installed into legacyscheme.Scheme
// (via identityapi.Install in config.go) before this is used.
var Strategy = machineAccountKeyStrategy{legacyscheme.Scheme, names.SimpleNameGenerator}

var _ rest.RESTCreateStrategy = machineAccountKeyStrategy{}
var _ rest.RESTUpdateStrategy = machineAccountKeyStrategy{}
var _ rest.RESTDeleteStrategy = machineAccountKeyStrategy{}

func (machineAccountKeyStrategy) NamespaceScoped() bool { return true }

// PrepareForCreate clears the PrivateKey field before the object is written to
// etcd. This is the primary defense-in-depth mechanism ensuring the private key
// is never persisted, even if the REST handler incorrectly sets it before calling
// the underlying store.
func (machineAccountKeyStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	key, ok := obj.(*identityv1alpha1.MachineAccountKey)
	if !ok {
		return
	}

	// Never persist the private key to etcd.
	key.Status.PrivateKey = ""

	// FR8: initialize Ready=Unknown if no conditions are set
	if len(key.Status.Conditions) == 0 {
		key.Status.Conditions = []metav1.Condition{
			{
				Type:               "Ready",
				Status:             metav1.ConditionUnknown,
				Reason:             "Unknown",
				Message:            "Waiting for control plane to reconcile",
				LastTransitionTime: metav1.NewTime(time.Unix(0, 0).UTC()),
			},
		}
	}
}

// PrepareForUpdate clears the PrivateKey field before the object is written to
// etcd (same as PrepareForCreate) to prevent accidental persistence.
func (machineAccountKeyStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	key, ok := obj.(*identityv1alpha1.MachineAccountKey)
	if !ok {
		return
	}

	// Never persist the private key to etcd (defense in depth).
	key.Status.PrivateKey = ""
}

// Validate enforces field-level constraints on MachineAccountKey before persistence.
func (machineAccountKeyStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	key, ok := obj.(*identityv1alpha1.MachineAccountKey)
	if !ok {
		return field.ErrorList{field.InternalError(field.NewPath(""), nil)}
	}

	var errs field.ErrorList
	specPath := field.NewPath("spec")

	// FR5: machineAccountName must be non-empty
	if key.Spec.MachineAccountName == "" {
		errs = append(errs, field.Required(specPath.Child("machineAccountName"), "machineAccountName is required"))
	}

	// FR6: expiration date must be in the future if provided
	if key.Spec.ExpirationDate != nil && !key.Spec.ExpirationDate.Time.IsZero() {
		if !key.Spec.ExpirationDate.Time.After(time.Now()) {
			errs = append(errs, field.Invalid(
				specPath.Child("expirationDate"),
				key.Spec.ExpirationDate,
				"expirationDate must be in the future",
			))
		}
	}

	// FR7: public key must be a valid PEM-encoded RSA public key if provided
	if key.Spec.PublicKey != "" {
		if err := validateRSAPublicKey(key.Spec.PublicKey); err != nil {
			errs = append(errs, field.Invalid(
				specPath.Child("publicKey"),
				"<redacted>",
				err.Error(),
			))
		}
	}

	return errs
}

// ValidateUpdate enforces immutability constraints and validates updates to MachineAccountKey.
// It blocks any updates to the Spec after creation.
func (machineAccountKeyStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	key, ok := obj.(*identityv1alpha1.MachineAccountKey)
	if !ok {
		return field.ErrorList{field.InternalError(field.NewPath(""), nil)}
	}

	oldKey, ok := old.(*identityv1alpha1.MachineAccountKey)
	if !ok {
		return field.ErrorList{field.InternalError(field.NewPath(""), nil)}
	}

	var errs field.ErrorList

	// Block all updates to Spec after creation
	if !apiequality.Semantic.DeepEqual(key.Spec, oldKey.Spec) {
		errs = append(errs, field.Forbidden(
			field.NewPath("spec"),
			"spec is immutable after creation",
		))
	}

	return errs
}

// Canonicalize normalizes the object. No-op here.
func (machineAccountKeyStrategy) Canonicalize(obj runtime.Object) {}

// AllowCreateOnUpdate returns false — callers must use POST to create.
func (machineAccountKeyStrategy) AllowCreateOnUpdate() bool { return false }

// AllowUnconditionalUpdate returns false — updates require a resourceVersion precondition.
func (machineAccountKeyStrategy) AllowUnconditionalUpdate() bool { return false }

// WarningsOnCreate returns no warnings for MachineAccountKey creation.
func (machineAccountKeyStrategy) WarningsOnCreate(_ context.Context, _ runtime.Object) []string {
	return nil
}

// WarningsOnUpdate returns no warnings for MachineAccountKey updates.
func (machineAccountKeyStrategy) WarningsOnUpdate(_ context.Context, _, _ runtime.Object) []string {
	return nil
}
