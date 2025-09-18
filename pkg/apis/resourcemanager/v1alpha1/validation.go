package v1alpha1

import (
	"fmt"
	"strings"
)

// ValidateCorporationType validates that a corporation type is valid according to the provided config
func ValidateCorporationType(corpType CorporationType, config *CorporationTypeConfig) error {
	if config == nil || !config.Spec.Active {
		return fmt.Errorf("no active corporation type configuration found")
	}

	corpTypeStr := string(corpType)
	if corpTypeStr == "" {
		return nil // Optional field
	}

	for _, typeDef := range config.Spec.CorporationTypes {
		if typeDef.Code == corpTypeStr {
			if !typeDef.Enabled {
				return fmt.Errorf("corporation type %q is disabled", corpTypeStr)
			}
			return nil
		}
	}

	return fmt.Errorf("invalid corporation type %q, must be one of: %s",
		corpTypeStr,
		strings.Join(getEnabledCorporationTypes(config), ", "))
}

// getEnabledCorporationTypes returns a list of enabled corporation type codes
func getEnabledCorporationTypes(config *CorporationTypeConfig) []string {
	var enabled []string
	for _, typeDef := range config.Spec.CorporationTypes {
		if typeDef.Enabled {
			enabled = append(enabled, typeDef.Code)
		}
	}
	return enabled
}

// GetCorporationTypeDisplayName returns the display name for a corporation type code
func GetCorporationTypeDisplayName(corpType CorporationType, config *CorporationTypeConfig) string {
	if config == nil {
		return string(corpType)
	}

	corpTypeStr := string(corpType)
	for _, typeDef := range config.Spec.CorporationTypes {
		if typeDef.Code == corpTypeStr {
			return typeDef.DisplayName
		}
	}
	return corpTypeStr
}

// GetAvailableCorporationTypes returns all available corporation types from the config
func GetAvailableCorporationTypes(config *CorporationTypeConfig) []CorporationTypeDefinition {
	if config == nil || !config.Spec.Active {
		return nil
	}
	return config.Spec.CorporationTypes
}
