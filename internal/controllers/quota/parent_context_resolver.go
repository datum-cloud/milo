package quota

import (
	"context"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ParentContextResolver manages REST configurations for target control planes
// with minimal memory footprint for cross-cluster grant creation.
type ParentContextResolver struct {
	// clientCache maps parent context identifiers to clients
	clientCache map[string]*CachedClient
	// mu protects concurrent access to the cache
	mu sync.RWMutex
	// defaultClient is used when no parent context is specified
	defaultClient client.Client
}

// CachedClient represents a cached client with TTL.
type CachedClient struct {
	client    client.Client
	createdAt time.Time
	ttl       time.Duration
}

// NewParentContextResolver creates a new ParentContextResolver.
func NewParentContextResolver(defaultClient client.Client) *ParentContextResolver {
	return &ParentContextResolver{
		clientCache:   make(map[string]*CachedClient),
		defaultClient: defaultClient,
	}
}

// ResolveClient resolves a client for the given parent context.
// For now, this is a simplified implementation that returns the default client.
// In a full implementation, this would integrate with Milo's ProjectProvider system.
func (r *ParentContextResolver) ResolveClient(
	ctx context.Context,
	parentContext *ParentContextSpec,
	triggerObj *unstructured.Unstructured,
) (client.Client, error) {
	logger := log.FromContext(ctx).WithValues(
		"parentContextKind", parentContext.Kind,
		"parentContextName", parentContext.NameExpression,
	)

	// For now, we'll return the default client since cross-cluster support
	// requires integration with Milo's ProjectProvider system which is complex.
	// TODO: Implement full cross-cluster support with ProjectProvider integration.

	logger.V(1).Info("Using default client for parent context resolution (cross-cluster support not yet implemented)")

	return r.defaultClient, nil
}

// ResolveClientWithCaching resolves a client with caching support.
// This is a placeholder for future cross-cluster implementation.
func (r *ParentContextResolver) ResolveClientWithCaching(
	ctx context.Context,
	parentContext *ParentContextSpec,
	triggerObj *unstructured.Unstructured,
) (client.Client, error) {
	// Create cache key
	cacheKey := fmt.Sprintf("%s/%s/%s",
		parentContext.APIGroup,
		parentContext.Kind,
		parentContext.NameExpression,
	)

	r.mu.RLock()
	cachedClient, exists := r.clientCache[cacheKey]
	r.mu.RUnlock()

	// Check if cached client is still valid
	if exists && time.Since(cachedClient.createdAt) < cachedClient.ttl {
		return cachedClient.client, nil
	}

	// Need to create or refresh the client
	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check pattern - another goroutine might have updated the cache
	if cachedClient, exists := r.clientCache[cacheKey]; exists &&
		time.Since(cachedClient.createdAt) < cachedClient.ttl {
		return cachedClient.client, nil
	}

	// Create new client (placeholder implementation)
	newClient, err := r.createClientForContext(ctx, parentContext, triggerObj)
	if err != nil {
		return nil, fmt.Errorf("failed to create client for parent context: %w", err)
	}

	// Cache the client with 5-minute TTL
	r.clientCache[cacheKey] = &CachedClient{
		client:    newClient,
		createdAt: time.Now(),
		ttl:       5 * time.Minute,
	}

	return newClient, nil
}

// createClientForContext creates a new client for the given parent context.
// This is a placeholder implementation.
func (r *ParentContextResolver) createClientForContext(
	ctx context.Context,
	parentContext *ParentContextSpec,
	triggerObj *unstructured.Unstructured,
) (client.Client, error) {
	logger := log.FromContext(ctx)

	// TODO: Implement actual client creation based on parent context
	// This would involve:
	// 1. Looking up the parent context resource (e.g., Project)
	// 2. Extracting connection information (kubeconfig, endpoint, etc.)
	// 3. Creating a new client with appropriate authentication
	// 4. Integrating with Milo's ProjectProvider system

	logger.V(1).Info("Creating client for parent context (placeholder implementation)")

	// For now, return the default client
	return r.defaultClient, nil
}

// CleanupExpiredClients removes expired clients from the cache.
func (r *ParentContextResolver) CleanupExpiredClients() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for key, cachedClient := range r.clientCache {
		if now.Sub(cachedClient.createdAt) >= cachedClient.ttl {
			delete(r.clientCache, key)
		}
	}
}

// StartCleanupTask starts a background task to clean up expired clients.
func (r *ParentContextResolver) StartCleanupTask(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.CleanupExpiredClients()
			}
		}
	}()
}

// GetCacheStats returns statistics about the client cache.
func (r *ParentContextResolver) GetCacheStats() (int, int) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	total := len(r.clientCache)
	expired := 0
	now := time.Now()

	for _, cachedClient := range r.clientCache {
		if now.Sub(cachedClient.createdAt) >= cachedClient.ttl {
			expired++
		}
	}

	return total, expired
}
