// pkg/filters/project_router.go
package filters

import (
	"net/http"
	"strings"

	"go.miloapis.com/milo/pkg/request"
	"k8s.io/klog/v2"
)

// ProjectRouter rewrites
//
//	/projects/<id>/control-plane/<k8s-path>
//
//	to   /<k8s-path>  and stashes <id> in the context.
func ProjectRouter(next http.Handler) http.Handler {
	const (
		projectsSeg     = "/projects/"
		controlPlaneSeg = "/control-plane"
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := strings.Index(r.URL.Path, projectsSeg)
		if idx < 0 {
			next.ServeHTTP(w, r) // not project scoped
			return
		}

		tail := r.URL.Path[idx+len(projectsSeg):] // "<id>/control-plane/..."
		slash := strings.IndexByte(tail, '/')
		if slash < 0 || !strings.HasPrefix(tail[slash:], controlPlaneSeg+"/") {
			next.ServeHTTP(w, r) // CRD url like .../apis/.../projects/foo
			return
		}
		projID := tail[:slash]

		// drop ".../projects/<id>/control-plane"
		newPath := "/" + strings.TrimPrefix(
			r.URL.Path[idx+len(projectsSeg)+slash+len(controlPlaneSeg):], "/")

		r2 := r.Clone(request.WithProject(r.Context(), projID))
		r2.URL.Path = newPath
		klog.Infof("ROUTER id=%q new=%q", projID, newPath)
		next.ServeHTTP(w, r2)
	})
}
