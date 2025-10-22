### Milo performance runner

This performance suite provisions Milo/Etcd service monitors and measures CPU/Memory snapshots from VictoriaMetrics.

Files and structure:
- performance/scripts/perf_run.py: runner script executed inside a Kubernetes Job
- performance/config/perf-runner-job.yaml: Job template for the run phase
- performance/config/perf-cleanup-job.yaml: Job template for cleanup
- performance/config/perf-runner-rbac.yaml: ServiceAccount/Role/RoleBinding used by the jobs

#### Summary

- Creates a Milo `Organization`, then N `Projects`, waits for all to be Ready, and times it.
- Takes metrics snapshots before (baseline), after projects are ready, and optionally after per-project object creation.
- Optionally creates M `Secrets` and K `ConfigMaps` in each Project (parallelized), then measures again.
- Saves results to a ConfigMap and downloads a local HTML report and JSON.

#### Prerequisites

1) Bring up dev stack and observability:

```bash
task dev:setup && task dev:install-observability
```

2) Ensure a Milo kubeconfig secret exists in your cluster. By default the tasks mount `Secret/milo-controller-manager-kubeconfig` (key `kubeconfig`). You can override via env (see knobs below).

#### How to run

- Full run (org + projects + objects) with defaults:

```bash
task perf:run
```

- Projects-only (skip secrets/configmaps) and higher parallelism:

```bash
task perf:run -- RUN_OBJECTS_PHASE=false PROJECT_CONCURRENCY=10
```

- Cleanup all resources from the last run:

```bash
task perf:cleanup
```

#### Outputs

- In-cluster: ConfigMap `perf-results` in `NS` (default `milo-system`) with keys `results.json`, `report.html`, `test_id`, `org_name`.
- Local: `reports/perf/<test_id>/results.json` and `report.html` downloaded by the task after the Job completes. The HTML report includes grouped bar charts (CPU cores and Memory MB) and per-project delta KPIs for apiserver and etcd.

#### What the runner does

1) Baseline: query VictoriaMetrics for Milo apiserver and etcd CPU/memory.
2) Create Organization (no wait), then create N Projects, wait for all Projects Ready; record duration.
3) Stabilize, then snapshot “after projects”.
4) If enabled, create per-Project objects (Secrets/ConfigMaps) concurrently; stabilize, then snapshot “after secrets+configmaps”.

Snapshots come from VictoriaMetrics using `container_cpu_usage_seconds_total` (rate) and `container_memory_working_set_bytes` (avg_over_time) for pods matching the configured namespace and pod name regexes.

#### Configuration knobs (env vars)

Pass on the `task perf:run -- KEY=value ...` command line. Defaults shown in parentheses.

- Resource selection
  - `NS` (milo-system): Namespace to run Job and store results ConfigMap
  - `MILO_NAMESPACE` (milo-system): Namespace to measure apiserver/etcd pods
  - `APISERVER_POD_REGEX` (milo-apiserver.*): Regex for apiserver pods
  - `ETCD_POD_REGEX` (etcd.*): Regex for etcd pods

- Metrics source (VictoriaMetrics)
  - `VM_NAMESPACE` (telemetry-system)
  - `VM_SERVICE_NAME` (vmsingle-telemetry-system-vm-victoria-metrics-k8s-stack)
  - `VM_PORT` (8428)
  - `VM_BASE_URL` (optional override, e.g. http://hostname:8428). Default uses in-cluster FQDN: `http://<service>.<namespace>.svc.cluster.local:8428`.
  - `MEASURE_WINDOW` (2m): Range window for rate/avg_over_time

- Scale and workload
  - `NUM_PROJECTS` (100)
  - `RUN_OBJECTS_PHASE` (true): Toggle per-project Secrets/ConfigMaps phase
  - `NUM_SECRETS_PER_PROJECT` (100)
  - `NUM_CONFIGMAPS_PER_PROJECT` (100)
  - `PROJECT_CONCURRENCY` (4): Number of projects processed in parallel when creating objects
  - `OBJECT_CONCURRENCY` (8): Secrets/ConfigMaps parallelism inside each project

- Stabilization windows
  - `STABILIZE_SECONDS` (90): Sleep before snapshots after Projects and after Objects

- Identity / scoping
  - `ORG_NAME` (auto-generated): Name of Organization to create
  - `MILO_KUBECONFIG_SECRET_NAME` (milo-controller-manager-kubeconfig): Secret containing Milo kubeconfig
  - `MILO_KUBECONFIG_SECRET_KEY` (kubeconfig): Secret key with kubeconfig content
  - `MILO_KUBECONFIG_PATH` (/work/milo-kubeconfig): In-container path to mount kubeconfig
  - `AUTH_BEARER_TOKEN` (optional): Override token injected into kubeconfig user for troubleshooting

#### Examples

- Measure project-only impact:

```bash
task perf:run -- RUN_OBJECTS_PHASE=false STABILIZE_SECONDS=60 NUM_PROJECTS=200
```

- Heavier objects phase, more parallelism:

```bash
task perf:run -- NUM_SECRETS_PER_PROJECT=500 NUM_CONFIGMAPS_PER_PROJECT=500 PROJECT_CONCURRENCY=12 OBJECT_CONCURRENCY=24
```

- Point to a custom VictoriaMetrics endpoint:

```bash
task perf:run -- VM_BASE_URL=http://vm.my-domain.local:8428
```

- Use a specific Organization name:

```bash
task perf:run -- ORG_NAME=perf-cow
```
