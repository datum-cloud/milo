# API Server Audit Logging Component

A Kustomize component that patches Milo API server deployments to enable
Kubernetes audit logging. This component adds the necessary configuration to
send audit events to the Milo telemetry system for processing and analysis.

## Usage

Apply this component to API server deployments to enable audit logging:

```yaml
# kustomization.yaml
components:
  - ../../components/apiserver-audit-logging
```

## Configuration

This component configures API servers with:
- Audit policy settings defining which events to capture
- Webhook configuration to send audit logs to telemetry processors
- Required volume mounts and configuration files

For detailed information on Kubernetes audit logging configuration options, see
the [Kubernetes Auditing Documentation](https://kubernetes.io/docs/tasks/debug/debug-cluster/audit/).

> [!IMPORTANT]
>
> This component **automatically mounts** the audit policy configuration file
> and the auditing webhook configuration file into the API server deployment at:
>
> `/etc/kubernetes/config/audit-policy-config.yaml` and
> `/etc/kubernetes/config/audit-webhook-config.yaml` respectively.
>
> If you need to change these mount locations, update the environment variables
> and volume mounts via deployment patches as appropriate.

## Audit Log Processing

Once enabled, audit logs are sent to the Milo telemetry system for processing
and forwarding to observability platforms. For details on how audit logs are
collected, enriched, and processed, see the
[Telemetry System README](../../telemetry/README.md).
