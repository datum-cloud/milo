// Package informer provides dynamic informer management for custom resources.
//
// The informer package allows controllers to watch custom resources dynamically,
// adding and removing watches at runtime without restarting the application.
// It handles resource lifecycle, reference counting, and distributes events to
// multiple consumers interested in the same resource type.
//
// # Key Features
//
//   - Dynamic watch management: Add/remove watches for any GroupVersionKind at runtime
//   - Reference counting: Automatically start/stop informers based on consumer demand
//   - Event distribution: Deliver events to all registered consumers for a resource type
//   - Integration: Implements controller-runtime's manager.Runnable interface
//
// # Concurrency
//
// The manager is safe for concurrent use. Multiple goroutines can safely
// call AddWatch and RemoveWatch simultaneously.
package informer

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Manager provides dynamic informer management for custom resources.
//
// Manager implements controller-runtime's manager.Runnable interface, allowing
// it to be registered with a controller manager and automatically started/stopped
// with the manager's lifecycle.
//
// The Manager handles reference counting of watches - multiple consumers can watch
// the same resource type, and the underlying informer is only stopped when all
// consumers have removed their watches.
type Manager interface {
	// AddWatch registers a new watch for the specified resource type.
	//
	// The manager must be started before calling AddWatch. If successful, events
	// for the specified GVK will be delivered to the provided handler.
	//
	// If multiple consumers watch the same GVK, they will all receive events
	// through their respective handlers.
	//
	// Returns an error if the manager is not started or if the informer cannot
	// be created (e.g., if the CRD doesn't exist).
	AddWatch(ctx context.Context, req WatchRequest) error

	// RemoveWatch unregisters a watch for the specified resource type and consumer.
	//
	// If this is the last consumer watching this GVK, the underlying informer
	// will be stopped and cleaned up. Other consumers watching the same GVK
	// are unaffected.
	//
	// Returns an error if the manager is not started. It is safe to call
	// RemoveWatch for a watch that doesn't exist (no-op).
	RemoveWatch(ctx context.Context, gvk schema.GroupVersionKind, consumerID string) error

	// Start initializes the manager and begins informer management.
	//
	// This method is called automatically by controller-runtime when the
	// controller manager starts. It should not be called directly.
	//
	// Start blocks until the provided context is cancelled, at which point
	// it stops all managed informers and returns.
	//
	// This implements the manager.Runnable interface.
	Start(ctx context.Context) error

	// Stop shuts down the manager and all managed informers.
	//
	// This is called automatically when Start's context is cancelled.
	// It is safe to call Stop multiple times.
	Stop() error

	// NeedLeaderElection returns true, indicating this component should only
	// run on the leader when leader election is enabled.
	//
	// This implements the manager.LeaderElectionRunnable interface.
	NeedLeaderElection() bool
}

// WatchRequest represents a request to watch a specific resource type.
//
// Each consumer must provide a unique ConsumerID to enable proper reference
// counting and cleanup. Multiple WatchRequests with the same GVK but different
// ConsumerIDs will share the underlying informer but receive events independently.
type WatchRequest struct {
	// GVK is the GroupVersionKind of the resource to watch.
	// The corresponding CRD must already be installed in the cluster.
	GVK schema.GroupVersionKind

	// ConsumerID is a unique identifier for the consumer requesting the watch.
	// This is used for reference counting and cleanup. Use a descriptive ID
	// like "my-controller-name" to aid debugging.
	ConsumerID string

	// Handler receives events for resources of this type.
	// The handler methods are called from informer worker goroutines and
	// should not block for extended periods.
	Handler ResourceEventHandler
}

// ResourceEventHandler receives events for watched resources.
//
// Implementations should be careful not to block for extended periods in
// these methods, as they are called from informer worker goroutines.
// For expensive operations, consider queuing work to be processed asynchronously.
type ResourceEventHandler interface {
	// OnAdd is called when a resource is created.
	// The object is provided as an unstructured.Unstructured for flexibility.
	OnAdd(obj *unstructured.Unstructured)

	// OnUpdate is called when a resource is updated.
	// Both the old and new versions of the object are provided to enable
	// comparison and delta processing.
	OnUpdate(old, new *unstructured.Unstructured)

	// OnDelete is called when a resource is deleted.
	// The object represents the last known state before deletion.
	OnDelete(obj *unstructured.Unstructured)
}
