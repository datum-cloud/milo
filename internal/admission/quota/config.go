package quota

import (
	"time"
)

// WatchManagerConfig holds configuration for the ClaimWatchManager
type WatchManagerConfig struct {
	// DefaultTimeout is the default timeout for waiting for ResourceClaim results
	DefaultTimeout time.Duration

	// WatchReconnectDelay is the delay between watch reconnection attempts
	WatchReconnectDelay time.Duration

	// WatchRestartDelay is the delay between watch restarts after unexpected closure
	WatchRestartDelay time.Duration

	// MaxWaiters is the maximum number of concurrent waiters (0 = unlimited)
	MaxWaiters int

	// CleanupInterval is how often to clean up expired waiters
	CleanupInterval time.Duration

	// EnableMetrics enables collection of watch manager metrics
	EnableMetrics bool
}

// DefaultWatchManagerConfig returns the default configuration for the watch manager
func DefaultWatchManagerConfig() *WatchManagerConfig {
	return &WatchManagerConfig{
		DefaultTimeout:      30 * time.Second,
		WatchReconnectDelay: 30 * time.Second,
		WatchRestartDelay:   5 * time.Second,
		MaxWaiters:          0, // unlimited
		CleanupInterval:     1 * time.Minute,
		EnableMetrics:       true,
	}
}

// AdmissionPluginConfig holds configuration for the ClaimCreationPlugin
type AdmissionPluginConfig struct {
	// WatchManager configuration
	WatchManager *WatchManagerConfig

	// DisableSharedWatch forces the plugin to use individual watches (for rollback)
	DisableSharedWatch bool
}

// DefaultAdmissionPluginConfig returns the default configuration for the admission plugin
func DefaultAdmissionPluginConfig() *AdmissionPluginConfig {
	return &AdmissionPluginConfig{
		WatchManager:       DefaultWatchManagerConfig(),
		DisableSharedWatch: false,
	}
}
