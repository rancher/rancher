package githubapp

import (
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	authCodeLifetime    = 5 * time.Minute
	accessTokenLifetime = 1 * time.Hour
	bearerPrefix        = "Bearer "
	tokenPrefix         = "token "
)

type fakeClientDetails struct {
	Secret       string
	RedirectURIs []string
}

type fakeAuthCode struct {
	ClientID    string
	RedirectURI string
	UserID      string
	Expiry      time.Time
}

type fakeAccessToken struct {
	ClientID string
	UserID   string
	Expiry   time.Time
}

type fakeInstallationToken struct {
	InstallationID int64
	Token          string
	Expiry         time.Time
}

type fakeTeam struct {
	ID      int    `json:"id"`
	URL     string `json:"url"`
	HTMLURL string `json:"html_url"`
	Name    string `json:"name"`
	Slug    string `json:"slug"`
}

type fakeOrganization struct {
	Login     string `json:"login"`
	ID        int    `json:"id"`
	Type      string `json:"type"`
	AvatarURL string `json:"avatar_url"`
	Name      string `json:"name"`

	teams map[string]fakeTeam
}

type fakeGitHubServer struct {
	*http.ServeMux
	t *testing.T
	// Registered clients: clientID -> clientSecret, redirectURIs
	registeredClients map[string]fakeClientDetails
	// Stored authorization codes: code -> {clientID, redirectURI, userID, expiry}
	authCodes map[string]fakeAuthCode
	// Stored access tokens: token -> {clientID, userID, expiry}
	accessTokens map[string]fakeAccessToken

	// Mapping from AppID to PrivateKey in PEM format
	privateKeys map[string][]byte

	// App Installation tokens
	installationTokens map[string]fakeInstallationToken

	organizations map[string]fakeOrganization

	// Simple user data in generic maps
	users []map[string]any
}

func withTestCode(clientID, code, redirectURI, userID string) func(*fakeGitHubServer) {
	return func(s *fakeGitHubServer) {
		s.authCodes[code] = fakeAuthCode{
			ClientID:    clientID,
			RedirectURI: redirectURI,
			UserID:      userID,
			Expiry:      time.Now().Add(authCodeLifetime),
		}
	}
}

func withPrivateKey(clientID string, pemData []byte) func(*fakeGitHubServer) {
	return func(s *fakeGitHubServer) {
		s.privateKeys[clientID] = pemData
	}
}

// This is a fake GitHub server - its primary use is validating the various
// tokens that we need to send.
func newFakeGitHubServer(t *testing.T, opts ...func(*fakeGitHubServer)) *fakeGitHubServer {
	mux := http.NewServeMux()

	srv := &fakeGitHubServer{
		ServeMux: mux,
		registeredClients: map[string]fakeClientDetails{
			"test_client_id": {
				Secret: "test_client_secret",
				RedirectURIs: []string{
					"http://localhost:3000/callback",
					"http://127.0.0.1:3000/callback"},
			},
		},
		privateKeys:        map[string][]byte{},
		authCodes:          map[string]fakeAuthCode{},
		accessTokens:       map[string]fakeAccessToken{},
		installationTokens: map[string]fakeInstallationToken{},
		t:                  t,
		users: []map[string]any{
			{
				"login":               "octocat",
				"name":                "monalisa octocat",
				"id":                  1234,
				"node_id":             "MDQ6VXNlcjE=",
				"avatar_url":          "https://github.com/images/error/octocat_happy.gif",
				"url":                 "https://api.github.com/users/octocat",
				"html_url":            "https://github.com/octocat",
				"followers_url":       "https://api.github.com/users/octocat/followers",
				"following_url":       "https://api.github.com/users/octocat/following{/other_user}",
				"gists_url":           "https://api.github.com/users/octocat/gists{/gist_id}",
				"starred_url":         "https://api.github.com/users/octocat/starred{/owner}{/repo}",
				"subscriptions_url":   "https://api.github.com/users/octocat/subscriptions",
				"organizations_url":   "https://api.github.com/users/octocat/orgs",
				"repos_url":           "https://api.github.com/users/octocat/repos",
				"events_url":          "https://api.github.com/users/octocat/events{/privacy}",
				"received_events_url": "https://api.github.com/users/octocat/received_events",
				"type":                "User",
				"site_admin":          false,
			},
			{
				"login":               "example",
				"name":                "example user",
				"id":                  2,
				"avatar_url":          "https://github.com/images/error/example_happy.gif",
				"url":                 "https://api.github.com/users/example",
				"html_url":            "https://github.com/example",
				"followers_url":       "https://api.github.com/users/example/followers",
				"following_url":       "https://api.github.com/users/example/following{/other_user}",
				"gists_url":           "https://api.github.com/users/example/gists{/gist_id}",
				"starred_url":         "https://api.github.com/users/example/starred{/owner}{/repo}",
				"subscriptions_url":   "https://api.github.com/users/example/subscriptions",
				"organizations_url":   "https://api.github.com/users/example/orgs",
				"repos_url":           "https://api.github.com/users/example/repos",
				"events_url":          "https://api.github.com/users/example/events{/privacy}",
				"received_events_url": "https://api.github.com/users/example/received_events",
				"type":                "User",
				"site_admin":          false,
			},
		},
		organizations: map[string]fakeOrganization{
			"1": {
				Login:     "example-org-1",
				ID:        1,
				Type:      "Organization",
				AvatarURL: "https://example.com/example-org-1-avatar.jpg",
				Name:      "Example Org 1",
				teams: map[string]fakeTeam{
					"1215": {
						ID:      1215,
						URL:     "https://api.github.com/teams/1215",
						HTMLURL: "https://github.com/orgs/example-org-1/dev-team",
						Name:    "Dev Team",
						Slug:    "dev-team",
					},
					"1217": {
						ID:      1217,
						URL:     "https://api.github.com/teams/1217",
						HTMLURL: "https://github.com/orgs/example-org-1/test-team",
						Name:    "Test Team",
						Slug:    "test-team",
					},
				},
			},
			"2": {
				Login:     "example-org-2",
				ID:        2,
				Type:      "Organization",
				AvatarURL: "https://example.com/example-org-2-avatar.jpg",
				Name:      "Example Org 2",
				teams: map[string]fakeTeam{
					"1216": {
						ID:      1216,
						URL:     "https://api.github.com/teams/1216",
						HTMLURL: "https://github.com/orgs/example-org-2/dev-team",
						Name:    "Dev Team",
						Slug:    "dev-team",
					},
				},
			},
		},
	}
	for _, opt := range opts {
		opt(srv)
	}

	srv.HandleFunc("POST /authorize", srv.authorizeHandler)
	srv.HandleFunc("POST /login/oauth/access_token", srv.tokenHandler)
	srv.HandleFunc("GET /userinfo", srv.userinfoHandler)
	srv.HandleFunc("GET /api/v3/user", srv.userHandler)
	srv.HandleFunc("GET /api/v3/app/installations", srv.installationsHandler)
	srv.HandleFunc("GET /api/v3/app/installations/{installationID}", srv.installationHandler)
	srv.HandleFunc("POST /api/v3/app/installations/{installationID}/access_tokens", srv.installationTokenHandler)
	srv.HandleFunc("GET /api/v3/organizations/{organizationID}", srv.organizationHandler)
	srv.HandleFunc("GET /api/v3/orgs/{organizationSlug}/teams", srv.organizationTeamsHandler)
	srv.HandleFunc("GET /api/v3/organizations/{organizationID}/team/{teamID}/members", srv.organizationTeamMembersHandler)
	srv.HandleFunc("GET /api/v3/search/users", srv.userSearchHandler)
	return srv
}

// authorizeHandler simulates the user consent and redirects back to the client.
// Expected query parameters: response_type, client_id, redirect_uri, scope, state
func (s *fakeGitHubServer) authorizeHandler(w http.ResponseWriter, r *http.Request) {
	s.t.Logf("Received request for %s", r.URL)
	query := r.URL.Query()

	responseType := query.Get("response_type")
	clientID := query.Get("client_id")
	redirectURI := query.Get("redirect_uri")
	state := query.Get("state")

	if responseType != "code" {
		http.Error(w, "Unsupported response_type. Only 'code' is supported.", http.StatusBadRequest)
		return
	}
	if clientID == "" {
		http.Error(w, "Missing client_id", http.StatusBadRequest)
		return
	}
	if redirectURI == "" {
		http.Error(w, "Missing redirect_uri", http.StatusBadRequest)
		return
	}
	if !s.isValidRedirectURI(clientID, redirectURI) {
		http.Error(w, "Invalid redirect_uri for the provided client_id", http.StatusBadRequest)
		return
	}

	userID := "fake_user_123"
	// Generate a unique authorization code
	authCode := uuid.New().String()
	s.authCodes[authCode] = fakeAuthCode{
		ClientID:    clientID,
		RedirectURI: redirectURI,
		UserID:      userID,
		Expiry:      time.Now().Add(authCodeLifetime),
	}
	s.t.Logf("Generated auth code: %s for client %s, user %s", authCode, clientID, userID)

	// Build the redirect URL with the authorization code and state
	redirectURL, err := url.Parse(redirectURI)
	if err != nil {
		s.t.Logf("Error parsing redirect URI: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	params := redirectURL.Query()
	params.Add("code", authCode)
	if state != "" {
		params.Add("state", state)
	}
	redirectURL.RawQuery = params.Encode()

	s.t.Logf("Redirecting to: %s", redirectURL.String())
	http.Redirect(w, r, redirectURL.String(), http.StatusFound)
}

// tokenHandler exchanges an authorization code for an access token.
// Expected form parameters: client_id, client_secret, code, redirect_uri
// https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#2-users-are-redirected-back-to-your-site-by-github
func (s *fakeGitHubServer) tokenHandler(w http.ResponseWriter, r *http.Request) {
	s.t.Logf("Received request for %s", r.URL)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	clientID := r.Form.Get("client_id")
	clientSecret := r.Form.Get("client_secret")
	code := r.Form.Get("code")

	registeredClient, ok := s.registeredClients[clientID]
	if !ok || registeredClient.Secret != clientSecret {
		s.t.Logf("Invalid client credentials: %s", clientID)
		http.Error(w, `{"error": "invalid_client"}`, http.StatusUnauthorized)
		return
	}

	authCodeData, ok := s.authCodes[code]
	if !ok || authCodeData.Expiry.Before(time.Now()) || authCodeData.ClientID != clientID {
		s.t.Logf("Invalid or expired authorization code for client %s", clientID)
		http.Error(w, `{"error": "invalid_grant"}`, http.StatusBadRequest)
		return
	}

	// Invalidate the used authorization code (one-time use)
	delete(s.authCodes, code)
	s.t.Logf("Auth code %s consumed.", code)

	// Generate a unique access token
	accessToken := uuid.New().String()
	s.accessTokens[accessToken] = fakeAccessToken{
		ClientID: clientID,
		UserID:   authCodeData.UserID,
		Expiry:   time.Now().Add(accessTokenLifetime),
	}
	s.t.Logf("Generated access token: %s for client %s, user %s", accessToken, clientID, authCodeData.UserID)

	// Respond with the access token
	marshalJSON(s.t, w, map[string]any{
		"access_token": accessToken,
		"token_type":   "Bearer",
		"expires_in":   int(accessTokenLifetime.Seconds()),
		// A real OAuth server might also return refresh_token, scope, etc.
	})
}

// userinfoHandler simulates a protected resource endpoint.
// Requires an Authorization: Bearer <access_token> header.
func (s *fakeGitHubServer) userinfoHandler(w http.ResponseWriter, r *http.Request) {
	s.t.Logf("Received request for %s", r.URL)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error": "unauthorized", "message": "Missing Authorization header"}`, http.StatusUnauthorized)
		return
	}

	// Extract token
	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		http.Error(w, `{"error": "invalid_token", "message": "Invalid Authorization header format"}`, http.StatusUnauthorized)
		return
	}
	accessToken := authHeader[len(bearerPrefix):]

	// Validate access token
	tokenData, ok := s.accessTokens[accessToken]
	if !ok || tokenData.Expiry.Before(time.Now()) {
		s.t.Logf("Invalid or expired access token: %s", accessToken)
		http.Error(w, `{"error": "invalid_token", "message": "Access token is invalid or expired"}`, http.StatusUnauthorized)
		return
	}

	s.t.Logf("Access token %s is valid for user %s", accessToken, tokenData.UserID)

	marshalJSON(s.t, w, map[string]any{
		"sub":      tokenData.UserID, // Subject (user ID)
		"name":     "Fake User",
		"email":    fmt.Sprintf("%s@example.com", tokenData.UserID),
		"client":   tokenData.ClientID,
		"accessed": time.Now().Format(time.RFC3339),
	})
}

// userHandler fakes the GitHub user info API
// Requires an Authorization: token <oauth_token> header.
// This should probably be updated to use the Bearer token convention https://docs.github.com/en/apps/oauth-apps/building-oauth-apps/authorizing-oauth-apps#3-use-the-access-token-to-access-the-api
// https://docs.github.com/en/enterprise-server@3.13/rest/users/users?apiVersion=2022-11-28#get-the-authenticated-user
func (s *fakeGitHubServer) userHandler(w http.ResponseWriter, r *http.Request) {
	s.t.Logf("Received request for %s", r.URL)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error": "unauthorized", "message": "Missing Authorization header"}`, http.StatusUnauthorized)
		return
	}

	if len(authHeader) < len(tokenPrefix) || authHeader[:len(tokenPrefix)] != tokenPrefix {
		http.Error(w, `{"error": "invalid_token", "message": "Invalid Authorization header format"}`, http.StatusUnauthorized)
		return
	}
	accessToken := authHeader[len(tokenPrefix):]

	tokenData, ok := s.accessTokens[accessToken]
	if !ok || tokenData.Expiry.Before(time.Now()) {
		s.t.Logf("Invalid or expired access token: %s", accessToken)
		http.Error(w, `{"error": "invalid_token", "message": "Access token is invalid or expired"}`, http.StatusUnauthorized)
		return
	}

	s.t.Logf("Access token %s is valid for user %s", accessToken, tokenData.UserID)

	marshalJSON(s.t, w, map[string]any{
		"login":                     "octocat",
		"id":                        1234,
		"node_id":                   "MDQ6VXNlcjE=",
		"avatar_url":                "https://github.com/images/error/octocat_happy.gif",
		"gravatar_id":               "",
		"url":                       "https://HOSTNAME/users/octocat",
		"html_url":                  "https://github.com/octocat",
		"followers_url":             "https://HOSTNAME/users/octocat/followers",
		"following_url":             "https://HOSTNAME/users/octocat/following{/other_user}",
		"gists_url":                 "https://HOSTNAME/users/octocat/gists{/gist_id}",
		"starred_url":               "https://HOSTNAME/users/octocat/starred{/owner}{/repo}",
		"subscriptions_url":         "https://HOSTNAME/users/octocat/subscriptions",
		"organizations_url":         "https://HOSTNAME/users/octocat/orgs",
		"repos_url":                 "https://HOSTNAME/users/octocat/repos",
		"events_url":                "https://HOSTNAME/users/octocat/events{/privacy}",
		"received_events_url":       "https://HOSTNAME/users/octocat/received_events",
		"type":                      "User",
		"site_admin":                false,
		"name":                      "monalisa octocat",
		"company":                   "GitHub",
		"blog":                      "https://github.com/blog",
		"location":                  "San Francisco",
		"email":                     "octocat@github.com",
		"hireable":                  false,
		"bio":                       "There once was...",
		"public_repos":              2,
		"public_gists":              1,
		"followers":                 20,
		"following":                 0,
		"created_at":                "2008-01-14T04:33:35Z",
		"updated_at":                "2008-01-14T04:33:35Z",
		"private_gists":             81,
		"total_private_repos":       100,
		"owned_private_repos":       100,
		"disk_usage":                10000,
		"collaborators":             8,
		"two_factor_authentication": true,
		"plan": map[string]any{
			"name":          "Medium",
			"space":         400,
			"private_repos": 20,
			"collaborators": 0,
		},
	})
}

// Helper function to check if a redirect URI is valid for a given client.
func (s *fakeGitHubServer) isValidRedirectURI(clientID, redirectURI string) bool {
	client, ok := s.registeredClients[clientID]
	if !ok {
		return false // Client not registered
	}
	for _, uri := range client.RedirectURIs {
		if uri == redirectURI {
			return true
		}
	}
	return false
}

// Requires Auth token with installation credentials
//
// GitHub API docs: https://docs.github.com/rest/apps/apps#get-an-installation-for-the-authenticated-app
func (s *fakeGitHubServer) installationHandler(w http.ResponseWriter, r *http.Request) {
	s.t.Logf("Received request for %s", r.URL)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error": "unauthorized", "message": "Missing Authorization header"}`, http.StatusUnauthorized)
		return
	}

	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		http.Error(w, `{"error": "invalid_token", "message": "Invalid Authorization header format"}`, http.StatusUnauthorized)
		return
	}

	accessToken := authHeader[len(bearerPrefix):]
	_, err := jwt.Parse(accessToken, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		appID, err := token.Claims.GetIssuer()
		if err != nil {
			return nil, fmt.Errorf("invalid appID in issuer: %w", err)
		}
		key := s.privateKeys[appID]
		parsed, err := jwt.ParseRSAPrivateKeyFromPEM(key)
		if err != nil {
			return nil, fmt.Errorf("error parsing key: %w", err)
		}

		return parsed.Public(), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	if err != nil {
		s.t.Logf("Invalid or expired access token: %s: %s", accessToken, err)
		http.Error(w, `{"error": "invalid_token", "message": "Access token is invalid or expired"}`, http.StatusUnauthorized)
		return
	}

	marshalJSON(s.t, w, map[string]any{
		"id": 1,
		"account": map[string]any{
			"login":               "octocat",
			"id":                  1234,
			"node_id":             "MDQ6VXNlcjE=",
			"avatar_url":          "https://github.com/images/error/octocat_happy.gif",
			"gravatar_id":         "",
			"url":                 "https://api.github.com/users/octocat",
			"html_url":            "https://github.com/octocat",
			"followers_url":       "https://api.github.com/users/octocat/followers",
			"following_url":       "https://api.github.com/users/octocat/following{/other_user}",
			"gists_url":           "https://api.github.com/users/octocat/gists{/gist_id}",
			"starred_url":         "https://api.github.com/users/octocat/starred{/owner}{/repo}",
			"subscriptions_url":   "https://api.github.com/users/octocat/subscriptions",
			"organizations_url":   "https://api.github.com/users/octocat/orgs",
			"repos_url":           "https://api.github.com/users/octocat/repos",
			"events_url":          "https://api.github.com/users/octocat/events{/privacy}",
			"received_events_url": "https://api.github.com/users/octocat/received_events",
			"type":                "User",
			"site_admin":          false,
		},
		"access_tokens_url": "https://api.github.com/app/installations/1/access_tokens",
		"repositories_url":  "https://api.github.com/installation/repositories",
		"html_url":          "https://github.com/organizations/github/settings/installations/1",
		"app_id":            1,
		"target_id":         1,
		"target_type":       "Organization",
		"permissions": map[string]any{
			"checks":   "write",
			"metadata": "read",
			"contents": "read",
		},
		"events": []string{
			"push",
			"pull_request",
		},
		"single_file_name":          "config.yaml",
		"has_multiple_single_files": true,
		"single_file_paths": []string{
			"config.yml",
			".github/issue_TEMPLATE.md",
		},
		"repository_selection": "selected",
		"created_at":           "2017-07-08T16:18:44-04:00",
		"updated_at":           "2017-07-08T16:18:44-04:00",
		"app_slug":             "github-actions",
		"suspended_at":         nil,
		"suspended_by":         nil,
	})
}

func (s *fakeGitHubServer) installationsHandler(w http.ResponseWriter, r *http.Request) {
	s.t.Logf("Received request for %s", r.URL)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error": "unauthorized", "message": "Missing Authorization header"}`, http.StatusUnauthorized)
		return
	}

	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		http.Error(w, `{"error": "invalid_token", "message": "Invalid Authorization header format"}`, http.StatusUnauthorized)
		return
	}

	accessToken := authHeader[len(bearerPrefix):]
	_, err := jwt.Parse(accessToken, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		appID, err := token.Claims.GetIssuer()
		if err != nil {
			return nil, fmt.Errorf("invalid appID in issuer: %w", err)
		}
		key := s.privateKeys[appID]
		parsed, err := jwt.ParseRSAPrivateKeyFromPEM(key)
		if err != nil {
			return nil, fmt.Errorf("error parsing key: %w", err)
		}

		return parsed.Public(), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	if err != nil {
		s.t.Logf("Invalid or expired access token: %s: %s", accessToken, err)
		http.Error(w, `{"error": "invalid_token", "message": "Access token is invalid or expired"}`, http.StatusUnauthorized)
		return
	}

	marshalJSON(s.t, w, []any{
		map[string]any{
			"id":          1,
			"target_type": "Organization",
			"target_id":   1,
		},
		map[string]any{
			"id":          2,
			"target_type": "Organization",
			"target_id":   2,
		},
	})
}

// Creates an installation token for use by an app.
// Authorization: Bearer <signed_JWT>
// Requires a signed JWT for the correct App.
// https://docs.github.com/en/rest/apps/apps#create-an-installation-access-token-for-an-app
func (s *fakeGitHubServer) installationTokenHandler(w http.ResponseWriter, r *http.Request) {
	s.t.Logf("Received request for %s", r.URL)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error": "unauthorized", "message": "Missing Authorization header"}`, http.StatusUnauthorized)
		return
	}

	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		http.Error(w, `{"error": "invalid_token", "message": "Invalid Authorization header format"}`, http.StatusUnauthorized)
		return
	}

	accessToken := authHeader[len(bearerPrefix):]
	_, err := jwt.Parse(accessToken, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		appID, err := token.Claims.GetIssuer()
		if err != nil {
			return nil, fmt.Errorf("invalid appID in issuer: %w", err)
		}
		key := s.privateKeys[appID]
		parsed, err := jwt.ParseRSAPrivateKeyFromPEM(key)
		if err != nil {
			return nil, fmt.Errorf("error parsing key: %w", err)
		}

		return parsed.Public(), nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	if err != nil {
		s.t.Logf("Invalid or expired access token: %s: %s", accessToken, err)
		http.Error(w, `{"error": "invalid_token", "message": "Access token is invalid or expired"}`, http.StatusUnauthorized)
		return
	}

	installationID := testInt64(s.t, r.PathValue("installationID"))
	authCode := uuid.New().String()
	expiry := time.Now().Add(authCodeLifetime)
	s.installationTokens[authCode] = fakeInstallationToken{
		InstallationID: installationID,
		Expiry:         expiry,
	}
	s.t.Logf("Generated code: %s for installation %v", authCode, installationID)

	marshalJSON(s.t, w, map[string]any{
		"token":      authCode,
		"expires_at": expiry,
		"permissions": map[string]any{
			"issues":   "write",
			"contents": "read",
		},
		"repository_selection": "selected",
		"repositories": []map[string]any{
			{
				"id":        1296269,
				"node_id":   "MDEwOlJlcG9zaXRvcnkxMjk2MjY5",
				"name":      "Hello-World",
				"full_name": "octocat/Hello-World",
				"owner": map[string]any{
					"login": "octocat",
					"id":    1234,
				},
			},
		},
	})
}

// organizationHandler fakes the Organization info API
// Requires an Authorization: Bearer <installation_token>.
//
// https://docs.github.com/en/rest/orgs/orgs?apiVersion=2022-11-28#get-an-organization
func (s *fakeGitHubServer) organizationHandler(w http.ResponseWriter, r *http.Request) {
	s.t.Logf("Received request for %s", r.URL)
	if !s.verifyInstallationToken(r, w) {
		return
	}

	organizationID := r.PathValue("organizationID")
	org, ok := s.organizations[organizationID]
	if !ok {
		http.NotFound(w, r)
		return
	}
	marshalJSON(s.t, w, org)
}

// organizationTeamsHandler fakes the Organization teams API
// Requires an Authorization: Bearer <installation_token>.
//
// https://docs.github.com/en/rest/teams/teams?apiVersion=2022-11-28#list-teams
func (s *fakeGitHubServer) organizationTeamsHandler(w http.ResponseWriter, r *http.Request) {
	s.t.Logf("Received request for %s", r.URL)

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error": "unauthorized", "message": "Missing Authorization header"}`, http.StatusUnauthorized)
		return
	}

	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		http.Error(w, `{"error": "invalid_token", "message": "Invalid Authorization header format"}`, http.StatusUnauthorized)
		return
	}
	accessToken := authHeader[len(bearerPrefix):]

	tokenData, ok := s.installationTokens[accessToken]
	if !ok || tokenData.Expiry.Before(time.Now()) {
		s.t.Logf("Invalid or expired access token: %s", accessToken)
		http.Error(w, `{"error": "invalid_token", "message": "Access token is invalid or expired"}`, http.StatusUnauthorized)
		return
	}
	organizationSlug := r.PathValue("organizationSlug")
	org, ok := s.findFakeOrgBySlug(organizationSlug)
	if !ok {
		http.NotFound(w, r)
		return
	}

	var teams []fakeTeam
	for _, team := range org.teams {
		teams = append(teams, team)
	}

	marshalJSON(s.t, w, teams)
}

func (s *fakeGitHubServer) findFakeOrgBySlug(slug string) (fakeOrganization, bool) {
	for _, org := range s.organizations {
		if org.Login == slug {
			return org, true
		}
	}

	return fakeOrganization{}, false
}

// organizationTeamMembersHandler fakes the Organization teams API
// Requires an Authorization: Bearer <installation_token>.
//
// https://docs.github.com/en/rest/teams/members?apiVersion=2022-11-28#list-team-members
func (s *fakeGitHubServer) organizationTeamMembersHandler(w http.ResponseWriter, r *http.Request) {
	s.t.Logf("Received request for %s", r.URL)
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error": "unauthorized", "message": "Missing Authorization header"}`, http.StatusUnauthorized)
		return
	}

	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		http.Error(w, `{"error": "invalid_token", "message": "Invalid Authorization header format"}`, http.StatusUnauthorized)
		return
	}
	accessToken := authHeader[len(bearerPrefix):]

	tokenData, ok := s.installationTokens[accessToken]
	if !ok || tokenData.Expiry.Before(time.Now()) {
		s.t.Logf("Invalid or expired access token: %s", accessToken)
		http.Error(w, `{"error": "invalid_token", "message": "Access token is invalid or expired"}`, http.StatusUnauthorized)
		return
	}

	var response []map[string]any
	// The members list does not include the user "name" field.
	for _, raw := range s.users {
		u := maps.Clone(raw)
		delete(u, "name")
		response = append(response, u)
	}

	marshalJSON(s.t, w, response)
}

// userSearchHandler fakes the user search api.
// Requires an Authorization: Bearer <installation_token>.
//
// https://docs.github.com/en/rest/search/search?apiVersion=2022-11-28#search-users
func (s *fakeGitHubServer) userSearchHandler(w http.ResponseWriter, r *http.Request) {
	s.t.Logf("Received request for %s", r.URL)
	if !s.verifyInstallationToken(r, w) {
		return
	}

	var response []map[string]any
	for _, user := range s.users {
		if strings.Contains(user["login"].(string), r.URL.Query().Get("q")) {
			response = append(response, user)
		}
	}

	marshalJSON(s.t, w, map[string]any{
		"total_count":        len(response),
		"items":              response,
		"incomplete_results": false,
	})
}

func (s *fakeGitHubServer) verifyInstallationToken(r *http.Request, w http.ResponseWriter) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		http.Error(w, `{"error": "unauthorized", "message": "Missing Authorization header"}`, http.StatusUnauthorized)
		return false
	}

	if len(authHeader) < len(bearerPrefix) || authHeader[:len(bearerPrefix)] != bearerPrefix {
		http.Error(w, `{"error": "invalid_token", "message": "Invalid Authorization header format"}`, http.StatusUnauthorized)
		return false
	}
	accessToken := authHeader[len(bearerPrefix):]

	tokenData, ok := s.installationTokens[accessToken]
	if !ok || tokenData.Expiry.Before(time.Now()) {
		s.t.Logf("Invalid or expired access token: %s", accessToken)
		http.Error(w, `{"error": "invalid_token", "message": "Access token is invalid or expired"}`, http.StatusUnauthorized)
		return false
	}
	return true
}
