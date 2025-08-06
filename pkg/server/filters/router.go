package filters

import (
	"net/http"
	"strings"

	"go.miloapis.com/milo/pkg/workspaces"
	"k8s.io/klog/v2"
)

// ProjectRouter proxies requests that match
// “…/projects/<id>/control-plane/<k8s-path>” to an in-process workspace server.
func ProjectRouter(table *workspaces.Table) func(http.Handler) http.Handler {
	const (
		projectsSeg     = "/projects/"
		controlPlaneSeg = "/control-plane"
	)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// ── 1. Fast-path: is “/projects/” even present? ────────────────
			idx := strings.Index(r.URL.Path, projectsSeg)
			if idx < 0 {
				next.ServeHTTP(w, r) // not project-scoped
				return
			}

			// tail = "<proj-id>/control-plane/…"
			tail := r.URL.Path[idx+len(projectsSeg):]
			if tail == "" {
				http.Error(w, "missing project id", http.StatusBadRequest)
				return
			}

			slash := strings.IndexByte(tail, '/')
			if slash < 0 {
				// “…/projects/foo”  → *not* a workspace URL
				next.ServeHTTP(w, r)
				return
			}
			projectID := tail[:slash]

			// Must immediately be “…/control-plane/”
			rest := tail[slash:] // starts with '/'
			if !strings.HasPrefix(rest, controlPlaneSeg) {
				// e.g. CRD path “…/apis/.../projects/foo” → pass through
				next.ServeHTTP(w, r)
				return
			}

			// ── 2. Ensure workspace only for valid control-plane paths ────
			ws, err := table.Ensure(r.Context(), projectID)
			if err != nil {
				http.Error(w, "bootstrap workspace: "+err.Error(),
					http.StatusInternalServerError)
				return
			}

			// prefix = “…/projects/<id>/control-plane”
			prefix := projectsSeg + projectID + controlPlaneSeg

			// Rewrite URL for the workspace handler.
			fwdPath := strings.TrimPrefix(r.URL.Path[idx+len(prefix):], "/")
			if fwdPath == "" { // hits “…/control-plane” exactly
				fwdPath = "/" // workspace root → discovery handler
			} else {
				fwdPath = "/" + fwdPath
			}

			r2 := r.Clone(r.Context())
			r2.URL.Path = fwdPath

			klog.V(4).InfoS("ProjectRouter",
				"project", projectID,
				"origPath", r.URL.Path,
				"fwdPath", r2.URL.Path,
			)

			ws.Handler.ServeHTTP(w, r2)
		})
	}
}
