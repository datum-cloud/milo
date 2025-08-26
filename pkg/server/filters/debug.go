// pkg/filters/debug_reqinfo.go
package filters

import (
	"net/http"

	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/server"
	"k8s.io/klog/v2"
)

func DebugReqInfo(c *server.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ri, _ := apirequest.RequestInfoFrom(r.Context())
		q := r.URL.Query()

		// Only call LongRunningFunc if ri is non-nil
		isLR := false
		if ri != nil {
			isLR = c.LongRunningFunc(r, ri)
		}

		klog.InfoS("REQ",
			"path", r.URL.Path,
			"rawPath", r.URL.RawPath,
			"reqURI", r.RequestURI,
			"watchQP", q.Get("watch"),
			"verb", func() string {
				if ri != nil {
					return ri.Verb
				}
				return "<nil>"
			}(),
			"apiPrefix", func() string {
				if ri != nil {
					return ri.APIPrefix
				}
				return "<nil>"
			}(),
			"resource", func() string {
				if ri != nil {
					return ri.Resource
				}
				return "<nil>"
			}(),
			"isResourceRequest", func() bool {
				if ri != nil {
					return ri.IsResourceRequest
				}
				return false
			}(),
			"isLongRunning", isLR,
		)
		next.ServeHTTP(w, r)
	})
}
