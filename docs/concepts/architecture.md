# Platform Architecture

Milo is a comprehensive business operating system that provides service
providers with declarative, API-driven management of their complete business

## Platform Layers

### Control Plane

The control plane provides the foundational API-driven interface for managing
organizations, projects, users, and services registered with the platform. Built
on a reliable control plane architecture, it offers declarative configuration,
reliable control loops, API extensibility with community-supported tooling, and
automated reconciliation across both built-in platform services and custom
services that service providers register.

This layer handles:

- Resource lifecycle management for platform entities and registered services
- Service registration and discovery for custom services
- Multi-tenant resource isolation and access control
- Event generation for downstream platform layers
- Policy enforcement and validation

For detailed implementation details, see the [Control Plane
Architecture](../developer-guides/architecture/control-plane/) documentation.

### Identity

The identity layer provides comprehensive identity and access management through
declarative APIs that integrate with external authentication and authorization
providers. This layer manages the complete lifecycle of users, groups, roles,
and permissions while delegating actual authentication and authorization
implementation to specialized external services.

Current capabilities:

- User lifecycle management with cluster-scoped human accounts
- Group management and membership for efficient access control
- Role-based access control with granular `{service}/{resource}.{action}`
  permissions
- Policy binding system linking users and groups to specific roles and resources
- Machine account management for service-to-service authentication
- User invitation workflows for streamlined onboarding
- Protected resource registration enabling IAM-aware service integration

The platform will evolve to support advanced identity features including
federated authentication and enhanced multi-tenancy capabilities.

For complete API details and integration patterns, see the [Identity
Architecture](../developer-guides/architecture/identity/) documentation.

### Integration Layer

The integration layer enables external services to integrate with Milo using
standard Kubernetes extensibility patterns. Integrations are deployed as
independent controller managers that watch Milo resources and coordinate with
external services, leveraging the proven [controller-runtime] framework and
webhook mechanisms that the Kubernetes ecosystem provides.

Integration capabilities:

- Controller-based integrations using [kubebuilder] and [controller-runtime]
  frameworks
- Validating and mutating webhooks for real-time API request processing
- Resource watching through the Watch API for event-driven responses
- Event subscription through the Events API for control plane notifications
- Custom APIs and services through Custom Resource Definitions and apiserver
  aggregation

See the [Integration
Architecture](../developer-guides/integrations) guide for
patterns and examples of building controller-based integrations.

## Getting Started

### For Business Stakeholders

- **[Business Value Overview](../getting-started/)** - Understanding Milo's
  value proposition

### For Technical Teams

- **[System Architecture](../developer-guides/architecture/)** - Deep dives into
  each platform layer
- **[API Reference](../reference/api/)** - Complete API documentation
- **[Integration Guides](../developer-guides/integrations/)** - Building custom
  integrations

---

This platform architecture enables service providers to focus on their core
business while Milo handles the complexity of multi-tenant operations,
compliance, billing, and integration with the broader business ecosystem.

[kubebuilder]: https://kubebuilder.io
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime
