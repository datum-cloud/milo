package admission

import (
	"time"
)

// WatchManagerConfig holds configuration for the ClaimWatchManager
type WatchManagerConfig struct {
	// DefaultTimeout is the default timeout for waiting for ResourceClaim results
	DefaultTimeout time.Duration

	// InformerResyncPeriod is the resync period for the shared informer
	// This determines how often the informer will resync with the API server
	InformerResyncPeriod time.Duration

	// MaxWaiters is the maximum number of concurrent waiters (0 = unlimited)
	MaxWaiters int
}

// DefaultWatchManagerConfig returns the default configuration for the watch manager
func DefaultWatchManagerConfig() *WatchManagerConfig {
	return &WatchManagerConfig{
		DefaultTimeout:       30 * time.Second,
		InformerResyncPeriod: 30 * time.Second,
		MaxWaiters:           0, // unlimited
	}
}

// AdmissionPluginConfig holds configuration for the ClaimCreationPlugin
type AdmissionPluginConfig struct {
	// WatchManager configuration
	WatchManager *WatchManagerConfig
}

// DefaultAdmissionPluginConfig returns the default configuration for the admission plugin
func DefaultAdmissionPluginConfig() *AdmissionPluginConfig {
	return &AdmissionPluginConfig{
		WatchManager: DefaultWatchManagerConfig(),
	}
}
