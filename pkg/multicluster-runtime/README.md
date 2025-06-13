# Multi-Cluster Runtime

The `multicluster-runtime` package provides Milo's implementation of the
[kubernetes-sigs/multicluster-runtime][multicluster-runtime] library, enabling
dynamic multi-cluster Kubernetes controller orchestration across Milo's
project-based infrastructure.

## Overview

Multicluster Runtime is an experimental Go library that extends
[controller-runtime][controller-runtime] to enable writing Kubernetes
controllers that can reconcile resources across a dynamic fleet of clusters.
Unlike traditional multi-cluster solutions that require static cluster
configuration, this library provides a clean extension to controller-runtime
without requiring forks or replacements.

This library is designed to be used by Milo's multi-cluster operators to
automatically connect to project control planes and manage resources across
them.

## Design Goals

The multicluster-runtime library addresses several key challenges in modern
Kubernetes environments:

1. **Dynamic Fleet Orchestration**: Automatically discover and manage clusters
   as they come online or go offline
2. **Universal Multi-Cluster Solutions**: Support diverse cluster management
   platforms (kind, cluster-api, Gardener, kcp, BYO clusters)
3. **Seamless Operation**: Work in both single-cluster and multi-cluster modes
   without code changes
4. **Provider Ecosystem**: Extensible architecture supporting external provider
   implementations

## Architecture

### Core Components

- **Providers**: Discover and manage cluster connections dynamically
- **Uniform Reconcilers**: Deploy the same reconciler logic across multiple
  clusters
- **Multi-Cluster-Aware Reconcilers**: Coordinate state across clusters with
  centralized decision-making

## Providers

### Milo Provider

The Milo provider (`pkg/multicluster-runtime/milo/`) implements Milo's
project-based cluster discovery and management. It watches for Project and
ProjectControlPlane resources to dynamically engage with project clusters.

#### Key Features

- **Project-based Discovery**: Automatically discovers clusters through Milo
  Project resources
- **Dynamic Engagement**: Handles cluster lifecycle events (creation, deletion,
  connectivity changes)
- **Service Discovery**: Supports both internal and external service endpoints
- **Graceful Degradation**: Continues operating when individual clusters become
  unavailable

#### Discovery Modes

The Milo provider supports two primary discovery modes:

- **Project Control Plane Mode** - Watches `ProjectControlPlane` resources for
  cluster discovery, enabling direct integration with Milo's infrastructure
  control plane.
- **Project Mode** - Watches `Project` resources directly, providing simplified
  cluster discovery through core Milo APIs.

See the [Milo provider documentation][milo-provider-docs] for detailed
architecture diagrams and guidance for discovery mode selection.

## Related Documentation

- [Multi-Cluster Runtime][multicluster-runtime]
- [Controller Runtime][controller-runtime]

---

[multicluster-runtime]: https://github.com/kubernetes-sigs/multicluster-runtime
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
[milo-provider-docs]: ./milo/README.md
