package errors

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

func InternalErrorsInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		resp, err := handler(ctx, req)
		if err == nil {
			return resp, nil
		}

		grpcErr, ok := status.FromError(err)
		if ok {
			return nil, grpcErr.Err()
		}
		logger.ErrorContext(ctx, "encountered internal error while serving gRPC request", slog.String("internal_error", err.Error()))

		return nil, Internal().Err()
	}
}
