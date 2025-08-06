# Integration Architecture

This guide covers high-level approaches for extending and integrating with the
Milo platform. Milo provides multiple extension points that enable service providers to build custom business logic and integrate with external systems.

> [!IMPORTANT]
>
> We're still heavily in the process of building out documentation for Milo. If
> you have questions about how to integrate with or extend Milo to meet your
> needs, [reach out on Slack](https://slack.datum.net)!

## Integration Approaches

Milo supports two primary integration patterns depending on your use case:

### Service Provider Extensions

**Use Case:** Service providers who need to extend Milo's business APIs with custom functionality specific to their domain (e.g., specialized billing models, custom compliance workflows, domain-specific resource types).

**Approach:** Extend Milo's declarative business model using [custom resource definitions][crds], [controllers][controllers], and [admission control policies][admission-controllers] to add new business capabilities.

**Key Components:**

- **Custom Resource Definitions:** Define new business resource types that integrate with Milo's organization and project hierarchy
- **Controllers:** Implement business logic that automatically manages your custom resources
- **Admission Webhooks:** Add custom validation and business rules to ensure data consistency

### Third-Party System Synchronization

**Use Case:** Integrations that keep Milo's business data synchronized with external systems like billing platforms, CRMs, identity providers, or compliance tools.

**Approach:** Build [controllers][controllers] and [webhooks][admission-webhooks] that monitor Milo's business resources and automatically synchronize changes with external systems.

**Key Components:**

- **Watch Controllers:** Monitor changes to Organizations, Projects, Users, and other core business resources
- **Synchronization Logic:** Translate Milo business events into appropriate third-party system operations
- **Status Management:** Report external system state back to Milo resources using [standard status patterns][object-status]

[crds]:
    https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/
    "Custom Resources - Kubernetes"
[controllers]: https://kubernetes.io/docs/concepts/architecture/controller/
    "Controllers - Kubernetes"
[admission-controllers]:
    https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/
    "Admission Controllers - Kubernetes"
[admission-webhooks]:
    https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/
    "Dynamic Admission Control - Kubernetes"
[object-status]:
    https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/#object-spec-and-status
    "Object Spec and Status - Kubernetes"
