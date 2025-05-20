package jwt

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jwt"
	"go.datum.net/iam/internal/grpc/errors"
	"go.datum.net/iam/internal/subject"
	"google.golang.org/genproto/googleapis/api/serviceconfig"
	"google.golang.org/grpc/metadata"
)

func SubjectExtractor(auth *serviceconfig.Authentication) (subject.Extractor, error) {
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

			authenticationProvider := auth.Providers[i].JwksUri

			if claims, err := jwt.Parse([]byte(token), jwt.WithKeySet(keySet)); err == nil {
				claimsMap, err := claims.AsMap(ctx)
				if err != nil {
					slog.ErrorContext(ctx, "failed to convert claims to map", slog.String("error", err.Error()))
					return "", err
				}

				claimsString := fmt.Sprint(claimsMap)
				slog.InfoContext(
					ctx,
					"bearer JWT token verified against authentication provider",
					slog.String("claims", claimsString),
					slog.String("authentication_provider", authenticationProvider),
				)

				email, ok := claimsMap["email"].(string)
				if !ok {
					slog.ErrorContext(ctx, "email claim not found or invalid in JWT claims")
					return "", errors.Unauthenticated().Err()
				}

				// TODO: Resolve subject type to support machine accounts and groups
				return fmt.Sprintf("user:%s", email), nil
			} else {
				slog.ErrorContext(
					ctx,
					"failed to verify JWT bearer token against authentication provider",
					slog.String("error", err.Error()),
					slog.String("authentication_provider", authenticationProvider),
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

	tokenPart := strings.Split(parts[1], "Bearer")[1]
	return tokenPart, nil // Return the token part
}
