package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/orbex-dev/orbex/internal/database"
	"github.com/orbex-dev/orbex/internal/models"
)

type contextKey string

const userContextKey contextKey = "user"

// AuthMiddleware validates API key authentication.
func AuthMiddleware(db *database.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract API key from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
					Error:   "unauthorized",
					Message: "Missing Authorization header. Use: Authorization: Bearer <api_key>",
				})
				return
			}

			// Support "Bearer <key>" format
			key := strings.TrimPrefix(authHeader, "Bearer ")
			key = strings.TrimSpace(key)
			if key == "" {
				writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
					Error:   "unauthorized",
					Message: "Invalid API key format",
				})
				return
			}

			// Hash the key and look up in database
			hash := sha256.Sum256([]byte(key))
			keyHash := hex.EncodeToString(hash[:])

			var user models.User
			err := db.Pool.QueryRow(r.Context(), `
				SELECT u.id, u.email, u.created_at, u.updated_at
				FROM api_keys ak
				JOIN users u ON u.id = ak.user_id
				WHERE ak.key_hash = $1
			`, keyHash).Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)

			if err != nil {
				writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
					Error:   "unauthorized",
					Message: "Invalid API key",
				})
				return
			}

			// Update last_used timestamp (fire and forget)
			go func() {
				_, _ = db.Pool.Exec(context.Background(),
					"UPDATE api_keys SET last_used = now() WHERE key_hash = $1", keyHash,
				)
			}()

			// Attach user to context
			ctx := context.WithValue(r.Context(), userContextKey, &user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserFromContext extracts the authenticated user from the request context.
func UserFromContext(ctx context.Context) *models.User {
	user, _ := ctx.Value(userContextKey).(*models.User)
	return user
}
