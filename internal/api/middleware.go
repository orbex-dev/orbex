package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"strings"
	"time"

	"github.com/orbex-dev/orbex/internal/database"
	"github.com/orbex-dev/orbex/internal/models"
)

type contextKey string

const userContextKey contextKey = "user"

// AuthMiddleware validates authentication via API key OR session cookie.
// Priority: Bearer token (for CLI/API) → session cookie (for dashboard).
func AuthMiddleware(db *database.DB) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var user *models.User

			// Channel 1: Check Authorization header (API key)
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				key := strings.TrimPrefix(authHeader, "Bearer ")
				key = strings.TrimSpace(key)
				if key != "" {
					user = authenticateByAPIKey(r.Context(), db, key)
				}
			}

			// Channel 2: Check session cookie (dashboard)
			if user == nil {
				if cookie, err := r.Cookie("orbex_session"); err == nil && cookie.Value != "" {
					user = authenticateBySession(r.Context(), db, cookie.Value)
				}
			}

			if user == nil {
				writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
					Error:   "unauthorized",
					Message: "Missing or invalid authentication. Use Authorization: Bearer <api_key> or login via the dashboard.",
				})
				return
			}

			ctx := context.WithValue(r.Context(), userContextKey, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// authenticateByAPIKey validates a Bearer API key.
func authenticateByAPIKey(ctx context.Context, db *database.DB, key string) *models.User {
	hash := sha256.Sum256([]byte(key))
	keyHash := hex.EncodeToString(hash[:])

	var user models.User
	err := db.Pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.created_at, u.updated_at
		FROM api_keys ak
		JOIN users u ON u.id = ak.user_id
		WHERE ak.key_hash = $1
	`, keyHash).Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		return nil
	}

	// Update last_used (fire and forget)
	go func() {
		_, _ = db.Pool.Exec(context.Background(),
			"UPDATE api_keys SET last_used = now() WHERE key_hash = $1", keyHash)
	}()

	return &user
}

// authenticateBySession validates a session cookie token.
func authenticateBySession(ctx context.Context, db *database.DB, token string) *models.User {
	hash := sha256.Sum256([]byte(token))
	tokenHash := hex.EncodeToString(hash[:])

	var user models.User
	var expiresAt time.Time
	err := db.Pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.created_at, u.updated_at, s.expires_at
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1
	`, tokenHash).Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt, &expiresAt)

	if err != nil {
		return nil
	}

	// Check expiry
	if time.Now().After(expiresAt) {
		// Clean up expired session
		_, _ = db.Pool.Exec(ctx, "DELETE FROM sessions WHERE token_hash = $1", tokenHash)
		return nil
	}

	return &user
}

// UserFromContext extracts the authenticated user from the request context.
func UserFromContext(ctx context.Context) *models.User {
	user, _ := ctx.Value(userContextKey).(*models.User)
	return user
}
