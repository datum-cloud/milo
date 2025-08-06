<!-- omit from toc -->
# Control Plane Architecture

- [Architecture Components](#architecture-components)
  - [API Server](#api-server)
  - [Controller Manager](#controller-manager)
  - [Custom Resource Metrics](#custom-resource-metrics)
  - [Vector](#vector)
- [Concepts](#concepts)
  - [Custom Resources](#custom-resources)
  - [Controllers](#controllers)
  - [Admission control through webhooks \& policies](#admission-control-through-webhooks--policies)
    - [Admission Webhooks](#admission-webhooks)
    - [Admission Policies](#admission-policies)
- [Multi-tenant Architecture](#multi-tenant-architecture)
  - [Resource Hierarchy](#resource-hierarchy)
  - [Core Control Plane](#core-control-plane)
  - [Project Control Plane](#project-control-plane)
- [Additional Context](#additional-context)

<!-- omit from toc -->
## Overview

The Milo control plane provides the core capabilities for keeping the state of
the platform in sync. Service providers can register custom resources, build
controllers, and leverage admission controls to adjust the behavior of the
control plane to meet the needs of the business.

Milo's control plane is focused on enabling businesses to customize the behavior
of the control plane through policies instead of code.

> [!IMPORTANT]
>
> ### 👋 Welcome!
>
> We're heavily investing in this foundation. Have a question around how it
> works or how to adapt Milo to meet your needs? Come [hang out on
> slack](https://slack.datum.net)!

## Architecture Components

The Milo control plane is built around several core components that work
together to create a reliable and extensible control plane that service
providers can leverage to offer services to their consumers.

### API Server

The Milo API server is an API server built on the [Kubernetes API server
library][kubernetes-api-server-library]. The API server is responsible for
handling all requests to the control plane, ensuring all requests are properly
validated, authenticated, and authorized. The API server can be extended with to
support the needs of service providers by registering custom resources with the
API, creating controllers to reconcile resources in the control plane and add
business logic, or create webhooks to react to API requests in real-time.

The APIServer also provides API discovery capabilities to support automatically
discovering the API endpoints that are available within the control plane API.

> [!NOTE]
>
> API Discovery is specific to each isolated control plane created for
> consumers. The API resources available within the core control plane that
> manages organizational level resources will differ from resources available in
> the control plane that exists for projects.

Learn more about the [API Server architecture](./api-server.md).

[kubernetes-api-server-library]: https://github.com/kubernetes/apiserver
[controller-runtime]: https://github.com/kubernetes-sigs/controller-runtime

### Controller Manager

The Milo controller manager is responsible for running a series of
[controllers](#controllers) that are responsible for managing the core
functionality of the platform. This component is responsible for reconciling all
core resources exposed by Milo and core control plane features like Garbage
Collection.

### Custom Resource Metrics

A deployment of [kube-state-metrics] is deployed with every control plane
configured to export metrics for any resources registered with the control
plane. The metrics are available via a Prometheus metrics endpoint so it can be
ingested into operational system for internal data or exported to a consumer's
system.

> [!NOTE]
>
> We're considering investing in a push based approach for a kube-state-metrics
> type system that could support multiple control planes.

[kube-state-metrics]: https://github.com/kubernetes/kube-state-metrics

### Vector

[Vector] is used to process and transform any data collected from the control
plane. Vector is configured to transform audit logs so they can be attributed to
the correct control plane and consumer before being sent off to storage. Review
the [vector telemetry
processor](../../../../config/telemetry/vector-audit-log-processor/)
documentation for more information.

[Vector]: https://vector.dev

## Concepts

### Custom Resources

Milo's control plane supports dynamically registering resources through [Custom
Resource Definitions] that can be created in the control plane API. Service
providers can register their resources with automatic support for versioning,
fine-grained authorization, quota control, controllers, and telemetry.

Resources are all defined with a common standard across all APIs to provide a
consistent user experience across all services registered with the control
plane. Service can take advantage of the resource hierarchy to organize the
resources they offer in an organization's control plane or it's projects.

Resources can be registered with the API directly, but it's highly recommended
to use tooling like [Kubebuilder] or other [controller-runtime] based libraries
to create new resources and build controllers / webhooks that extend the control
plane.

[Kubebuilder]: https://www.kubebuilder.io

<!-- TODO: Link off to some documentation about writing new integrations with the control plane -->

[Custom Resource Definitions]: https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/

### Controllers

Controllers are responsible for constantly reconciling the expected state of a
resource with it's desired state. A good example of a controller in a real world
scenario is a thermostat.

When you set the temperature, that's telling the thermostat about your desired
state. The actual room temperature is the current state. The thermostat acts to
bring the current state closer to the desired state, by turning equipment on or
off.

Controllers can be configured to track one or more resources in the control
plane. Typically when a controller starts up, it will reconcile all resources in
the system that its configured to track and then start a watch for new changes.
A controller will be notified of any events to resources it cares about (create,
update, delete, etc.) so it can reconcile the current state of the system with
the resource's desired state.

As the controller makes progress on bringing a resource's desired state closer
to it's expected state, the controller should update the status information of
the resource so it's available to consumers.

<!--
TODO: Update this section in the future to talk about emitting events from
      controllers that can be related to resources so its easy for control plane
      components to communicate more fine-grained events around resources.
-->

Service providers can leverage the [controller-runtime] library to build
controllers as components that can be deployed to a Kubernetes cluster and
connected to the Milo control plane. [Kubebuilder] is a popular framework for
building controllers and can be used to extend the Milo control plane with new
resources, controllers, or webhooks.

[Kubebuilder]: https://www.kubebuilder.io

### Admission control through webhooks & policies

Milo's control plane supports using admission webhooks and admission policies to
modify the behavior of the control plane. These concepts allow service providers
to modify resources or introduce custom validation before a resource is created,
updated, or deleted.

#### Admission Webhooks

Webhooks can be registered with the API server to call the webhook when actions
happen against resources in the control plane. The request sent to the webhook
includes information about the request, including the involved object. Depending
on the type of webhook that's registered, the webhook can modify the object or
reject the request due to validation constraints. Webhooks can also have
side-effects where they create other resources in the control plane when another
resource is created.

Two types of admission webhooks are supported:

- **Mutating webhooks**: These webhooks can modify the request before validation
  is executed against the request. This is useful for adding default values to
  resources or for adding additional metadata to the request.
- **Validating webhooks**: These webhooks are executed right before the request
  is stored in the underlying data store. Validating webhooks are useful for
  ensuring the request is valid and meets the business rules of the service
  provider.

For more information on Admission Webhooks, refer to the [Kubernetes Dynamic
Admission Control][admission-control] documentation.

#### Admission Policies

In addition to the webhooks, Milo exposes [validating admission policies] and
which can be used to add validating logic to the control plane without having to
implement and deploy webhooks. These policies make it easy for service providers
to customize the control plane to meet their needs through configuration.

Refer to the [validating admission policies] documentation for more information.

[admission-control]: https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/
[validating admission policies]: https://kubernetes.io/docs/reference/access-authn-authz/validating-admission-policy/

> [!NOTE]
>
> We also plan to support the [Mutating Admission Policy] type in the future as
> well to support modifying resources through policies instead of requiring
> custom code or deploying webhooks.

<!--
TODO: Replace the Kubernetes links here with Milo documentation links.
-->

[Mutating Admission Policy]: https://kubernetes.io/docs/reference/access-authn-authz/mutating-admission-policy/

## Multi-tenant Architecture

The Milo control plane implements a hierarchical multi-tenant architecture with
two distinct operational control planes that serve different purposes for
service providers.

### Resource Hierarchy

Milo supports a resource hierarchy that enables both organizational management
and service delivery. A service provider can support one or more organizations
to represent it's customers. Service delivery is managed through projects which
are given isolated control planes with just the resources installed for services
enabled in the project.

```
Organization (Tenant Boundary)
├── Identity & Access Management (IAM)
│   ├── Users & Groups
│   ├── Roles & Policies
│   └── Organization Memberships
└── Projects (Service Delivery Units)
    ├── Project-specific Resources
    ├── Service Configurations
    └── Feature Enablement
```

### Core Control Plane

The core control plane is responsible for managing a service provider's
platform. Service providers leverage the core control plane to manage
organizations, users, and projects across the platform. Namespaces exist in the
core control plane to provide isolation between organizations and administrative
resources available to service providers.

The following namespaces are expected to exist in the core control plane.

- **kube-system** - can't be removed because some controller manager components
  depend on this namespace existing
- **default** this namespace is not used
- **kube-public** and **kube-system** is an internal namespace that can't be removed due to
  controller manager expectations
- **milo-system** is the system namespace for managing the Milo platform
- **organization-*** contains all resources related to an organization.
  this namespaces

Consumers can query their organizational resources through the organization
context to ensure their requests are properly authorized. The organization
context information in passed through the authorization context so it can be
leveraged in authorization queries.

<!-- TODO: Create documentation for how to query the API through the organizational context -->

Users also manage their projects in the core control plane. Every project is
provisioned it's own control plane to provide full tenancy isolation.

### Project Control Plane

The project control plane operates at the service delivery level and manages
individual consumer workspaces. Projects are how users consume services from
service providers and how service providers can control how many resources
consumers can use in their projects.

Projects also provide tenancy isolation between other projects and
organizations. Project admins can leverage Kubernetes RBAC policies to manage
individual access to resources within the project.

Services operating in the project can leverage the control plane to integrate
with each other.

> [!IMPORTANT]
>
> Currently, project control plane isolation is done by deploying a separate
> deployment of all control plane components. The API gateway is used to route
> requests to the appropriate control plane based on the URL path.
>
> We plan to invest in the control plane components in the future to virtualize
> project control planes so the core control plane and all project control
> planes could be served out of a single API server deployment.
>
> This will simplify the operations of the platform and reduce the compute
> overhead of each project.

## Additional Context

- [API Server Architecture](./api-server.md)
- [Control Plane Multi Tenancy Architecture](./multi-tenancy.md)
- [Resource Hierarchy](../../../concepts/resource-hierarchy.md)
