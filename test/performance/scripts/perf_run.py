import base64
import json
import concurrent.futures
import os
import sys
import time
import uuid
from datetime import datetime, timezone
from io import BytesIO

import requests
import urllib3
import yaml
from kubernetes import client as k8s_client
from kubernetes import config as k8s_config
from kubernetes.client import ApiException


def get_env(name: str, default: str | None = None) -> str:
    value = os.getenv(name, default)
    if value is None:
        print(f"Missing required env var: {name}", file=sys.stderr)
        sys.exit(1)
    return value


def parse_bool(value: str | None, default: bool = True) -> bool:
    if value is None:
        return default
    return value.strip().lower() in {"1", "true", "t", "yes", "y", "on"}


def load_yaml_file(path: str) -> dict:
    with open(path, "r", encoding="utf-8") as f:
        return yaml.safe_load(f)


# Reduce noisy TLS warnings from in-cluster/self-signed configs
urllib3.disable_warnings(urllib3.exceptions.InsecureRequestWarning)
try:
    # Also quiet requests' vendored urllib3, if present
    requests.packages.urllib3.disable_warnings(category=urllib3.exceptions.InsecureRequestWarning)  # type: ignore
except Exception:
    pass


def log(message: str) -> None:
    now = datetime.now(timezone.utc).strftime("%H:%M:%S")
    print(f"[{now}] {message}", flush=True)


def should_retry_api_exception(e: ApiException) -> bool:
    try:
        if e.status in (429, 500):
            return True
        body = getattr(e, 'body', '') or ''
        msg = str(body)
        # Handle webhook EOF/transient failures
        if 'Internal error occurred' in msg or 'failed calling webhook' in msg or 'EOF' in msg:
            return True
    except Exception:
        pass
    return False


def retry_with_backoff(action_name: str, fn, *args, **kwargs):
    delay = 1.0
    for attempt in range(1, 7):
        try:
            return fn(*args, **kwargs)
        except ApiException as e:
            if should_retry_api_exception(e) and attempt < 6:
                log(f"{action_name} failed (attempt {attempt}/6): status={e.status} retrying in {delay:.0f}s …")
                time.sleep(delay)
                delay = min(delay * 2, 16)
                continue
            raise


def save_results_configmap(namespace: str, name: str, data: dict[str, str]) -> None:
    # Uses in-cluster config to write to Kubernetes ConfigMap
    try:
        k8s_config.load_incluster_config()
    except Exception:
        # Fallback to default kubeconfig for local runs
        k8s_config.load_kube_config()

    v1 = k8s_client.CoreV1Api()
    metadata = k8s_client.V1ObjectMeta(name=name)
    cm = k8s_client.V1ConfigMap(metadata=metadata, data=data)

    try:
        existing = v1.read_namespaced_config_map(name=name, namespace=namespace)
        existing.data = data
        v1.replace_namespaced_config_map(name=name, namespace=namespace, body=existing)
    except ApiException as e:
        if e.status == 404:
            v1.create_namespaced_config_map(namespace=namespace, body=cm)
        else:
            raise


def save_checkpoint(namespace: str, test_id: str, org_name: str, phase: str, extra: dict | None = None) -> None:
    data: dict[str, str] = {
        "test_id": test_id,
        "org_name": org_name,
        "phase": phase,
    }
    if extra:
        for k, v in extra.items():
            try:
                data[k] = json.dumps(v) if isinstance(v, (dict, list)) else str(v)
            except Exception:
                data[k] = str(v)
    save_results_configmap(namespace, "perf-results", data)


def http_get_json(url: str, params: dict | None = None) -> dict:
    # Basic retry for transient VM connectivity (EOF, connection reset, etc.)
    last_err: Exception | None = None
    for attempt in range(6):
        try:
            resp = requests.get(url, params=params, timeout=30)
            resp.raise_for_status()
            return resp.json()
        except Exception as e:
            last_err = e
            sleep_s = 2 * (attempt + 1)
            log(f"[metrics] request failed (attempt {attempt+1}/6): {e}; retrying in {sleep_s}s")
            time.sleep(sleep_s)
    assert last_err is not None
    raise last_err


def prom_query(base_url: str, query: str, context: str | None = None) -> float:
    url = f"{base_url.rstrip('/')}/api/v1/query"
    start = time.time()
    data = http_get_json(url, params={"query": query})
    duration = time.time() - start
    if context:
        log(f"[metrics] {context} took {duration:.1f}s")
    if data.get("status") != "success":
        raise RuntimeError(f"Prom query failed: {data}")
    result = data.get("data", {}).get("result", [])
    if not result:
        if context:
            log(f"[metrics] {context} returned empty result")
        return 0.0
    # Use the first scalar/vector value (sum/avg queries should return single series)
    value = float(result[0]["value"][1])
    if context:
        log(f"[metrics] {context} value={value}")
    return value


def measure_metrics(base_url: str, namespace: str, apiserver_regex: str, etcd_regex: str, window: str) -> dict:
    # CPU is in cores (rate over window). Memory in bytes (avg over window)
    log(f"[metrics] VM_BASE_URL={base_url} namespace={namespace} window={window}")
    log("[metrics] querying apiserver cpu/memory and etcd cpu/memory …")
    # Pre-flight series counts to aid debugging
    try:
        ns_cpu_series = prom_query(
            base_url,
            f'count(container_cpu_usage_seconds_total{{namespace="{namespace}"}})',
            context="series_count_ns_cpu",
        )
        ns_mem_series = prom_query(
            base_url,
            f'count(container_memory_working_set_bytes{{namespace="{namespace}"}})',
            context="series_count_ns_mem",
        )
        apiserver_series = prom_query(
            base_url,
            f'count(container_cpu_usage_seconds_total{{namespace="{namespace}",pod=~"{apiserver_regex}"}})',
            context="series_count_apiserver",
        )
        etcd_series = prom_query(
            base_url,
            f'count(container_cpu_usage_seconds_total{{namespace="{namespace}",pod=~"{etcd_regex}"}})',
            context="series_count_etcd",
        )
        log(
            f"[metrics] series counts: ns_cpu={ns_cpu_series} ns_mem={ns_mem_series} apiserver={apiserver_series} etcd={etcd_series}"
        )
    except Exception as e:
        log(f"[metrics] pre-flight series counts failed: {e}")
    def run_queries(pod_label: str, include_container_filter: bool) -> dict[str, float]:
        container_filter = 'container!="",container!="POD"' if include_container_filter else ''
        comma = ',' if include_container_filter else ''
        # Build label selectors without f-strings to avoid brace escaping issues
        apiserver_selector = '{namespace="%s",%s=~"%s"%s%s}' % (
            namespace,
            pod_label,
            apiserver_regex,
            comma,
            container_filter,
        )
        etcd_selector = '{namespace="%s",%s=~"%s"%s%s}' % (
            namespace,
            pod_label,
            etcd_regex,
            comma,
            container_filter,
        )
        queries: dict[str, str] = {
            "apiserver_cpu_cores": 'sum(rate(container_cpu_usage_seconds_total%s[%s]))' % (apiserver_selector, window),
            "apiserver_mem_bytes": 'sum(avg_over_time(container_memory_working_set_bytes%s[%s]))' % (apiserver_selector, window),
            "etcd_cpu_cores": 'sum(rate(container_cpu_usage_seconds_total%s[%s]))' % (etcd_selector, window),
            "etcd_mem_bytes": 'sum(avg_over_time(container_memory_working_set_bytes%s[%s]))' % (etcd_selector, window),
        }
        for k, q in queries.items():
            log(f"[metrics] query[{k}] (label={pod_label}, filter={include_container_filter}): {q}")
        results: dict[str, float] = {}
        with concurrent.futures.ThreadPoolExecutor(max_workers=4) as executor:
            future_to_key = {executor.submit(prom_query, base_url, query, k): k for k, query in queries.items()}
            for future in concurrent.futures.as_completed(future_to_key):
                k = future_to_key[future]
                try:
                    results[k] = future.result()
                except Exception as e:
                    log(f"[metrics] query failed for {k}: {e}; using 0")
                    results[k] = 0.0
        return results

    # Try multiple label variants and filters; return first non-zero set
    for pod_label, with_filter in [("pod", True), ("pod", False), ("pod_name", True), ("pod_name", False)]:
        log(f"[metrics] attempting with pod_label={pod_label} filter={with_filter}")
        results = run_queries(pod_label, with_filter)
        if any(v != 0.0 for v in results.values()):
            return results

    log("[metrics] all query variants returned 0; returning zeros")
    return {"apiserver_cpu_cores": 0.0, "apiserver_mem_bytes": 0.0, "etcd_cpu_cores": 0.0, "etcd_mem_bytes": 0.0}


def find_condition(conditions: list[dict] | None, ctype: str) -> dict | None:
    if not conditions:
        return None
    for c in conditions:
        if c.get("type") == ctype:
            return c
    return None


def wait_condition_ready(
    coapi: k8s_client.CustomObjectsApi,
    group: str,
    version: str,
    plural: str,
    name: str,
    timeout_s: int = 600,
    log_context: str | None = None,
) -> None:
    start = time.time()
    deadline = start + timeout_s
    last_log = 0.0
    while time.time() < deadline:
        obj = coapi.get_cluster_custom_object(group=group, version=version, plural=plural, name=name)
        cond = find_condition(obj.get("status", {}).get("conditions"), "Ready")
        if cond and str(cond.get("status")) == "True":
            return
        # Periodic detail log to explain why we're still waiting
        now = time.time()
        if now - start >= 10 and now - last_log >= 15:
            last_log = now
            if cond:
                reason = cond.get("reason", "")
                message = cond.get("message", "")
                status = cond.get("status", "Unknown")
                ctx = f"{plural}/{name}" if not log_context else f"{log_context} ({plural}/{name})"
                log(f"waiting for {ctx}: Ready={status} reason={reason} message={message}")
            else:
                ctx = f"{plural}/{name}" if not log_context else f"{log_context} ({plural}/{name})"
                log(f"waiting for {ctx}: no Ready condition yet")
        time.sleep(2)
    raise TimeoutError(f"Timed out waiting for {plural}/{name} Ready")


def build_scoped_kubeconfig(base_cfg: dict, scope_path: str, new_name: str) -> dict:
    cfg = yaml.safe_load(yaml.safe_dump(base_cfg))  # deep copy
    for c in cfg.get("clusters", []):
        server = c["cluster"].get("server", "").rstrip("/")
        c["name"] = new_name
        c["cluster"]["server"] = f"{server}{scope_path}"
    # context names follow cluster names
    if cfg.get("contexts"):
        for ctx in cfg["contexts"]:
            ctx["name"] = new_name
            ctx["context"]["cluster"] = new_name
    if cfg.get("current-context"):
        cfg["current-context"] = new_name
    return cfg


def kube_client_from_config(cfg: dict):
    # Load a kubernetes client from a kubeconfig dict (not a file)
    loader = k8s_config.kube_config.KubeConfigLoader(config_dict=cfg)
    configuration = k8s_client.Configuration()
    loader.load_and_set(configuration)
    return k8s_client.ApiClient(configuration)


def create_org_and_projects(milo_kubeconfig_path: str, org_name: str, num_projects: int, labels: dict[str, str]) -> tuple[dict, list[str], float]:
    base_cfg = load_yaml_file(milo_kubeconfig_path)
    # Optional override to inject a bearer token for auth troubleshooting
    override_token = os.getenv("AUTH_BEARER_TOKEN")
    if override_token:
        try:
            if base_cfg.get("users"):
                base_cfg["users"][0]["user"]["token"] = override_token
                log("Using AUTH_BEARER_TOKEN override for kubeconfig user[0]")
        except Exception:
            pass
    # Client for Milo API server (cluster-scoped CRDs)
    api_client = kube_client_from_config(base_cfg)
    coapi = k8s_client.CustomObjectsApi(api_client)
    try:
        cluster_server = base_cfg.get("clusters", [{}])[0].get("cluster", {}).get("server", "")
        user_name = base_cfg.get("users", [{}])[0].get("name", "")
        log(f"Using cluster-scoped kubeconfig: server={cluster_server} user={user_name}")
    except Exception:
        pass

    # Create Organization
    log(f"Creating Organization '{org_name}' …")
    org_body = {
        "apiVersion": "resourcemanager.miloapis.com/v1alpha1",
        "kind": "Organization",
        "metadata": {"name": org_name, "labels": labels},
        "spec": {"type": "Standard"},
    }
    try:
        retry_with_backoff(
            "create Organization",
            coapi.create_cluster_custom_object,
            group="resourcemanager.miloapis.com",
            version="v1alpha1",
            plural="organizations",
            body=org_body,
        )
    except ApiException as e:
        if e.status != 409:
            raise

    # Do not wait for Organization readiness; proceed immediately to projects
    log(f"Organization '{org_name}' created")

    # Build an organization-scoped kubeconfig so requests carry parent context
    org_scope_path = f"/apis/resourcemanager.miloapis.com/v1alpha1/organizations/{org_name}/control-plane"
    org_cfg = build_scoped_kubeconfig(base_cfg, org_scope_path, new_name=f"organization-{org_name}")
    org_client = kube_client_from_config(org_cfg)
    org_coapi = k8s_client.CustomObjectsApi(org_client)
    try:
        org_server = org_cfg.get("clusters", [{}])[0].get("cluster", {}).get("server", "")
        org_user = org_cfg.get("users", [{}])[0].get("name", "")
        log(f"Using organization-scoped kubeconfig: server={org_server} user={org_user}")
    except Exception:
        pass

    # Create Projects
    project_names: list[str] = []
    start = time.time()
    log(f"Creating {num_projects} Projects …")
    for i in range(1, num_projects + 1):
        pname = f"{org_name}-p-{i:03d}"
        project_names.append(pname)
        proj_body = {
            "apiVersion": "resourcemanager.miloapis.com/v1alpha1",
            "kind": "Project",
            "metadata": {"name": pname, "labels": labels},
            "spec": {"ownerRef": {"kind": "Organization", "name": org_name}},
        }
        try:
            retry_with_backoff(
                f"create Project {pname}",
                org_coapi.create_cluster_custom_object,
                group="resourcemanager.miloapis.com",
                version="v1alpha1",
                plural="projects",
                body=proj_body,
            )
        except ApiException as e:
            if e.status != 409:
                log(f"error creating Project '{pname}': {getattr(e, 'body', e)}")
                raise
        if i % 10 == 0 or i == num_projects:
            log(f"Created {i}/{num_projects} Projects …")

    # Wait for all projects Ready
    ready = 0
    log("Waiting for Projects to become Ready …")
    for pname in project_names:
        wait_condition_ready(org_coapi, "resourcemanager.miloapis.com", "v1alpha1", "projects", pname, timeout_s=900, log_context="Project")
        ready += 1
        if ready % 10 == 0 or ready == len(project_names):
            log(f"Projects Ready: {ready}/{len(project_names)} …")
    end = time.time()
    total_seconds = end - start

    # Return base kubeconfig (for building scoped configs), project names, and duration
    return base_cfg, project_names, total_seconds


def create_objects_in_projects(
    base_cfg: dict,
    org_name: str,
    project_names: list[str],
    num_secrets: int,
    num_configmaps: int,
    labels: dict[str, str],
    project_concurrency: int,
    object_concurrency: int,
) -> None:
    def work_project(pname: str) -> None:
        scope_path = f"/apis/resourcemanager.miloapis.com/v1alpha1/projects/{pname}/control-plane"
        proj_cfg = build_scoped_kubeconfig(base_cfg, scope_path, new_name=f"project-{pname}")
        proj_client = kube_client_from_config(proj_cfg)
        v1 = k8s_client.CoreV1Api(proj_client)

        def create_secret(i: int) -> None:
            sname = f"perf-secret-{i:03d}"
            body = k8s_client.V1Secret(
                metadata=k8s_client.V1ObjectMeta(name=sname, labels=labels),
                string_data={"note": f"secret {i} for {pname}"},
                type="Opaque",
            )
            try:
                v1.create_namespaced_secret(namespace="default", body=body)
            except ApiException as e:
                if e.status != 409:
                    raise

        def create_configmap(i: int) -> None:
            cname = f"perf-configmap-{i:03d}"
            body = k8s_client.V1ConfigMap(
                metadata=k8s_client.V1ObjectMeta(name=cname, labels=labels),
                data={"note": f"configmap {i} for {pname}"},
            )
            try:
                v1.create_namespaced_config_map(namespace="default", body=body)
            except ApiException as e:
                if e.status != 409:
                    raise

        log(f"[{pname}] Creating {num_secrets} Secrets (concurrency={object_concurrency}) …")
        if num_secrets > 0:
            with concurrent.futures.ThreadPoolExecutor(max_workers=object_concurrency) as ex:
                list(ex.map(create_secret, range(1, num_secrets + 1)))
        log(f"[{pname}] Secrets created: {num_secrets}/{num_secrets}")

        log(f"[{pname}] Creating {num_configmaps} ConfigMaps (concurrency={object_concurrency}) …")
        if num_configmaps > 0:
            with concurrent.futures.ThreadPoolExecutor(max_workers=object_concurrency) as ex:
                list(ex.map(create_configmap, range(1, num_configmaps + 1)))
        log(f"[{pname}] ConfigMaps created: {num_configmaps}/{num_configmaps}")

    # Run multiple projects in parallel
    if project_concurrency <= 1:
        for pname in project_names:
            work_project(pname)
    else:
        log(f"Creating objects across projects (concurrency={project_concurrency}) …")
        with concurrent.futures.ThreadPoolExecutor(max_workers=project_concurrency) as ex:
            list(ex.map(work_project, project_names))


def generate_report_html(
    metrics_before: dict,
    metrics_after_projects: dict,
    metrics_after_secrets: dict | None,
    num_projects: int,
    projects_ready_seconds: float,
) -> str:
    # Minimal inline charts using simple ASCII bars as fallback if matplotlib unavailable
    try:
        import matplotlib.pyplot as plt  # type: ignore

        def grouped_bars_png(
            series: list[tuple[str, list[float]]],
            categories: list[str],
            title: str,
            ylabel: str,
        ) -> str:
            fig, ax = plt.subplots(figsize=(7.5, 3.5))
            num_series = len(series)
            x = range(len(categories))
            total_bar_width = 0.8
            bar_width = total_bar_width / max(1, num_series)
            offsets = [(-total_bar_width / 2) + (i + 0.5) * bar_width for i in range(num_series)]
            colors = ["#4c78a8", "#f58518", "#54a24b"]
            for idx, (label, values) in enumerate(series):
                ax.bar([xi + offsets[idx] for xi in x], values, width=bar_width, label=label, color=colors[idx % len(colors)])
            ax.set_title(title)
            ax.set_ylabel(ylabel)
            ax.set_xticks(list(x))
            ax.set_xticklabels(categories)
            ax.legend(loc="upper left", fontsize=8)
            plt.tight_layout()
            buf = BytesIO()
            plt.savefig(buf, format="png")
            plt.close(fig)
            b64 = base64.b64encode(buf.getvalue()).decode("ascii")
            return f"<img alt='{title}' src='data:image/png;base64,{b64}' />"

        # Build series for CPU (cores) and Memory (MB)
        cpu_series: list[tuple[str, list[float]]] = [
            ("baseline", [metrics_before["apiserver_cpu_cores"], metrics_before["etcd_cpu_cores"]]),
            ("after-projects", [metrics_after_projects["apiserver_cpu_cores"], metrics_after_projects["etcd_cpu_cores"]]),
        ]
        mem_series: list[tuple[str, list[float]]] = [
            (
                "baseline",
                [metrics_before["apiserver_mem_bytes"] / (1024 * 1024), metrics_before["etcd_mem_bytes"] / (1024 * 1024)],
            ),
            (
                "after-projects",
                [
                    metrics_after_projects["apiserver_mem_bytes"] / (1024 * 1024),
                    metrics_after_projects["etcd_mem_bytes"] / (1024 * 1024),
                ],
            ),
        ]
        if metrics_after_secrets is not None:
            cpu_series.append(
                (
                    "after-objects",
                    [metrics_after_secrets["apiserver_cpu_cores"], metrics_after_secrets["etcd_cpu_cores"]],
                )
            )
            mem_series.append(
                (
                    "after-objects",
                    [
                        metrics_after_secrets["apiserver_mem_bytes"] / (1024 * 1024),
                        metrics_after_secrets["etcd_mem_bytes"] / (1024 * 1024),
                    ],
                )
            )

        cpu_img = grouped_bars_png(cpu_series, ["apiserver", "etcd"], "CPU (cores)", "cores")
        mem_img = grouped_bars_png(mem_series, ["apiserver", "etcd"], "Memory (MB)", "MB")

        # Quick stats and deltas
        t_total = projects_ready_seconds
        per_project_s = (t_total / num_projects) if num_projects > 0 else 0.0
        def delta(a: float, b: float) -> float:
            return b - a
        apiserver_mem_delta_mb = delta(
            metrics_before["apiserver_mem_bytes"] / (1024 * 1024),
            metrics_after_projects["apiserver_mem_bytes"] / (1024 * 1024),
        )
        etcd_mem_delta_mb = delta(
            metrics_before["etcd_mem_bytes"] / (1024 * 1024),
            metrics_after_projects["etcd_mem_bytes"] / (1024 * 1024),
        )
        apiserver_cpu_delta = delta(metrics_before["apiserver_cpu_cores"], metrics_after_projects["apiserver_cpu_cores"])
        etcd_cpu_delta = delta(metrics_before["etcd_cpu_cores"], metrics_after_projects["etcd_cpu_cores"])

        # Per-project implications (naive average impact per created Project)
        per_proj_cpu_apiserver = (apiserver_cpu_delta / num_projects) if num_projects > 0 else 0.0
        per_proj_cpu_etcd = (etcd_cpu_delta / num_projects) if num_projects > 0 else 0.0
        per_proj_mem_apiserver_mb = (apiserver_mem_delta_mb / num_projects) if num_projects > 0 else 0.0
        per_proj_mem_etcd_mb = (etcd_mem_delta_mb / num_projects) if num_projects > 0 else 0.0

        after_objects_stats = ""
        if metrics_after_secrets is not None:
            apiserver_mem_delta_mb_obj = delta(
                metrics_after_projects["apiserver_mem_bytes"] / (1024 * 1024),
                metrics_after_secrets["apiserver_mem_bytes"] / (1024 * 1024),
            )
            etcd_mem_delta_mb_obj = delta(
                metrics_after_projects["etcd_mem_bytes"] / (1024 * 1024),
                metrics_after_secrets["etcd_mem_bytes"] / (1024 * 1024),
            )
            apiserver_cpu_delta_obj = delta(
                metrics_after_projects["apiserver_cpu_cores"], metrics_after_secrets["apiserver_cpu_cores"]
            )
            etcd_cpu_delta_obj = delta(
                metrics_after_projects["etcd_cpu_cores"], metrics_after_secrets["etcd_cpu_cores"]
            )
            per_proj_cpu_apiserver_obj = (apiserver_cpu_delta_obj / num_projects) if num_projects > 0 else 0.0
            per_proj_cpu_etcd_obj = (etcd_cpu_delta_obj / num_projects) if num_projects > 0 else 0.0
            per_proj_mem_apiserver_mb_obj = (apiserver_mem_delta_mb_obj / num_projects) if num_projects > 0 else 0.0
            per_proj_mem_etcd_mb_obj = (etcd_mem_delta_mb_obj / num_projects) if num_projects > 0 else 0.0
            after_objects_stats = f"""
<div class="card">
  <div class="card-title">After objects (per-project deltas)</div>
  <div class="kpis">
    <div class="kpi"><div class="kpi-label">CPU apiserver</div><div class="kpi-value">{per_proj_cpu_apiserver_obj:+.4f} cores</div></div>
    <div class="kpi"><div class="kpi-label">CPU etcd</div><div class="kpi-value">{per_proj_cpu_etcd_obj:+.4f} cores</div></div>
    <div class="kpi"><div class="kpi-label">MEM apiserver</div><div class="kpi-value">{per_proj_mem_apiserver_mb_obj:+.2f} MB</div></div>
    <div class="kpi"><div class="kpi-label">MEM etcd</div><div class="kpi-value">{per_proj_mem_etcd_mb_obj:+.2f} MB</div></div>
  </div>
</div>
"""

        html = """
<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <title>Milo Performance Report</title>
    <style>
      body {{ font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Oxygen, Ubuntu, Cantarell, "Helvetica Neue", Arial, sans-serif; color: #1f2937; margin: 24px; }}
      h1 {{ margin: 0 0 8px; font-size: 22px; }}
      .subtitle {{ color: #6b7280; margin-bottom: 20px; }}
      .section {{ margin: 24px 0; }}
      .grid {{ display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 12px; }}
      .card {{ border: 1px solid #e5e7eb; border-radius: 8px; padding: 12px; background: #fff; }}
      .card-title {{ font-weight: 600; margin-bottom: 8px; color: #374151; }}
      .kpis {{ display: grid; grid-template-columns: repeat(4, minmax(120px, 1fr)); gap: 8px; }}
      .kpi {{ background: #f9fafb; border: 1px solid #f3f4f6; border-radius: 6px; padding: 8px; }}
      .kpi-label {{ color: #6b7280; font-size: 12px; }}
      .kpi-value {{ font-size: 14px; font-weight: 600; }}
      .charts {{ display: grid; grid-template-columns: 1fr; gap: 16px; }}
      @media (min-width: 900px) {{ .charts {{ grid-template-columns: 1fr 1fr; }} }}
      .img {{ border: 1px solid #e5e7eb; border-radius: 8px; padding: 8px; background: #fff; text-align: center; }}
    </style>
  </head>
  <body>
    <h1>Milo Performance Report</h1>
    <div class="subtitle">Projects: {num_projects} • Time to Ready: {t_total:.1f}s ({per_project_s:.2f}s/project)</div>

    <div class="section charts">
      <div class="img">{cpu_img}</div>
      <div class="img">{mem_img}</div>
    </div>

    <div class="section">
      <div class="card">
        <div class="card-title">After-projects (per-project deltas)</div>
        <div class="kpis">
          <div class="kpi"><div class="kpi-label">CPU apiserver</div><div class="kpi-value">{per_proj_cpu_apiserver:+.4f} cores</div></div>
          <div class="kpi"><div class="kpi-label">CPU etcd</div><div class="kpi-value">{per_proj_cpu_etcd:+.4f} cores</div></div>
          <div class="kpi"><div class="kpi-label">MEM apiserver</div><div class="kpi-value">{per_proj_mem_apiserver_mb:+.2f} MB</div></div>
          <div class="kpi"><div class="kpi-label">MEM etcd</div><div class="kpi-value">{per_proj_mem_etcd_mb:+.2f} MB</div></div>
        </div>
      </div>
      {after_objects_stats}
    </div>
  </body>
</html>
"""
        return html.format(
            cpu_img=cpu_img,
            mem_img=mem_img,
            num_projects=num_projects,
            t_total=t_total,
            per_project_s=per_project_s,
            apiserver_mem_delta_mb=apiserver_mem_delta_mb,
            etcd_mem_delta_mb=etcd_mem_delta_mb,
            apiserver_cpu_delta=apiserver_cpu_delta,
            etcd_cpu_delta=etcd_cpu_delta,
            per_proj_cpu_apiserver=per_proj_cpu_apiserver,
            per_proj_cpu_etcd=per_proj_cpu_etcd,
            per_proj_mem_apiserver_mb=per_proj_mem_apiserver_mb,
            per_proj_mem_etcd_mb=per_proj_mem_etcd_mb,
            after_objects_stats=after_objects_stats,
        )
    except Exception as e:  # Fallback text-only report with error context
        payload: dict[str, object] = {
            "baseline": metrics_before,
            "after_projects": metrics_after_projects,
            "num_projects": num_projects,
            "projects_ready_seconds": projects_ready_seconds,
        }
        if metrics_after_secrets is not None:
            payload["after_secrets"] = metrics_after_secrets
        return (
            "<pre>chart rendering unavailable; showing raw metrics\n\n"
            + "error: "
            + str(e)
            + "\n\n"
            + json.dumps(payload, indent=2)
            + "</pre>"
        )


def cleanup_resources(milo_kubeconfig_path: str, test_id: str, org_name: str) -> None:
    base_cfg = load_yaml_file(milo_kubeconfig_path)
    api_client = kube_client_from_config(base_cfg)
    coapi = k8s_client.CustomObjectsApi(api_client)

    # Use organization-scoped client for project-scoped operations (admission context)
    org_scope_path = f"/apis/resourcemanager.miloapis.com/v1alpha1/organizations/{org_name}/control-plane"
    org_cfg = build_scoped_kubeconfig(base_cfg, org_scope_path, new_name=f"organization-{org_name}")
    org_client = kube_client_from_config(org_cfg)
    org_coapi = k8s_client.CustomObjectsApi(org_client)

    # List and delete projects by label
    proj_list = coapi.list_cluster_custom_object(
        group="resourcemanager.miloapis.com",
        version="v1alpha1",
        plural="projects",
        label_selector=f"app=milo-perf,test-id={test_id}",
    )
    for item in proj_list.get("items", []):
        pname = item["metadata"]["name"]
        log(f"[cleanup] project {pname}: deleting Secrets/ConfigMaps and Project …")
        # Delete per-project objects first (tolerate not found)
        try:
            proj_scope_path = f"/apis/resourcemanager.miloapis.com/v1alpha1/projects/{pname}/control-plane"
            proj_cfg = build_scoped_kubeconfig(base_cfg, proj_scope_path, new_name=f"project-{pname}")
            proj_client = kube_client_from_config(proj_cfg)
            v1 = k8s_client.CoreV1Api(proj_client)
            label_sel = f"app=milo-perf,test-id={test_id}"
            # Secrets
            try:
                sl = v1.list_namespaced_secret(namespace="default", label_selector=label_sel)
                for s in sl.items or []:
                    try:
                        v1.delete_namespaced_secret(name=s.metadata.name, namespace="default")
                    except ApiException as e:
                        if e.status != 404:
                            raise
            except ApiException:
                pass
            # ConfigMaps
            try:
                cml = v1.list_namespaced_config_map(namespace="default", label_selector=label_sel)
                for cm in cml.items or []:
                    try:
                        v1.delete_namespaced_config_map(name=cm.metadata.name, namespace="default")
                    except ApiException as e:
                        if e.status != 404:
                            raise
            except ApiException:
                pass
        except Exception:
            # Keep going even if per-project object cleanup fails
            pass

        # Delete the Project (use org-scoped API for proper parent context)
        try:
            org_coapi.delete_cluster_custom_object(
                group="resourcemanager.miloapis.com",
                version="v1alpha1",
                plural="projects",
                name=pname,
            )
        except ApiException as e:
            if e.status not in (404, 409):
                raise

    # Delete organization last
    try:
        coapi.delete_cluster_custom_object(
            group="resourcemanager.miloapis.com",
            version="v1alpha1",
            plural="organizations",
            name=org_name,
        )
    except ApiException as e:
        if e.status not in (404, 409):
            raise


def main() -> None:
    run_mode = os.getenv("RUN_MODE", "run").lower()
    target_ns = get_env("TARGET_NAMESPACE", "milo-system")

    if run_mode == "cleanup":
        milo_kubeconfig_path = get_env("MILO_KUBECONFIG_PATH", "/work/milo-kubeconfig")
        test_id = get_env("TEST_ID")
        org_name = get_env("ORG_NAME")
        cleanup_resources(milo_kubeconfig_path, test_id=test_id, org_name=org_name)
        # Remove results ConfigMap if present
        try:
            k8s_config.load_incluster_config()
        except Exception:
            k8s_config.load_kube_config()
        v1 = k8s_client.CoreV1Api()
        try:
            v1.delete_namespaced_config_map(name="perf-results", namespace=target_ns)
        except ApiException as e:
            if e.status != 404:
                raise
        print("Cleanup complete")
        return

    # RUN
    milo_kubeconfig_path = get_env("MILO_KUBECONFIG_PATH", "/work/milo-kubeconfig")
    milo_metrics_ns = get_env("MILO_NAMESPACE", "milo-system")
    vm_base_url = get_env("VM_BASE_URL")
    apiserver_regex = get_env("APISERVER_POD_REGEX", "milo-apiserver.*")
    etcd_regex = get_env("ETCD_POD_REGEX", "etcd.*")
    window = get_env("MEASURE_WINDOW", "2m")
    stabilize_seconds = int(get_env("STABILIZE_SECONDS", "90"))
    num_projects = int(get_env("NUM_PROJECTS", "100"))
    num_secrets = int(get_env("NUM_SECRETS_PER_PROJECT", "100"))
    num_configmaps = int(get_env("NUM_CONFIGMAPS_PER_PROJECT", "100"))
    project_concurrency = int(os.getenv("PROJECT_CONCURRENCY", "4"))
    object_concurrency = int(os.getenv("OBJECT_CONCURRENCY", "8"))
    run_objects_phase = parse_bool(os.getenv("RUN_OBJECTS_PHASE", "true"), default=True)
    out_dir = os.getenv("OUT_DIR", "/work/out")

    test_id = uuid.uuid4().hex[:8]
    _org_env = os.getenv("ORG_NAME")
    org_name = _org_env.strip() if (_org_env is not None and _org_env.strip() != "") else f"perf-{test_id}"
    labels = {"app": "milo-perf", "test-id": test_id}

    # Initial checkpoint (so cleanup works even if run aborts early)
    save_checkpoint(target_ns, test_id, org_name, phase="init", extra={"num_projects": num_projects})

    # Baseline metrics (no pre-stabilization)
    log("Measuring baseline metrics …")
    baseline = measure_metrics(vm_base_url, milo_metrics_ns, apiserver_regex, etcd_regex, window)

    # Create org & projects
    log(f"Creating org '{org_name}' and {num_projects} projects …")
    base_cfg, project_names, projects_ready_seconds = create_org_and_projects(
        milo_kubeconfig_path, org_name, num_projects, labels
    )
    # Update checkpoint that org exists (projects will be created next)
    save_checkpoint(target_ns, test_id, org_name, phase="org-created")

    # After projects metrics
    if stabilize_seconds > 0:
        log(f"Stabilizing for {stabilize_seconds}s after projects are Ready …")
        time.sleep(stabilize_seconds)
    log("Measuring metrics after projects are Ready …")
    after_projects = measure_metrics(vm_base_url, milo_metrics_ns, apiserver_regex, etcd_regex, window)
    # Update checkpoint after projects are ready
    save_checkpoint(target_ns, test_id, org_name, phase="projects-ready", extra={"num_projects": num_projects})

    after_secrets = None
    if run_objects_phase:
        # Create objects within each project
        log(f"Creating {num_secrets} secrets and {num_configmaps} configmaps per project …")
        create_objects_in_projects(
            base_cfg,
            org_name,
            project_names,
            num_secrets,
            num_configmaps,
            labels,
            project_concurrency,
            object_concurrency,
        )

        # After secrets/configmaps metrics
        if stabilize_seconds > 0:
            log(f"Stabilizing for {stabilize_seconds}s after secrets/configmaps …")
            time.sleep(stabilize_seconds)
        log("Measuring metrics after creating secrets/configmaps …")
        after_secrets = measure_metrics(vm_base_url, milo_metrics_ns, apiserver_regex, etcd_regex, window)

    # Build results
    now_iso = datetime.now(timezone.utc).isoformat()
    results = {
        "test_id": test_id,
        "timestamp": now_iso,
        "org_name": org_name,
        "num_projects": num_projects,
        "num_secrets_per_project": num_secrets,
        "num_configmaps_per_project": num_configmaps,
        "projects_ready_seconds": projects_ready_seconds,
        "metrics": {
            "baseline": baseline,
            "after_projects": after_projects,
            "after_secrets": after_secrets,
        },
    }

    report_html = generate_report_html(
        baseline,
        after_projects,
        after_secrets,
        num_projects,
        projects_ready_seconds,
    )

    # Persist results to files (Task will copy locally and publish a ConfigMap)
    try:
        os.makedirs(out_dir, exist_ok=True)
        with open(os.path.join(out_dir, "results.json"), "w", encoding="utf-8") as f:
            json.dump(results, f, indent=2)
        with open(os.path.join(out_dir, "report.html"), "w", encoding="utf-8") as f:
            f.write(report_html)
        with open(os.path.join(out_dir, "meta.txt"), "w", encoding="utf-8") as f:
            f.write(f"test_id={test_id}\norg_name={org_name}\n")
        log(f"Results written to {out_dir}")
    except Exception as e:
        log(f"Failed to write results to {out_dir}: {e}")

    # Best-effort attempt to also write a ConfigMap (may fail if SA lacks RBAC)
    try:
        cm_data = {
            "results.json": json.dumps(results, indent=2),
            "report.html": report_html,
            "test_id": test_id,
            "org_name": org_name,
        }
        save_results_configmap(target_ns, "perf-results", cm_data)
        log("Also saved results to ConfigMap 'perf-results'")
    except Exception as e:
        log(f"Skipping ConfigMap save (insufficient RBAC?): {e}")

    log("Perf run complete")


if __name__ == "__main__":
    main()


