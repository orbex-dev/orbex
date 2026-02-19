package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

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
