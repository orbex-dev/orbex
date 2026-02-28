package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/orbex-dev/orbex/internal/database"
	"github.com/orbex-dev/orbex/internal/models"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler handles user registration and API key management.
type AuthHandler struct {
	db *database.DB
}

// NewAuthHandler creates a new AuthHandler.
func NewAuthHandler(db *database.DB) *AuthHandler {
	return &AuthHandler{db: db}
}

// Register creates a new user account.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid JSON body",
		})
		return
	}

	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "validation_error", Message: "Email and password are required",
		})
		return
	}

	if len(req.Password) < 8 {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "validation_error", Message: "Password must be at least 8 characters",
		})
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to process registration",
		})
		return
	}

	// Create user
	var user models.User
	err = h.db.Pool.QueryRow(r.Context(), `
		INSERT INTO users (email, password)
		VALUES ($1, $2)
		RETURNING id, email, created_at, updated_at
	`, req.Email, string(hashedPassword)).Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)

	if err != nil {
		// Check for duplicate email
		writeJSON(w, http.StatusConflict, models.ErrorResponse{
			Error: "conflict", Message: "Email already registered",
		})
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// CreateAPIKey generates a new API key for the authenticated user.
func (h *AuthHandler) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
		})
		return
	}

	// Parse optional name
	var body struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	if body.Name == "" {
		body.Name = "default"
	}

	// Generate a random API key
	rawKey, keyHash, prefix, err := generateAPIKey()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to generate API key",
		})
		return
	}

	// Store in database
	var apiKey models.APIKeyResponse
	err = h.db.Pool.QueryRow(r.Context(), `
		INSERT INTO api_keys (user_id, name, key_hash, prefix)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, prefix, created_at
	`, user.ID, body.Name, keyHash, prefix).Scan(
		&apiKey.ID, &apiKey.Name, &apiKey.Prefix, &apiKey.CreatedAt,
	)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to create API key",
		})
		return
	}

	// Return the full key (only shown once!)
	apiKey.Key = rawKey
	writeJSON(w, http.StatusCreated, apiKey)
}

// Login validates credentials and creates a session with an httpOnly cookie.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid JSON body",
		})
		return
	}

	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "validation_error", Message: "Email and password are required",
		})
		return
	}

	// Verify credentials
	var userID uuid.UUID
	var hashedPassword string
	err := h.db.Pool.QueryRow(r.Context(), `
		SELECT id, password FROM users WHERE email = $1
	`, req.Email).Scan(&userID, &hashedPassword)

	if err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized", Message: "Invalid email or password",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized", Message: "Invalid email or password",
		})
		return
	}

	// Generate session token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to create session",
		})
		return
	}
	rawToken := hex.EncodeToString(tokenBytes)
	tokenHash := sha256.Sum256([]byte(rawToken))
	tokenHashStr := hex.EncodeToString(tokenHash[:])

	// Store session (7 day expiry)
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	_, err = h.db.Pool.Exec(r.Context(), `
		INSERT INTO sessions (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, tokenHashStr, expiresAt)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to create session",
		})
		return
	}

	// Set httpOnly cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "orbex_session",
		Value:    rawToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   7 * 24 * 60 * 60, // 7 days
	})

	// Return user info
	var user models.User
	_ = h.db.Pool.QueryRow(r.Context(), `
		SELECT id, email, created_at, updated_at FROM users WHERE id = $1
	`, userID).Scan(&user.ID, &user.Email, &user.CreatedAt, &user.UpdatedAt)

	writeJSON(w, http.StatusOK, user)
}

// Logout deletes the session and clears the cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("orbex_session")
	if err == nil && cookie.Value != "" {
		tokenHash := sha256.Sum256([]byte(cookie.Value))
		tokenHashStr := hex.EncodeToString(tokenHash[:])
		_, _ = h.db.Pool.Exec(r.Context(),
			"DELETE FROM sessions WHERE token_hash = $1", tokenHashStr)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "orbex_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})

	writeJSON(w, http.StatusOK, map[string]string{"status": "logged_out"})
}

// GetMe returns the currently authenticated user.
func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized", Message: "Not authenticated",
		})
		return
	}
	writeJSON(w, http.StatusOK, user)
}

// GenerateBootstrapKey creates a first API key for a newly registered user.
// This is called during registration flow before the user has any API keys.
func (h *AuthHandler) GenerateBootstrapKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid_request", Message: "Invalid JSON body",
		})
		return
	}

	// Verify credentials
	var userID uuid.UUID
	var hashedPassword string
	err := h.db.Pool.QueryRow(r.Context(), `
		SELECT id, password FROM users WHERE email = $1
	`, req.Email).Scan(&userID, &hashedPassword)

	if err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized", Message: "Invalid email or password",
		})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(req.Password)); err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized", Message: "Invalid email or password",
		})
		return
	}

	// Generate API key
	rawKey, keyHash, prefix, err := generateAPIKey()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to generate API key",
		})
		return
	}

	var apiKey models.APIKeyResponse
	err = h.db.Pool.QueryRow(r.Context(), `
		INSERT INTO api_keys (user_id, name, key_hash, prefix)
		VALUES ($1, 'default', $2, $3)
		RETURNING id, name, prefix, created_at
	`, userID, keyHash, prefix).Scan(
		&apiKey.ID, &apiKey.Name, &apiKey.Prefix, &apiKey.CreatedAt,
	)

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "internal_error", Message: "Failed to create API key",
		})
		return
	}

	apiKey.Key = rawKey
	writeJSON(w, http.StatusCreated, apiKey)
}

// ChangePassword updates the authenticated user's password.
func (h *AuthHandler) ChangePassword(w http.ResponseWriter, r *http.Request) {
	user, _ := r.Context().Value(userContextKey).(*models.User)
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "unauthorized"})
		return
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "invalid request"})
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "both current and new password required"})
		return
	}
	if len(req.NewPassword) < 8 {
		writeJSON(w, http.StatusBadRequest, models.ErrorResponse{Error: "new password must be at least 8 characters"})
		return
	}

	// Verify current password
	var storedHash string
	err := h.db.Pool.QueryRow(r.Context(), "SELECT password FROM users WHERE id = $1", user.ID).Scan(&storedHash)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to verify password"})
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(storedHash), []byte(req.CurrentPassword)); err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{Error: "current password is incorrect"})
		return
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to hash password"})
		return
	}

	// Update
	_, err = h.db.Pool.Exec(r.Context(), "UPDATE users SET password = $1, updated_at = now() WHERE id = $2", string(newHash), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{Error: "failed to update password"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "password_updated"})
}

// generateAPIKey creates a random API key with prefix "obx_".
func generateAPIKey() (rawKey, keyHash, prefix string, err error) {
	// Generate 32 random bytes
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generating random bytes: %w", err)
	}

	rawKey = "obx_" + hex.EncodeToString(b)
	prefix = rawKey[:12] // "obx_" + first 8 hex chars

	hash := sha256.Sum256([]byte(rawKey))
	keyHash = hex.EncodeToString(hash[:])

	return rawKey, keyHash, prefix, nil
}
