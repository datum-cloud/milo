# Milo Documentation

Technical documentation for Milo, an extensible business operating system that provides powerful APIs for modern service providers to manage their operations and serve consumers.

> [!IMPORTANT]
>
> We're actively building out Milo's documentation. If you have questions or
> need help getting started, [reach out on Slack](https://slack.datum.net)!

## Getting Started

- [**Overview**](getting-started/) - What Milo is and who it's for
- [**Quickstart**](getting-started/quickstart.md) - Get Milo running locally

## Core Concepts

- [**Architecture**](concepts/architecture.md) - System design and components
- [**Resource Hierarchy**](concepts/resource-hierarchy.md) - Organizations,
  projects, and resource relationships

## Developer Guides

### Architecture Deep Dives

- [**Control Plane**](developer-guides/architecture/control-plane/) - API server
  and multi-tenancy patterns
- [**Identity & Access**](developer-guides/architecture/identity/) -
  Authentication and authorization systems

### Integration & Extension

- [**Integration Architecture**](developer-guides/integrations/) - Patterns for
  extending Milo with CRDs, controllers, and webhooks

## API Reference

- [**API Overview**](reference/api/) - Complete API documentation
- [**Resource Manager APIs**](reference/api/resourcemanager.md) - Organizations,
  projects, memberships
- [**IAM APIs**](reference/api/iam.md) - Users, groups, roles, policy bindings
- [**Infrastructure APIs**](reference/api/infrastructure.md) - Project control
  planes and infrastructure management
