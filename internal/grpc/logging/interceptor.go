package logging

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
)

func UnaryServerInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		logger.InfoContext(ctx, "gRPC request recieved", slog.String("method", info.FullMethod), slog.Any("request", req.(proto.Message)))
		resp, err := handler(ctx, req)
		if err != nil {
			logger.ErrorContext(ctx, "request failed", slog.String("method", info.FullMethod), slog.Any("error", status.Convert(err).Proto()))
		} else {
			logger.InfoContext(ctx, "gRPC response received", slog.String("method", info.FullMethod), slog.Any("response", resp.(proto.Message)))
		}
		return resp, err
	}
}

func UnaryClientInterceptor(logger *slog.Logger) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		logger.InfoContext(ctx, method, slog.Any("request", req.(proto.Message)))
		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			logger.ErrorContext(ctx, "request failed", slog.String("method", method), slog.Any("error", status.Convert(err).Proto()))
		} else {
			logger.InfoContext(ctx, "gRPC response received", slog.String("method", method), slog.Any("response", reply.(proto.Message)))
		}
		return err
	}
}
