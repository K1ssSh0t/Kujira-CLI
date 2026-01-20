package sync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type SyncProvider string

const (
	ProviderMyAnimeList SyncProvider = "myanimelist"
	ProviderAniList     SyncProvider = "anilist"
)

type SyncConfig struct {
	Provider   SyncProvider  `json:"provider"`
	MAL        MALConfig     `json:"mal"`
	AniList    AniListConfig `json:"anilist"`
	LastSyncAt time.Time     `json:"last_sync_at"`
	AutoSync   bool          `json:"auto_sync"`
}

type MALConfig struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenExpiry  time.Time `json:"token_expiry"`
	ClientID     string    `json:"client_id"`
	ClientSecret string    `json:"client_secret"`
	AuthCode     string    `json:"auth_code"`
}

type AniListConfig struct {
	AccessToken string `json:"access_token"`
}

func getSyncConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, ".kujira_cli")
	return filepath.Join(configDir, "sync_config.json"), nil
}

func ReadSyncConfig() (*SyncConfig, error) {
	configPath, err := getSyncConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &SyncConfig{
				AutoSync: false,
			}, nil
		}
		return nil, err
	}

	var config SyncConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func SaveSyncConfig(config *SyncConfig) error {
	configPath, err := getSyncConfigPath()
	if err != nil {
		return err
	}

	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(configPath, data, 0600)
}

type AnimeMatch struct {
	MALID   int     `json:"mal_id"`
	Title   string  `json:"title"`
	TitleEN string  `json:"title_en"`
	TitleJP string  `json:"title_jp"`
	Score   float64 `json:"score"`
}

type SyncStatus string

const (
	SyncStatusWatching    SyncStatus = "watching"
	SyncStatusCompleted   SyncStatus = "completed"
	SyncStatusPlanToWatch SyncStatus = "plan_to_watch"
	SyncStatusOnHold      SyncStatus = "on_hold"
	SyncStatusDropped     SyncStatus = "dropped"
)

type SyncAnime struct {
	MALID           int        `json:"mal_id"`
	Title           string     `json:"title"`
	Status          SyncStatus `json:"status"`
	Score           int        `json:"score"`
	WatchedEpisodes int        `json:"watched_episodes"`
	TotalEpisodes   int        `json:"total_episodes"`
}

func MALStatusFromLibrary(status string) SyncStatus {
	switch status {
	case "Viendo":
		return SyncStatusWatching
	case "Completado":
		return SyncStatusCompleted
	case "Para ver":
		return SyncStatusPlanToWatch
	case "En espera":
		return SyncStatusOnHold
	case "Dropeado":
		return SyncStatusDropped
	default:
		return SyncStatusPlanToWatch
	}
}

func LibraryStatusFromMAL(status SyncStatus) string {
	switch status {
	case SyncStatusWatching:
		return "Viendo"
	case SyncStatusCompleted:
		return "Completado"
	case SyncStatusPlanToWatch:
		return "Para ver"
	case SyncStatusOnHold:
		return "En espera"
	case SyncStatusDropped:
		return "Dropeado"
	default:
		return "Para ver"
	}
}

type SyncResult struct {
	Success  bool     `json:"success"`
	Message  string   `json:"message"`
	Uploaded []string `json:"uploaded"`
	Updated  []string `json:"updated"`
	Errors   []string `json:"errors"`
}

func (sr *SyncResult) AddError(err error) {
	sr.Errors = append(sr.Errors, err.Error())
}

func (sr *SyncResult) AddUploaded(title string) {
	sr.Uploaded = append(sr.Uploaded, title)
}

func (sr *SyncResult) AddUpdated(title string) {
	sr.Updated = append(sr.Updated, title)
}

func (sr *SyncResult) HasErrors() bool {
	return len(sr.Errors) > 0
}

func (sr *SyncResult) String() string {
	return fmt.Sprintf("Success: %v, Message: %s, Uploaded: %d, Updated: %d, Errors: %d",
		sr.Success, sr.Message, len(sr.Uploaded), len(sr.Updated), len(sr.Errors))
}
