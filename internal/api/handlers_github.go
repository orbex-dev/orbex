package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/orbex-dev/orbex/internal/config"
	"github.com/orbex-dev/orbex/internal/database"
	"github.com/orbex-dev/orbex/internal/models"
	"github.com/orbex-dev/orbex/internal/storage"
)

// GithubHandler handles GitHub OAuth and repo operations.
type GithubHandler struct {
	db      *database.DB
	storage *storage.Client
	cfg     *config.Config
}

// NewGithubHandler creates a new GithubHandler.
func NewGithubHandler(db *database.DB, storageClient *storage.Client, cfg *config.Config) *GithubHandler {
	return &GithubHandler{db: db, storage: storageClient, cfg: cfg}
}

// StartOAuth redirects the user to GitHub's OAuth authorization page.
func (h *GithubHandler) StartOAuth(w http.ResponseWriter, r *http.Request) {
	state := r.URL.Query().Get("redirect")
	if state == "" {
		state = "/dashboard/jobs"
	}

	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&scope=repo&state=%s",
		h.cfg.GithubClientID,
		url.QueryEscape(state),
	)

	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// OAuthCallback handles the GitHub OAuth callback, exchanges the code for a token.
func (h *GithubHandler) OAuthCallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	redirectPath := r.URL.Query().Get("state")
	if redirectPath == "" {
		redirectPath = "/dashboard/jobs"
	}

	if code == "" {
		http.Error(w, "Missing code parameter", http.StatusBadRequest)
		return
	}

	// Exchange code for access token
	tokenResp, err := http.PostForm("https://github.com/login/oauth/access_token", url.Values{
		"client_id":     {h.cfg.GithubClientID},
		"client_secret": {h.cfg.GithubClientSecret},
		"code":          {code},
	})
	if err != nil {
		http.Error(w, "Failed to exchange code", http.StatusInternalServerError)
		return
	}
	defer tokenResp.Body.Close()

	body, _ := io.ReadAll(tokenResp.Body)
	params, _ := url.ParseQuery(string(body))
	accessToken := params.Get("access_token")
	if accessToken == "" {
		http.Error(w, "Failed to get access token", http.StatusInternalServerError)
		return
	}

	// Get GitHub user info
	ghUser, err := getGithubUser(accessToken)
	if err != nil {
		http.Error(w, "Failed to get GitHub user info", http.StatusInternalServerError)
		return
	}

	// We need to get the Orbex user from the session cookie
	// For now, store the token with a placeholder that gets associated on first use
	// Check if user has a session
	cookie, err := r.Cookie("orbex_session")
	if err != nil {
		// Redirect to login with the token in a temporary cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "github_token_pending",
			Value:    accessToken + "|" + ghUser,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   300, // 5 min to complete
		})
		http.Redirect(w, r, "/login?redirect="+url.QueryEscape(redirectPath), http.StatusTemporaryRedirect)
		return
	}

	// Look up the user from session
	var userID uuid.UUID
	err = h.db.Pool.QueryRow(r.Context(), `
		SELECT user_id FROM sessions WHERE token = $1 AND expires_at > now()
	`, cookie.Value).Scan(&userID)
	if err != nil {
		http.Redirect(w, r, "/login?redirect="+url.QueryEscape(redirectPath), http.StatusTemporaryRedirect)
		return
	}

	// Store the GitHub token
	_, err = h.db.Pool.Exec(r.Context(), `
		INSERT INTO github_tokens (user_id, access_token, github_username)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id) DO UPDATE SET access_token = $2, github_username = $3
	`, userID, accessToken, ghUser)
	if err != nil {
		http.Error(w, "Failed to store GitHub token", http.StatusInternalServerError)
		return
	}

	// Redirect back to dashboard
	http.Redirect(w, r, redirectPath, http.StatusTemporaryRedirect)
}

// GetGithubStatus returns whether the current user has connected their GitHub account.
func (h *GithubHandler) GetGithubStatus(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	var token models.GithubToken
	err := h.db.Pool.QueryRow(r.Context(), `
		SELECT id, user_id, github_username, created_at
		FROM github_tokens
		WHERE user_id = $1
	`, user.ID).Scan(&token.ID, &token.UserID, &token.GithubUsername, &token.CreatedAt)

	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"connected": false,
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"connected":       true,
		"github_username": token.GithubUsername,
		"connected_at":    token.CreatedAt,
	})
}

// ListRepos returns the authenticated user's GitHub repositories.
func (h *GithubHandler) ListRepos(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())

	var accessToken string
	err := h.db.Pool.QueryRow(r.Context(), `
		SELECT access_token FROM github_tokens WHERE user_id = $1
	`, user.ID).Scan(&accessToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
			Error: "github_not_connected", Message: "GitHub account not connected",
		})
		return
	}

	// Fetch repos from GitHub API
	req, _ := http.NewRequestWithContext(r.Context(), "GET", "https://api.github.com/user/repos?per_page=100&sort=updated", nil)
	req.Header.Set("Authorization", "token "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "github_error", Message: "Failed to fetch repos",
		})
		return
	}
	defer resp.Body.Close()

	var repos []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&repos)

	// Extract only what we need
	var result []map[string]interface{}
	for _, repo := range repos {
		result = append(result, map[string]interface{}{
			"full_name":      repo["full_name"],
			"name":           repo["name"],
			"owner":          repo["owner"].(map[string]interface{})["login"],
			"default_branch": repo["default_branch"],
			"private":        repo["private"],
			"description":    repo["description"],
			"html_url":       repo["html_url"],
		})
	}
	if result == nil {
		result = []map[string]interface{}{}
	}

	writeJSON(w, http.StatusOK, result)
}

// ListBranches returns branches for a specific repo.
func (h *GithubHandler) ListBranches(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
	owner := chi.URLParam(r, "owner")
	repo := chi.URLParam(r, "repo")

	var accessToken string
	err := h.db.Pool.QueryRow(r.Context(), `
		SELECT access_token FROM github_tokens WHERE user_id = $1
	`, user.ID).Scan(&accessToken)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, models.ErrorResponse{
			Error: "github_not_connected", Message: "GitHub account not connected",
		})
		return
	}

	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches?per_page=100", owner, repo)
	req, _ := http.NewRequestWithContext(r.Context(), "GET", apiURL, nil)
	req.Header.Set("Authorization", "token "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, models.ErrorResponse{
			Error: "github_error", Message: "Failed to fetch branches",
		})
		return
	}
	defer resp.Body.Close()

	var branches []map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&branches)

	var result []string
	for _, b := range branches {
		if name, ok := b["name"].(string); ok {
			result = append(result, name)
		}
	}
	if result == nil {
		result = []string{}
	}

	writeJSON(w, http.StatusOK, result)
}

// GithubWebhook handles incoming push events from GitHub.
func (h *GithubHandler) GithubWebhook(w http.ResponseWriter, r *http.Request) {
	event := r.Header.Get("X-GitHub-Event")
	if event != "push" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var payload struct {
		Ref        string `json:"ref"`
		After      string `json:"after"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid payload", http.StatusBadRequest)
		return
	}

	// Extract branch from ref (refs/heads/main -> main)
	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	repoName := payload.Repository.FullName

	// Find matching jobs
	rows, err := h.db.Pool.Query(r.Context(), `
		SELECT j.id, j.user_id, gt.access_token
		FROM jobs j
		JOIN github_tokens gt ON gt.id = j.github_token_id
		WHERE j.source_type = 'github'
		  AND j.github_repo = $1
		  AND j.github_branch = $2
		  AND j.is_active = true
	`, repoName, branch)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var triggered int
	for rows.Next() {
		var jobID, userID uuid.UUID
		var accessToken string
		if err := rows.Scan(&jobID, &userID, &accessToken); err != nil {
			continue
		}

		// Download repo tarball from GitHub
		tarURL := fmt.Sprintf("https://api.github.com/repos/%s/tarball/%s", repoName, payload.After)
		req, _ := http.NewRequestWithContext(r.Context(), "GET", tarURL, nil)
		req.Header.Set("Authorization", "token "+accessToken)

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			continue
		}

		// Store in MinIO
		key := fmt.Sprintf("builds/%s/%s/repo.tar.gz", userID, jobID)
		h.storage.Upload(r.Context(), key, resp.Body, resp.ContentLength, "application/gzip")
		resp.Body.Close()

		// Enqueue build
		h.db.Pool.Exec(r.Context(), `
			INSERT INTO build_queue (job_id, user_id) VALUES ($1, $2)
		`, jobID, userID)

		triggered++
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"builds_triggered": triggered,
	})
}

// getGithubUser fetches the authenticated GitHub user's login name.
func getGithubUser(accessToken string) (string, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "token "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var user struct {
		Login string `json:"login"`
	}
	json.NewDecoder(resp.Body).Decode(&user)
	return user.Login, nil
}
