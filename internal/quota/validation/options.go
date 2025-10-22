package validation

// ValidationOptions configures validation behavior.
type ValidationOptions struct {
	// DryRun, when true, skips validations that query API server state.
	// Set DryRun to true when validating resources during dry-run operations,
	// such as GitOps workflows where multiple resources are applied together
	// and may not exist yet in the API server.
	//
	// When DryRun is true, validators perform:
	// - Syntax validation (CEL expressions, template syntax)
	// - Structural validation (required fields, mutual exclusivity)
	// - Static schema validation
	//
	// When DryRun is true, validators skip:
	// - Resource type existence checks against ResourceRegistrations
	// - Cross-resource reference validation
	// - Any validation requiring API server queries
	//
	// Use DryRun=true for:
	// - Flux/ArgoCD server-side apply with --dry-run
	// - kubectl apply --dry-run=server
	// - Admission webhooks handling dry-run requests
	DryRun bool
}

// DefaultValidationOptions returns options for full validation.
// Controllers use DefaultValidationOptions to perform complete validation,
// including API state checks.
func DefaultValidationOptions() ValidationOptions {
	return ValidationOptions{
		DryRun: false,
	}
}

// DryRunValidationOptions returns options for dry-run validation.
// Admission webhooks use DryRunValidationOptions when handling dry-run requests.
func DryRunValidationOptions() ValidationOptions {
	return ValidationOptions{
		DryRun: true,
	}
}
