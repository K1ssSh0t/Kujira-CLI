package sync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nstratos/go-myanimelist/mal"
	"golang.org/x/oauth2"
)

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

// GenerateCodeVerifier creates a high-entropy cryptographic random string used for PKCE.
func GenerateCodeVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func GenerateCodeChallenge(verifier string) string {
	return verifier
}

func NewMALSync(config *MALConfig) (*MALSync, error) {
	if config.ClientID == "" {
		return nil, fmt.Errorf("MAL client ID is required")
	}

	oauth2Conf := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
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
		oauth2Client := oauth2Conf.Client(malSync.oauth2Ctx, token)
		malSync.client = mal.NewClient(oauth2Client)

		if time.Until(token.Expiry) < 5*time.Minute {
			log.Println("Token expiring soon, refreshing...")
			if err := malSync.RefreshToken(); err != nil {
				log.Printf("Warning: Failed to refresh token: %v", err)
			}
		}
	}

	return malSync, nil
}

func (m *MALSync) UpdateAccessToken(accessToken, refreshToken string, expiry time.Time) error {
	m.token = &oauth2.Token{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Expiry:       expiry,
		TokenType:    "Bearer",
	}

	oauth2Client := m.oauth2Conf.Client(m.oauth2Ctx, m.token)
	m.client = mal.NewClient(oauth2Client)

	m.config.AccessToken = accessToken
	m.config.RefreshToken = refreshToken
	m.config.TokenExpiry = expiry

	return nil
}

func (m *MALSync) RefreshToken() error {
	if m.token == nil || m.token.RefreshToken == "" {
		return fmt.Errorf("no refresh token available")
	}

	data := url.Values{}
	data.Set("client_id", m.config.ClientID)
	data.Set("client_secret", m.config.ClientSecret)
	data.Set("grant_type", "refresh_token")
	data.Set("refresh_token", m.token.RefreshToken)

	req, err := http.NewRequest("POST", m.oauth2Conf.Endpoint.TokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to refresh token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("PKCE: Error response from MAL: %s", string(body))
		return fmt.Errorf("token refresh failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return fmt.Errorf("failed to decode token response: %v", err)
	}

	m.token.AccessToken = tokenResp.AccessToken
	m.token.RefreshToken = tokenResp.RefreshToken
	m.token.Expiry = time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)
	m.token.TokenType = tokenResp.TokenType

	oauth2Client := m.oauth2Conf.Client(m.oauth2Ctx, m.token)
	m.client = mal.NewClient(oauth2Client)

	m.config.AccessToken = tokenResp.AccessToken
	m.config.RefreshToken = tokenResp.RefreshToken
	m.config.TokenExpiry = m.token.Expiry

	log.Printf("Token refreshed successfully, expires at: %v", m.token.Expiry)

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

func (m *MALSync) GetAuthURL(codeChallenge string) string {
	log.Printf("PKCE: Generando URL de autenticación con code_challenge = %s", codeChallenge)
	return fmt.Sprintf(
		"%s?client_id=%s&response_type=code&redirect_uri=%s&code_challenge=%s&code_challenge_method=plain&state=state",
		m.oauth2Conf.Endpoint.AuthURL,
		m.config.ClientID,
		m.oauth2Conf.RedirectURL,
		codeChallenge,
	)
}

func (m *MALSync) ExchangeCodeForToken(code, codeVerifier string) (string, string, time.Time, error) {
	log.Printf("PKCE: Intercambiando código por token. code_verifier = %s", codeVerifier)
	log.Printf("PKCE: Código recibido = %s", code)

	data := url.Values{}
	data.Set("client_id", m.config.ClientID)
	data.Set("client_secret", m.config.ClientSecret)
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("code_verifier", codeVerifier)
	data.Set("redirect_uri", m.oauth2Conf.RedirectURL)

	reqBody := data.Encode()
	log.Printf("PKCE: Request body = %s", reqBody)

	req, err := http.NewRequest("POST", m.oauth2Conf.Endpoint.TokenURL, strings.NewReader(reqBody))
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	log.Printf("PKCE: Sending request to %s", m.oauth2Conf.Endpoint.TokenURL)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to exchange code for token: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("PKCE: Error response from MAL: %s", string(body))
		return "", "", time.Time{}, fmt.Errorf("token exchange failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		TokenType    string `json:"token_type"`
		ExpiresIn    int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", "", time.Time{}, fmt.Errorf("failed to decode token response: %v", err)
	}

	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	return tokenResp.AccessToken, tokenResp.RefreshToken, expiry, nil
}

type AuthCallbackHandler struct {
	CodeChan chan string
}

func (h *AuthCallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	log.Printf("PKCE: Código recibido en callback = %s", code)
	if code == "" {
		http.Error(w, "No code parameter", http.StatusBadRequest)
		return
	}

	h.CodeChan <- code

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`
		<html>
			<head><title>Authentication Successful</title></head>
			<body>
				<h1>Authentication Successful!</h1>
				<p>You can close this window and return to the terminal.</p>
			</body>
		</html>
	`))
}

func StartMALAuthServer() (chan string, error) {
	codeChan := make(chan string, 1)
	handler := &AuthCallbackHandler{CodeChan: codeChan}

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	go func() {
		log.Printf("Starting auth server on %s", MALRedirectURL)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Auth server error: %v", err)
		}
	}()

	return codeChan, nil
}
