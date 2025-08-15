// pkg/server/filters/projectmembership.go
package filters

import (
	"fmt"
	"net/http"

	iamv1alpha1 "go.miloapis.com/milo/pkg/apis/iam/v1alpha1"
	resourcemanagerv1alpha1 "go.miloapis.com/milo/pkg/apis/resourcemanager/v1alpha1"
	projctx "go.miloapis.com/milo/pkg/request"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/endpoints/handlers/responsewriters"
	"k8s.io/apiserver/pkg/endpoints/request"
)

// ProjectContextAuthorizationDecorator needs to run AFTER authentication, BEFORE authorization.
// It injects {ParentAPIGroup, ParentKind, ParentName} for the current project into the user extras.
func ProjectContextAuthorizationDecorator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		ctx := req.Context()

		// Did the ProjectRouter stash a project id on this request?
		projID, ok := projctx.ProjectID(ctx)
		if !ok || projID == "" {
			// Not a project-scoped request; pass through
			next.ServeHTTP(w, req)
			return
		}

		reqUser, ok := request.UserFrom(ctx)
		if !ok {
			responsewriters.InternalError(w, req, fmt.Errorf("failed to extract user info from context"))
			return
		}
		u, ok := reqUser.(*user.DefaultInfo)
		if !ok {
			responsewriters.InternalError(w, req, fmt.Errorf("unexpected user.Info type. Expected *user.DefaultInfo, got %T", reqUser))
			return
		}

		// Project takes precedence over Organization for parent scoping
		extra := map[string][]string{
			iamv1alpha1.ParentAPIGroupExtraKey: {resourcemanagerv1alpha1.GroupVersion.Group},
			iamv1alpha1.ParentKindExtraKey:     {"Project"},
			iamv1alpha1.ParentNameExtraKey:     {projID},
		}

		req = req.WithContext(request.WithUser(ctx, userWithExtra(u, extra)))
		next.ServeHTTP(w, req)
	})
}
