# API Reference

Welcome to the Milo API reference documentation. Milo provides a comprehensive
API that enables modern service providers to support and operate their business.

## Overview

Milo's API follows standard Kubernetes API conventions and provides declarative
resource management through custom resource definitions (CRDs). All resources
support standard operations including `create`, `get`, `list`, `update`,
`delete`, and `watch`.

## API Groups

API groups provide a way to organize and version related resources in Milo.
Every service in Milo is expected to use a unique API group that properly
categorizes the service and it's resources. API groups allow multiple resource
types to be grouped together under a common domain, enabling better
organization, versioning, and API evolution. Each API group has its own URL
path, version, and resource definitions.

In Milo, API groups are identified by their domain name (`iam.miloapis.com`, or
custom domains like `datumapis.com`). Resources within an API group share common
characteristics, lifecycle management patterns, and often work together to
provide cohesive functionality.


### Resource Manager API

The Resource Manager API provides core organizational resources and hierarchy
management for consumers. Consumers can leverage the resource manager service to
organize resources within their organizations.

[View detailed Resource Manager API documentation](resourcemanager.md)

### Identity and Access Management API

The IAM API handles authentication, authorization, and access control across all
resources registered with the Milo platform. Service providers and consumers can
leverage this service to control who has access to resources they provision in
Milo.

[View detailed IAM API documentation](iam.md)

### Infrastructure API

The Infrastructure API is used for managing infrastructure in the infrastructure
control plane that Milo is running in. This is primarily used to orchestrate the
deployment of Project Control Planes.

> [!NOTE]
>
> This service will be removed in the future when project control planes are
> virtualized.

[View detailed Infrastructure API documentation](infrastructure.md)
