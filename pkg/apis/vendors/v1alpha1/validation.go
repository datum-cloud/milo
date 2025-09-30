package v1alpha1

import (
	"fmt"
	"strings"
)

// ValidateVendorType validates that a vendor type is valid according to the provided definition
func ValidateVendorType(vendorType VendorType, definition *VendorTypeDefinition) error {
	if definition == nil {
		return fmt.Errorf("vendor type definition not found")
	}

	vendorTypeStr := string(vendorType)
	if vendorTypeStr == "" {
		return nil // Optional field
	}

	if definition.Spec.Code != vendorTypeStr {
		return fmt.Errorf("vendor type %q does not match definition code %q", vendorTypeStr, definition.Spec.Code)
	}

	if !definition.Spec.Enabled {
		return fmt.Errorf("vendor type %q is disabled", vendorTypeStr)
	}

	return nil
}

// ValidateVendorTypeFromList validates that a vendor type is valid according to a list of definitions
func ValidateVendorTypeFromList(vendorType VendorType, definitions []VendorTypeDefinition) error {
	vendorTypeStr := string(vendorType)
	if vendorTypeStr == "" {
		return nil // Optional field
	}

	for _, def := range definitions {
		if def.Spec.Code == vendorTypeStr {
			if !def.Spec.Enabled {
				return fmt.Errorf("vendor type %q is disabled", vendorTypeStr)
			}
			return nil
		}
	}

	enabledTypes := getEnabledVendorTypes(definitions)
	return fmt.Errorf("invalid vendor type %q, must be one of: %s",
		vendorTypeStr,
		strings.Join(enabledTypes, ", "))
}

// getEnabledVendorTypes returns a list of enabled vendor type codes
func getEnabledVendorTypes(definitions []VendorTypeDefinition) []string {
	var enabled []string
	for _, def := range definitions {
		if def.Spec.Enabled {
			enabled = append(enabled, def.Spec.Code)
		}
	}
	return enabled
}

// GetVendorTypeDisplayName returns the display name for a vendor type code
func GetVendorTypeDisplayName(vendorType VendorType, definition *VendorTypeDefinition) string {
	if definition == nil {
		return string(vendorType)
	}

	vendorTypeStr := string(vendorType)
	if definition.Spec.Code == vendorTypeStr {
		return definition.Spec.DisplayName
	}
	return vendorTypeStr
}

// GetVendorTypeDisplayNameFromList returns the display name for a vendor type code from a list
func GetVendorTypeDisplayNameFromList(vendorType VendorType, definitions []VendorTypeDefinition) string {
	vendorTypeStr := string(vendorType)
	for _, def := range definitions {
		if def.Spec.Code == vendorTypeStr {
			return def.Spec.DisplayName
		}
	}
	return vendorTypeStr
}

// GetAvailableVendorTypes returns all available vendor type definitions
func GetAvailableVendorTypes(definitions []VendorTypeDefinition) []VendorTypeDefinition {
	var available []VendorTypeDefinition
	for _, def := range definitions {
		if def.Spec.Enabled {
			available = append(available, def)
		}
	}
	return available
}

// FindVendorTypeDefinition finds a vendor type definition by code
func FindVendorTypeDefinition(code string, definitions []VendorTypeDefinition) *VendorTypeDefinition {
	for _, def := range definitions {
		if def.Spec.Code == code {
			return &def
		}
	}
	return nil
}
