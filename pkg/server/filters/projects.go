// pkg/filters/project_router.go
package filters

import (
	"net/http"
	"strings"

	"go.miloapis.com/milo/pkg/request"
	"k8s.io/klog/v2"
)

// ProjectRouter rewrites
//   /projects/<id>/control-plane/<k8s-path>
// to
//   /<k8s-path>
// and stashes <id> in the context.
func ProjectRouter(next http.Handler) http.Handler {
	const (
		projectsSeg     = "/projects/"
		controlPlaneSeg = "/control-plane"
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := strings.Index(r.URL.Path, projectsSeg)
		if idx < 0 {
			// not project-scoped
			next.ServeHTTP(w, r)
			return
		}

		tail := r.URL.Path[idx+len(projectsSeg):] // "<id>/control-plane/..."
		slash := strings.IndexByte(tail, '/')
		if slash < 0 || !strings.HasPrefix(tail[slash:], controlPlaneSeg+"/") {
			// e.g. CRD paths that just contain ".../projects/..." in the middle
			next.ServeHTTP(w, r)
			return
		}

		projID := tail[:slash]

		// Drop ".../projects/<id>/control-plane"
		newPath := "/" + strings.TrimPrefix(
			r.URL.Path[idx+len(projectsSeg)+slash+len(controlPlaneSeg):], "/")

		// Clone request with project in context and fully-rewritten URL bits
		r2 := r.Clone(request.WithProject(r.Context(), projID))
		r2.URL.Path = newPath
		r2.URL.RawPath = newPath // important for request-info & long-running detection

		if r.URL.RawQuery != "" {
			r2.RequestURI = newPath + "?" + r.URL.RawQuery
		} else {
			r2.RequestURI = newPath
		}

		klog.InfoS("ProjectRouter", "project", projID, "newPath", newPath, "rawQuery", r.URL.RawQuery)
		next.ServeHTTP(w, r2)
	})
}
