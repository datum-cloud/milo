// pkg/filters/project_router.go
package filters

import (
	"net/http"
	"strings"

	reqinfo "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/klog/v2"

	"go.miloapis.com/milo/pkg/request"
)

func ProjectRouterWithRequestInfo(next http.Handler, rir reqinfo.RequestInfoResolver) http.Handler {
	const (
		projectsSeg     = "/projects/"
		controlPlaneSeg = "/control-plane"
	)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := strings.Index(r.URL.Path, projectsSeg)
		if idx < 0 {
			next.ServeHTTP(w, r)
			return
		}

		tail := r.URL.Path[idx+len(projectsSeg):] // "<id>/control-plane/..."
		slash := strings.IndexByte(tail, '/')
		if slash < 0 || !strings.HasPrefix(tail[slash:], controlPlaneSeg+"/") {
			next.ServeHTTP(w, r)
			return
		}

		projID := tail[:slash]

		// Drop ".../projects/<id>/control-plane"
		newPath := "/" + strings.TrimPrefix(
			r.URL.Path[idx+len(projectsSeg)+slash+len(controlPlaneSeg):], "/")

		// Clone request, stash project, and rewrite URL bits
		r2 := r.Clone(request.WithProject(r.Context(), projID))
		r2.URL.Path = newPath
		r2.URL.RawPath = newPath
		if r.URL.RawQuery != "" {
			r2.RequestURI = newPath + "?" + r.URL.RawQuery
		} else {
			r2.RequestURI = newPath
		}

		// 🔁 Recompute RequestInfo on the rewritten request
		ri, err := rir.NewRequestInfo(r2)
		if err != nil {
			klog.ErrorS(err, "Failed to create RequestInfo for project router")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		r2 = r2.WithContext(reqinfo.WithRequestInfo(r2.Context(), ri))

		klog.InfoS("ProjectRouter",
			"project", projID,
			"newPath", newPath,
			"ns", ri.Namespace,
			"resource", ri.Resource,
			"subresource", ri.Subresource,
			"verb", ri.Verb,
			"isResourceRequest", ri.IsResourceRequest,
		)

		next.ServeHTTP(w, r2)
	})
}
