package jwt

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"go.datum.net/iam/internal/grpc/auth"
	"go.datum.net/iam/internal/grpc/errors"
	"google.golang.org/genproto/googleapis/api/serviceconfig"
	"google.golang.org/grpc/metadata"
)

func SubjectExtractor(auth *serviceconfig.Authentication) (auth.SubjectExtractor, error) {
	// Create a set of token validators that should be checked against the JWT
	// token that's retrieved from the request context.
	cache := jwk.NewCache(context.Background(), jwk.WithRefreshWindow(time.Hour))

	for _, provider := range auth.Providers {
		if err := cache.Register(provider.JwksUri); err != nil {
			return nil, fmt.Errorf("failed to register provider JWK URL: %w", err)
		}
	}

	return func(ctx context.Context) (string, error) {
		// Extract the bearer token from the context
		token, err := extractBearerToken(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "failed to extract bearer token from JWT context", slog.String("error", err.Error()))
			return "", errors.Unauthenticated().Err()
		}

		for i, provider := range auth.Providers {
			keySet, err := cache.Get(ctx, provider.JwksUri)
			if err != nil {
				return "", err
			}

			if claims, err := jwt.Parse([]byte(token), jwt.WithKeySet(keySet)); err == nil {
				return claims.Subject(), nil
			} else {
				slog.ErrorContext(
					ctx,
					"failed to verify JWT bearer token against authentication provider",
					slog.String("error", err.Error()),
					slog.String("authentication_provider", auth.Providers[i].JwksUri),
				)
			}
		}

		slog.ErrorContext(ctx, "failed to validate JWT against all authentication providers")

		return "", errors.Unauthenticated().Err()
	}, nil
}

// Function to extract the bearer token from the context
func extractBearerToken(ctx context.Context) (string, error) {
	// Get the metadata from the context
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return "", fmt.Errorf("no metadata found in context")
	}

	// Get the authorization header values
	authHeaders, exists := md["authorization"]
	if !exists || len(authHeaders) == 0 {
		return "", fmt.Errorf("authorization header not found")
	}

	// Authorization header is usually in the format "Bearer <token>"
	authHeader := authHeaders[0]
	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", fmt.Errorf("invalid authorization header format")
	}

	return parts[1], nil // Return the token part
}
