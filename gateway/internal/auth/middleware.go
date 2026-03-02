package auth

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
)

type contextKey string

const UserContextKey contextKey = "user"

// JWTMiddleware validates JWT from Authorization header
func JWTMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error": "missing Authorization header"}`, http.StatusUnauthorized)
				return
			}

			// Expect "Bearer <token>"
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				http.Error(w, `{"error": "invalid Authorization header format"}`, http.StatusUnauthorized)
				return
			}

			token := parts[1]

			// Validate JWT
			claims, err := ValidateJWT(token, secret)
			if err != nil {
				slog.Warn("jwt validation failed", "error", err)
				http.Error(w, `{"error": "invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			// Add user info to request context
			ctx := context.WithValue(r.Context(), UserContextKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserFromContext extracts user claims from request context
func GetUserFromContext(r *http.Request) *Claims {
	if claims, ok := r.Context().Value(UserContextKey).(*Claims); ok {
		return claims
	}
	return nil
}
