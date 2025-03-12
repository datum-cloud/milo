package recovery

import (
	"fmt"
	"log/slog"

	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
	"go.datum.net/iam/internal/grpc/errors"
	"google.golang.org/grpc"
)

func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return recovery.UnaryServerInterceptor(
		recovery.WithRecoveryHandler(func(p any) (err error) {
			slog.Warn("request failed with panic", slog.String("stacktrace", fmt.Sprintf("%v", p)))
			return errors.Internal().Err()
		}),
	)
}
