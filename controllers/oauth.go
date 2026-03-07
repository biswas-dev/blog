package controllers

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"anshumanbiswas.com/blog/models"
)

// OAuthController handles GitHub OAuth sign-in/sign-up.
type OAuthController struct {
	DB                 *sql.DB
	UserService        *models.UserService
	SessionService     *models.SessionService
	GitHubClientID     string
	GitHubClientSecret string
	AppURL             string
	StateSecret        string
}

// HandleGitHubLogin redirects to GitHub's OAuth authorization page.
func (oc *OAuthController) HandleGitHubLogin(w http.ResponseWriter, r *http.Request) {
	// Generate random 16-byte nonce.
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	nonce := hex.EncodeToString(nonceBytes)

	// HMAC-sign the nonce.
	mac := hmac.New(sha256.New, []byte(oc.StateSecret))
	mac.Write([]byte(nonce))
	sig := hex.EncodeToString(mac.Sum(nil))
	state := nonce + "." + sig

	// Store nonce in short-lived HttpOnly cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "oauth_state",
		Value:    nonce,
		Path:     "/",
		MaxAge:   600,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Store return_to if provided (must be a relative path for safety).
	if returnTo := r.URL.Query().Get("return_to"); returnTo != "" && strings.HasPrefix(returnTo, "/") && !strings.HasPrefix(returnTo, "//") {
		http.SetCookie(w, &http.Cookie{
			Name:     "oauth_return_to",
			Value:    returnTo,
			Path:     "/",
			MaxAge:   600,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})
	}

	callbackURL := oc.AppURL + "/auth/github/callback"
	authURL := fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&redirect_uri=%s&scope=read:user,user:email&state=%s",
		url.QueryEscape(oc.GitHubClientID),
		url.QueryEscape(callbackURL),
		url.QueryEscape(state),
	)
	http.Redirect(w, r, authURL, http.StatusFound)
}

// HandleGitHubCallback handles the OAuth callback from GitHub.
func (oc *OAuthController) HandleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	// --- Validate state ---
	stateParam := r.URL.Query().Get("state")
	nonceCookie, err := r.Cookie("oauth_state")
	// Clear cookie regardless of outcome.
	http.SetCookie(w, &http.Cookie{
		Name:    "oauth_state",
		Value:   "",
		Path:    "/",
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	})

	if err != nil || stateParam == "" {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	parts := strings.SplitN(stateParam, ".", 2)
	if len(parts) != 2 {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}
	nonce, sig := parts[0], parts[1]

	// Constant-time HMAC verification.
	mac := hmac.New(sha256.New, []byte(oc.StateSecret))
	mac.Write([]byte(nonce))
	expectedSig := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) || nonce != nonceCookie.Value {
		http.Error(w, "Invalid OAuth state", http.StatusBadRequest)
		return
	}

	// --- Exchange code for access token ---
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "Missing OAuth code", http.StatusBadRequest)
		return
	}

	accessToken, err := oc.exchangeCode(r.Context(), code)
	if err != nil {
		http.Error(w, "Failed to exchange OAuth code", http.StatusInternalServerError)
		return
	}

	// --- Fetch GitHub user profile ---
	ghUser, err := oc.fetchGitHubUser(r.Context(), accessToken)
	if err != nil {
		http.Error(w, "Failed to fetch GitHub user", http.StatusInternalServerError)
		return
	}

	// --- Resolve email ---
	email := ghUser.Email
	if email == "" {
		email, err = oc.fetchPrimaryEmail(r.Context(), accessToken)
		if err != nil || email == "" {
			http.Error(w, "GitHub account has no verified email; please add one at github.com/settings/emails", http.StatusBadRequest)
			return
		}
	}
	email = strings.ToLower(email)

	// --- Find or create user ---
	user, err := oc.findOrCreateUser(email, ghUser.Login, ghUser.ID)
	if err != nil {
		http.Error(w, "Failed to sign in with GitHub", http.StatusInternalServerError)
		return
	}

	// --- Create session ---
	session, err := oc.SessionService.Create(user.UserID)
	if err != nil {
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}
	setCookie(w, CookieSession, session.Token)
	setCookie(w, CookieUserEmail, email)

	// Redirect to return_to if set, otherwise home.
	redirectTo := "/"
	if c, err := r.Cookie("oauth_return_to"); err == nil && strings.HasPrefix(c.Value, "/") {
		redirectTo = c.Value
	}
	http.SetCookie(w, &http.Cookie{
		Name:    "oauth_return_to",
		Value:   "",
		Path:    "/",
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	})
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

// exchangeCode exchanges the GitHub OAuth authorization code for an access token.
func (oc *OAuthController) exchangeCode(ctx context.Context, code string) (string, error) {
	vals := url.Values{
		"client_id":     {oc.GitHubClientID},
		"client_secret": {oc.GitHubClientSecret},
		"code":          {code},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://github.com/login/oauth/access_token",
		strings.NewReader(vals.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("github oauth error: %s", result.Error)
	}
	return result.AccessToken, nil
}

type gitHubUser struct {
	ID    string `json:"id"`
	Login string `json:"login"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UnmarshalJSON handles GitHub's id field which can be int or string.
func (u *gitHubUser) UnmarshalJSON(data []byte) error {
	type Alias gitHubUser
	var raw struct {
		ID    interface{} `json:"id"`
		Login string      `json:"login"`
		Name  string      `json:"name"`
		Email string      `json:"email"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	u.Login = raw.Login
	u.Name = raw.Name
	u.Email = raw.Email
	u.ID = fmt.Sprintf("%v", raw.ID)
	return nil
}

func (oc *OAuthController) fetchGitHubUser(ctx context.Context, token string) (*gitHubUser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var u gitHubUser
	body, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(body, &u); err != nil {
		return nil, err
	}
	return &u, nil
}

func (oc *OAuthController) fetchPrimaryEmail(ctx context.Context, token string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.github.com/user/emails", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", err
	}
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	// Fall back to first verified email.
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}
	return "", nil
}

// findOrCreateUser looks up a user by GitHub provider ID or email, creating one if needed.
func (oc *OAuthController) findOrCreateUser(email, githubLogin, githubID string) (*models.User, error) {
	// 1. Find by provider ID.
	var userID int
	err := oc.DB.QueryRow(
		`SELECT user_id FROM oauth_providers WHERE provider = 'github' AND provider_user_id = $1`,
		githubID,
	).Scan(&userID)
	if err == nil {
		// Found by provider ID — return the user.
		return oc.userByID(userID)
	}

	// 2. Find by email.
	var user models.User
	row := oc.DB.QueryRow(
		`SELECT user_id, username, email, role_id FROM Users WHERE email = $1`,
		email,
	)
	err = row.Scan(&user.UserID, &user.Username, &user.Email, &user.Role)
	if err == nil {
		// Upsert oauth_providers row so future logins use provider_id path.
		_, _ = oc.DB.Exec(
			`INSERT INTO oauth_providers (user_id, provider, provider_user_id) VALUES ($1, 'github', $2)
             ON CONFLICT (provider, provider_user_id) DO NOTHING`,
			user.UserID, githubID,
		)
		return &user, nil
	}

	// 3. New user — create with commenter role.
	newUser, err := oc.UserService.CreateOAuthUser(email, githubLogin, models.RoleCommenter)
	if err != nil {
		return nil, fmt.Errorf("create oauth user: %w", err)
	}
	_, err = oc.DB.Exec(
		`INSERT INTO oauth_providers (user_id, provider, provider_user_id) VALUES ($1, 'github', $2)`,
		newUser.UserID, githubID,
	)
	if err != nil {
		return nil, fmt.Errorf("insert oauth_providers: %w", err)
	}
	return newUser, nil
}

func (oc *OAuthController) userByID(userID int) (*models.User, error) {
	var u models.User
	err := oc.DB.QueryRow(
		`SELECT user_id, username, email, role_id FROM Users WHERE user_id = $1`, userID,
	).Scan(&u.UserID, &u.Username, &u.Email, &u.Role)
	if err != nil {
		return nil, fmt.Errorf("user by id: %w", err)
	}
	return &u, nil
}
