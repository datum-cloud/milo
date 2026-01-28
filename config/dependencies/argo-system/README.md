# Argo Events

Deploys Argo Events for event-driven automation in the Milo infrastructure.

## Overview

Argo Events provides:
- Event-driven automation via EventSources and Sensors
- Integration with Milo's API server to watch custom resources
- NATS-based EventBus for reliable event delivery

## Architecture

Argo Events uses a **two-tier architecture**:

```
Infrastructure Cluster (milo-system namespace)
├── Argo Events Controller (manages EventSources/Sensors)
├── EventSource Pods (watch Milo API for resource changes)
├── Sensor Pods (trigger actions based on events)
└── EventBus (NATS - message broker)

Milo API Server
└── Custom Resources (Organizations, Projects, etc.)
```

**Key Point**: The controller runs on the infrastructure cluster and creates
EventSource pods that connect to Milo's API server via kubeconfig. This works
because EventSources use Kubernetes' dynamic client which works with any
Kubernetes-compatible API.

## Configuration

### Deployment Mode
- **Namespaced**: `singleNamespace: true` - controller only watches
  `milo-system` namespace
- **RBAC**: Uses namespace-scoped Roles instead of ClusterRoles
- **ServiceAccount**: `argo-events-sa` created automatically for Sensors and
  EventSources

### EventBus (NATS)
- 3 replicas for high availability
- Token-based authentication
- Persistent storage (10Gi per replica)
- Deployed via `extraObjects` in the HelmRelease

### Connecting to Milo

EventSource pods connect to Milo by mounting a kubeconfig secret via
`spec.template`:

```yaml
template:
  container:
    env:
      - name: KUBECONFIG
        value: /etc/milo/kubeconfig
    volumeMounts:
      - name: milo-kubeconfig
        mountPath: /etc/milo
  volumes:
    - name: milo-kubeconfig
      secret:
        secretName: eventsource-milo-kubeconfig
```

## Usage

See
[./examples/organization-notifications](./examples/organization-notificationsREADME.md)
for a complete example of watching Milo Organizations and sending Slack
notifications.

## Debugging

```bash
# Check controller status
kubectl get pods -n milo-system -l app.kubernetes.io/name=argo-events-controller-manager

# View EventSources and Sensors
kubectl get eventsources,sensors -n milo-system

# Check EventBus
kubectl get eventbus -n milo-system

# View logs
kubectl logs -n milo-system -l app.kubernetes.io/name=argo-events-controller-manager
```

## Related Documentation

- [Argo Events Docs](https://argoproj.github.io/argo-events/)
- [Organization Notifications
  Example](../../argo-events/organization-notifications/README.md)
