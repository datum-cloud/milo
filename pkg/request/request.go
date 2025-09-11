// pkg/request/context.go
package request

import "context"

type ctxKey string

const projectKey ctxKey = "project-id"

func WithProject(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, projectKey, id)
}

func ProjectID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(projectKey).(string)
	return id, ok
}
