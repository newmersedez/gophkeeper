// Package middleware содержит HTTP-middleware сервера.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"gophkeeper/internal/auth"

	"github.com/google/uuid"
)

type contextKey string

const userIDKey contextKey = "userID"

// SetUserID сохраняет ID пользователя в контексте.
func SetUserID(ctx context.Context, userID uuid.UUID) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// GetUserID извлекает ID пользователя из контекста.
func GetUserID(ctx context.Context) (uuid.UUID, bool) {
	userID, ok := ctx.Value(userIDKey).(uuid.UUID)
	return userID, ok
}

// AuthMiddleware проверяет Bearer JWT.
func AuthMiddleware(authSvc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			const prefix = "Bearer "
			if !strings.HasPrefix(header, prefix) {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			token := strings.TrimSpace(header[len(prefix):])
			userID, err := authSvc.ValidateToken(token)
			if err != nil {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(SetUserID(r.Context(), userID)))
		})
	}
}
