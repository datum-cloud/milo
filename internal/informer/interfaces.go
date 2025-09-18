// Package informer provides dynamic informer management for CRDs.
//
// This package allows controllers to dynamically add and remove watches for resource types
// without needing to restart informer factories. It automatically handles CRD lifecycle,
// reference counting, and event distribution to multiple consumers.
package informer

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Manager provides dynamic informer management for CRDs.
type Manager interface {
	// AddWatch adds a requirement for watching a resource type.
	// If the CRD is already available, the informer will be created immediately.
	// If not, it will be created when the CRD becomes available.
	AddWatch(ctx context.Context, req WatchRequest) error

	// RemoveWatch removes a requirement for watching a resource type.
	// If no other consumers need this resource type, the informer will be stopped.
	RemoveWatch(ctx context.Context, gvk schema.GroupVersionKind, consumerID string) error

	// Start begins CRD watching and informer management.
	// This should be called once during application startup.
	Start(ctx context.Context) error

	// Stop shuts down the manager and all managed informers.
	Stop() error

	// GetStatus returns the current status of the manager.
	GetStatus() ManagerStatus

	// NeedLeaderElection indicates this component should run under leader election.
	NeedLeaderElection() bool
}

// WatchRequest represents a request to watch a specific resource type.
type WatchRequest struct {
	// GVK is the GroupVersionKind of the resource to watch.
	GVK schema.GroupVersionKind

	// ConsumerID is a unique identifier for the consumer requesting the watch.
	// This is used for reference counting and cleanup.
	ConsumerID string

	// Handler receives events for resources of this type.
	Handler ResourceEventHandler

	// Options contains optional configuration for the watch.
	Options WatchOptions
}

// ResourceEventHandler receives events for watched resources.
type ResourceEventHandler interface {
	// OnAdd is called when a resource is created.
	OnAdd(obj *unstructured.Unstructured)

	// OnUpdate is called when a resource is updated.
	OnUpdate(old, new *unstructured.Unstructured)

	// OnDelete is called when a resource is deleted.
	OnDelete(obj *unstructured.Unstructured)
}

// WatchOptions contains optional configuration for a watch.
type WatchOptions struct {
	// Namespace restricts the watch to a specific namespace.
	// If empty, all namespaces are watched.
	Namespace string

	// LabelSelector restricts the watch to resources matching the label selector.
	LabelSelector string

	// FieldSelector restricts the watch to resources matching the field selector.
	FieldSelector string

	// ResyncPeriod is the period for forced resync.
	// If zero, a default period is used.
	ResyncPeriod time.Duration
}

// ManagerStatus provides information about the current state of the manager.
type ManagerStatus struct {
	// Started indicates if the manager has been started.
	Started bool

	// ActiveInformers maps GVKs to their informer status.
	ActiveInformers map[schema.GroupVersionKind]InformerStatus

	// PendingCRDs lists GVKs that are being watched but their CRDs are not yet available.
	PendingCRDs []schema.GroupVersionKind

	// Consumers maps consumer IDs to the GVKs they are watching.
	Consumers map[string][]schema.GroupVersionKind
}

// InformerStatus provides information about a specific informer.
type InformerStatus struct {
	// GVK is the GroupVersionKind being watched.
	GVK schema.GroupVersionKind

	// Synced indicates if the informer has completed its initial sync.
	Synced bool

	// LastSync is the timestamp of the last successful sync.
	LastSync time.Time

	// Consumers lists the consumer IDs using this informer.
	Consumers []string

	// EventCount is the total number of events processed by this informer.
	EventCount int64

	// CRDAvailable indicates if the CRD for this resource type is available.
	CRDAvailable bool
}

// ResourceEventHandlerFunc provides a convenient way to create ResourceEventHandlers from functions.
type ResourceEventHandlerFunc struct {
	AddFunc    func(obj *unstructured.Unstructured)
	UpdateFunc func(old, new *unstructured.Unstructured)
	DeleteFunc func(obj *unstructured.Unstructured)
}

// OnAdd implements ResourceEventHandler.
func (f *ResourceEventHandlerFunc) OnAdd(obj *unstructured.Unstructured) {
	if f.AddFunc != nil {
		f.AddFunc(obj)
	}
}

// OnUpdate implements ResourceEventHandler.
func (f *ResourceEventHandlerFunc) OnUpdate(old, new *unstructured.Unstructured) {
	if f.UpdateFunc != nil {
		f.UpdateFunc(old, new)
	}
}

// OnDelete implements ResourceEventHandler.
func (f *ResourceEventHandlerFunc) OnDelete(obj *unstructured.Unstructured) {
	if f.DeleteFunc != nil {
		f.DeleteFunc(obj)
	}
}
