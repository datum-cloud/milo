# Milo Telemetry System

The Milo Telemetry System provides services and components for collecting,
processing, and forwarding telemetry data from Milo API servers to downstream
observability systems.

## Overview

This directory contains telemetry processors that:
- Collect audit logs, resource metrics, and other telemetry data from Milo control planes
- Enrich data with organizational and project context
- Forward processed data to observability platforms (Loki, Prometheus, etc.)

## Components

### Vector Audit Log Processor
**Location**: `vector-audit-log-processor/`

Processes Kubernetes audit logs from both core and project-specific Milo control
planes, adding contextual metadata for organization and project-scoped analysis.

See
[vector-audit-log-processor/README.md](vector-audit-log-processor/README.md)
for detailed documentation.

### Resource Metrics Collector
**Location**: `resource-metrics-collector/`

Collects detailed resource and custom resource state metrics from Milo control planes, exporting these metrics in a Prometheus-compatible format. This enables enhanced observability, alerting, and reporting on Milo APIs and managed objects.

See
[resource-metrics-collector/README.md](resource-metrics-collector/README.md)
for setup and customization of metrics collection for custom Milo resources.

## Getting Started

1. Deploy telemetry components using their individual Kustomize configurations
2. Configure Milo API servers to send telemetry data to the appropriate endpoints
3. Verify data flow in downstream observability systems

For component-specific setup instructions, configuration details, and integration guides, refer to the README files in each component's subdirectory, including the resource-metrics-collector for metrics integration.
