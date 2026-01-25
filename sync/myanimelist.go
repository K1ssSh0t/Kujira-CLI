package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/nstratos/go-myanimelist/mal"
	"golang.org/x/oauth2"
)

// KujiraBackendURL is the URL of the Kujira backend for MAL auth
// Change this to your deployed backend URL
const KujiraBackendURL = "https://kujira.unusualoticramo.workers.dev"

const (
	MALRedirectURL = "http://localhost:8080/callback"
)

type MALSync struct {
	client     *mal.Client
	oauth2Ctx  context.Context
	oauth2Conf *oauth2.Config
	token      *oauth2.Token
	config     *MALConfig
}

// AuthInitResponse is the response from the backend /auth/mal/init endpoint
type AuthInitResponse struct {
	AuthURL string `json:"authUrl"`
	State   string `json:"state"`
}

// TokenResponse is the response from the backend with tokens
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

// BackendStatus is the response from the backend /auth/mal/status endpoint
type BackendStatus struct {
	Configured bool   `json:"configured"`
	Message    string `json:"message"`
}

// CheckBackendStatus checks if the Kujira backend is configured for MAL auth
func CheckBackendStatus() (*BackendStatus, error) {
	resp, err := http.Get(KujiraBackendURL + "/auth/mal/status")
	if err != nil {
		return nil, fmt.Errorf("failed to reach backend: %v", err)
	}
	defer resp.Body.Close()

	var status BackendStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return &status, nil
}

// GetAuthURLFromBackend gets the OAuth auth URL from the Kujira backend
func GetAuthURLFromBackend() (*AuthInitResponse, error) {
	resp, err := http.Get(KujiraBackendURL + "/auth/mal/init?client=cli")
	if err != nil {
		return nil, fmt.Errorf("failed to get auth URL from backend: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("backend returned status %d: %s", resp.StatusCode, string(body))
	}

	var authResp AuthInitResponse
	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return nil, fmt.Errorf("failed to decode auth response: %v", err)
	}

	return &authResp, nil
}

// RefreshTokenViaBackend refreshes the access token using the Kujira backend
func RefreshTokenViaBackend(refreshToken string) (*TokenResponse, error) {
	body := map[string]string{"refresh_token": refreshToken}
	jsonBody, _ := json.Marshal(body)

	resp, err := http.Post(
		KujiraBackendURL+"/auth/mal/refresh",
		"application/json",
		bytes.NewBuffer(jsonBody),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to refresh token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %v", err)
	}

	return &tokenResp, nil
}

func NewMALSync(config *MALConfig) (*MALSync, error) {
	// OAuth2 config without client credentials (backend handles them)
	oauth2Conf := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://myanimelist.net/v1/oauth2/authorize",
			TokenURL: "https://myanimelist.net/v1/oauth2/token",
		},
		RedirectURL: MALRedirectURL,
	}

	malSync := &MALSync{
		oauth2Conf: oauth2Conf,
		oauth2Ctx:  context.Background(),
		config:     config,
	}

	if config.AccessToken != "" {
		token := &oauth2.Token{
			AccessToken:  config.AccessToken,
			RefreshToken: config.RefreshToken,
			Expiry:       config.TokenExpiry,
			TokenType:    "Bearer",
		}

		malSync.token = token

		// Create HTTP client that uses the access token
		httpClient := &http.Client{
			Transport: &bearerAuthTransport{
				Token:     config.AccessToken,
				Transport: http.DefaultTransport,
			},
		}
		malSync.client = mal.NewClient(httpClient)

		// Check if token needs refresh
		if time.Until(token.Expiry) < 5*time.Minute {
			log.Println("Token expiring soon, refreshing via backend...")
			if err := malSync.RefreshToken(); err != nil {
				log.Printf("Warning: Failed to refresh token: %v", err)
			}
		}
	}

	return malSync, nil
}

// bearerAuthTransport adds Bearer token to requests
type bearerAuthTransport struct {
	Token     string
	Transport http.RoundTripper
}

func (t *bearerAuthTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+t.Token)
	return t.Transport.RoundTrip(req)
}

func (m *MALSync) UpdateAccessToken(accessToken, refreshToken string, expiry time.Time) error {
	m.token = &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       expiry,
		TokenType:    "Bearer",
	}

	// Create HTTP client with bearer auth
	httpClient := &http.Client{
		Transport: &bearerAuthTransport{
			Token:     accessToken,
			Transport: http.DefaultTransport,
		},
	}
	m.client = mal.NewClient(httpClient)

	m.config.AccessToken = accessToken
	m.config.RefreshToken = refreshToken
	m.config.TokenExpiry = expiry

	return nil
}

func (m *MALSync) RefreshToken() error {
	if m.token == nil || m.token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	log.Println("Refreshing token via Kujira backend...")

	tokenResp, err := RefreshTokenViaBackend(m.token.RefreshToken)
	if err != nil {
		return err
	}

	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	m.token.AccessToken = tokenResp.AccessToken
	m.token.RefreshToken = tokenResp.RefreshToken
	m.token.Expiry = expiry

	// Create HTTP client with new token
	httpClient := &http.Client{
		Transport: &bearerAuthTransport{
			Token:     tokenResp.AccessToken,
			Transport: http.DefaultTransport,
		},
	}
	m.client = mal.NewClient(httpClient)

	m.config.AccessToken = tokenResp.AccessToken
	m.config.RefreshToken = tokenResp.RefreshToken
	m.config.TokenExpiry = expiry

	log.Printf("Token refreshed successfully, expires at: %v", expiry)

	return nil
}

func (m *MALSync) SearchAnime(title string) ([]mal.Anime, error) {
	ctx := context.Background()
	animeList, _, err := m.client.Anime.List(ctx, title,
		mal.Limit(10),
		mal.NSFW(true),
		mal.Fields([]string{"id", "title", "alternative_titles", "num_episodes", "main_picture", "mean"}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to search anime: %v", err)
	}

	return animeList, nil
}

func (m *MALSync) GetAnimeDetails(animeID int) (*mal.Anime, error) {
	ctx := context.Background()
	anime, _, err := m.client.Anime.Details(ctx, animeID, mal.Fields([]string{
		"id", "title", "alternative_titles", "num_episodes", "status",
		"mean", "genres", "synopsis", "main_picture", "my_list_status",
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to get anime details: %v", err)
	}

	return anime, nil
}

func (m *MALSync) UpdateAnimeList(animeID int, status string, score int, watchedEpisodes int) error {
	ctx := context.Background()

	var malStatus mal.AnimeStatus
	switch strings.ToLower(status) {
	case "watching":
		malStatus = mal.AnimeStatusWatching
	case "completed":
		malStatus = mal.AnimeStatusCompleted
	case "plan_to_watch":
		malStatus = mal.AnimeStatusPlanToWatch
	case "on_hold":
		malStatus = mal.AnimeStatusOnHold
	case "dropped":
		malStatus = mal.AnimeStatusDropped
	default:
		malStatus = mal.AnimeStatusPlanToWatch
	}

	_, _, err := m.client.Anime.UpdateMyListStatus(ctx, animeID,
		mal.AnimeStatus(malStatus),
		mal.Score(score),
		mal.NumEpisodesWatched(watchedEpisodes),
	)
	if err != nil {
		return fmt.Errorf("failed to update anime list: %v", err)
	}

	return nil
}

func (m *MALSync) GetAnimeList(status string) ([]mal.UserAnime, error) {
	ctx := context.Background()

	var malStatus mal.AnimeStatus
	switch strings.ToLower(status) {
	case "watching":
		malStatus = mal.AnimeStatusWatching
	case "completed":
		malStatus = mal.AnimeStatusCompleted
	case "plan_to_watch":
		malStatus = mal.AnimeStatusPlanToWatch
	case "on_hold":
		malStatus = mal.AnimeStatusOnHold
	case "dropped":
		malStatus = mal.AnimeStatusDropped
	default:
		malStatus = mal.AnimeStatusWatching
	}

	animeList, _, err := m.client.User.AnimeList(ctx, "@me",
		mal.AnimeStatus(malStatus),
		mal.Limit(100),
		mal.Fields([]string{
			"id", "title", "alternative_titles", "num_episodes", "status",
			"mean", "genres", "my_list_status", "main_picture",
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get anime list: %v", err)
	}

	return animeList, nil
}

func (m *MALSync) GetUser() (*mal.User, error) {
	ctx := context.Background()
	user, _, err := m.client.User.MyInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %v", err)
	}
	return user, nil
}

// AuthCallbackHandler handles the OAuth callback from the Kujira backend
type AuthCallbackHandler struct {
	TokenChan chan *TokenResponse
}

func (h *AuthCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// The backend sends tokens as query params
	accessToken := r.URL.Query().Get("access_token")
	refreshToken := r.URL.Query().Get("refresh_token")
	expiresIn := r.URL.Query().Get("expires_in")

	if accessToken == "" || refreshToken == "" {
		// Check for error
		errMsg := r.URL.Query().Get("error")
		if errMsg != "" {
			log.Printf("Auth error: %s", errMsg)
		}
		http.Error(w, "Authentication failed", http.StatusBadRequest)
		return
	}

	var expiresInInt int
	fmt.Sscanf(expiresIn, "%d", &expiresInInt)

	h.TokenChan <- &TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresInInt,
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
		<html>
			<head><title>Authentication Successful</title></head>
			<body>
				<h1>Authentication Successful!</h1>
				<p>You have been connected to MyAnimeList.</p>
				<p>You can close this window and return to the terminal.</p>
			</body>
		</html>
	`))
}

func StartMALAuthServer() (chan *TokenResponse, *http.Server, error) {
	tokenChan := make(chan *TokenResponse, 1)
	handler := &AuthCallbackHandler{TokenChan: tokenChan}

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	go func() {
		log.Printf("Starting auth callback server on %s", MALRedirectURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Auth server error: %v", err)
		}
	}()

	return tokenChan, server, nil
}
