package sync

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nstratos/go-myanimelist/mal"
)

type LibraryStatus string

const (
	StatusAll         LibraryStatus = "Todos"
	StatusWatching    LibraryStatus = "Viendo"
	StatusPlanToWatch LibraryStatus = "Para ver"
	StatusCompleted   LibraryStatus = "Completado"
	StatusDropped     LibraryStatus = "Dropeado"
	StatusOnHold      LibraryStatus = "En espera"
)

type LibraryItem struct {
	Title     string        `json:"title"`
	URL       string        `json:"url"`
	Image     string        `json:"image"`
	Status    LibraryStatus `json:"status"`
	Provider  string        `json:"provider"`
	UpdatedAt time.Time     `json:"updated_at"`
}

type HistoryItem struct {
	Title     string    `json:"title"`
	Episode   string    `json:"episode"`
	URL       string    `json:"url"`
	Provider  string    `json:"provider"`
	Timestamp time.Time `json:"timestamp"`
}

type ReadLibraryFunc func() ([]LibraryItem, error)
type ReadHistoryFunc func() ([]HistoryItem, error)

type SyncService struct {
	malSync     *MALSync
	matcher     *TitleMatcher
	config      *SyncConfig
	readLibrary ReadLibraryFunc
	readHistory ReadHistoryFunc
}

func NewSyncService(config *SyncConfig, readLibrary ReadLibraryFunc, readHistory ReadHistoryFunc) (*SyncService, error) {
	if config.Provider != ProviderMyAnimeList {
		return nil, fmt.Errorf("unsupported sync provider: %s", config.Provider)
	}

	malSync, err := NewMALSync(&config.MAL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize MAL sync: %v", err)
	}

	return &SyncService{
		malSync:     malSync,
		matcher:     NewTitleMatcher(),
		config:      config,
		readLibrary: readLibrary,
		readHistory: readHistory,
	}, nil
}

func (ss *SyncService) TestConnection() error {
	user, err := ss.malSync.GetUser()
	if err != nil {
		return fmt.Errorf("failed to get user info: %v", err)
	}

	log.Printf("Connected to MAL as: %s (%d)", user.Name, user.ID)
	return nil
}

func (ss *SyncService) SyncLibrary() (*SyncResult, error) {
	result := &SyncResult{
		Success: true,
		Message: "Library sync completed",
	}

	library, err := ss.readLibrary()
	if err != nil {
		result.Success = false
		result.Message = "Failed to read library"
		result.AddError(err)
		return result, nil
	}

	for _, item := range library {
		match, err := ss.matcher.MatchAnime(item.Title, ss.malSync)
		if err != nil {
			log.Printf("Warning: Could not match '%s': %v", item.Title, err)
			result.AddError(fmt.Errorf("no match for '%s': %v", item.Title, err))
			continue
		}

		malStatus := MALStatusFromLibrary(string(item.Status))
		err = ss.malSync.UpdateAnimeList(match.MALID, string(malStatus), 0, 0)
		if err != nil {
			log.Printf("Warning: Could not update '%s' on MAL: %v", item.Title, err)
			result.AddError(fmt.Errorf("failed to update '%s': %v", item.Title, err))
			continue
		}

		result.AddUploaded(item.Title)
		log.Printf("Synced '%s' -> MAL ID: %d (Status: %s)", item.Title, match.MALID, malStatus)
	}

	return result, nil
}

func (ss *SyncService) SyncHistory() (*SyncResult, error) {
	result := &SyncResult{
		Success: true,
		Message: "History sync completed",
	}

	history, err := ss.readHistory()
	if err != nil {
		result.Success = false
		result.Message = "Failed to read history"
		result.AddError(err)
		return result, nil
	}

	log.Printf("SyncHistory: Found %d items in history", len(history))

	for _, item := range history {
		log.Printf("SyncHistory: Processing item '%s' (Episode: %s)", item.Title, item.Episode)

		match, err := ss.matcher.MatchAnime(item.Title, ss.malSync)
		if err != nil {
			log.Printf("Warning: Could not match '%s': %v", item.Title, err)
			result.AddError(fmt.Errorf("no match for '%s': %v", item.Title, err))
			continue
		}

		log.Printf("SyncHistory: Matched '%s' to MAL ID: %d", item.Title, match.MALID)

		anime, err := ss.malSync.GetAnimeDetails(match.MALID)
		if err != nil {
			log.Printf("Warning: Could not get details for '%s': %v", item.Title, err)
			result.AddError(fmt.Errorf("failed to get details for '%s': %v", item.Title, err))
			continue
		}

		totalEpisodes := anime.NumEpisodes
		if totalEpisodes == 0 {
			totalEpisodes = 999
		}

		currentEpisodes := anime.MyListStatus.NumEpisodesWatched

		episodeNum := 0
		n, err := fmt.Sscanf(item.Episode, "Episodio %d", &episodeNum)
		if n != 1 {
			n, err = fmt.Sscanf(item.Episode, "%d", &episodeNum)
		}

		if n == 1 {
			log.Printf("SyncHistory: '%s' - History Ep: %d, Current MAL Ep: %d, Total: %d",
				item.Title, episodeNum, currentEpisodes, totalEpisodes)

			if episodeNum > currentEpisodes && episodeNum <= totalEpisodes {
				err = ss.malSync.UpdateAnimeList(match.MALID, string(mal.AnimeStatusWatching), 0, episodeNum)
				if err != nil {
					log.Printf("Warning: Could not update episode for '%s': %v", item.Title, err)
					result.AddError(fmt.Errorf("failed to update episode for '%s': %v", item.Title, err))
					continue
				}

				result.AddUpdated(fmt.Sprintf("%s - Ep %d", item.Title, episodeNum))
				log.Printf("Updated '%s' - Episode %d on MAL", item.Title, episodeNum)
			} else {
				log.Printf("SyncHistory: '%s' - No update needed (Ep %d <= Current %d or Ep %d > Total %d)",
					item.Title, episodeNum, currentEpisodes, episodeNum, totalEpisodes)
			}
		} else {
			log.Printf("SyncHistory: Could not parse episode from '%s': %v", item.Episode, err)
		}
	}

	return result, nil
}

func (ss *SyncService) PullFromMAL() (*SyncResult, error) {
	result := &SyncResult{
		Success: true,
		Message: "MAL sync pull completed",
	}

	malAnimeList, err := ss.malSync.GetAnimeList("watching")
	if err != nil {
		result.Success = false
		result.Message = "Failed to get MAL anime list"
		result.AddError(err)
		return result, nil
	}

	for _, userAnime := range malAnimeList {
		anime := userAnime.Anime

		kujiraStatus := LibraryStatusFromMAL(SyncStatus(userAnime.Status.Status))

		var imageURL string
		if anime.MainPicture.Medium != "" {
			imageURL = anime.MainPicture.Medium
		}

		item := LibraryItem{
			Title:     anime.Title,
			URL:       fmt.Sprintf("/anime/%d", anime.ID),
			Image:     imageURL,
			Status:    LibraryStatus(kujiraStatus),
			Provider:  "mal",
			UpdatedAt: time.Now(),
		}

		result.AddUpdated(item.Title)
		log.Printf("Pulled '%s' from MAL (Status: %s, Watched: %d/%d)",
			item.Title, kujiraStatus, userAnime.Status.NumEpisodesWatched, anime.NumEpisodes)
	}

	return result, nil
}

func (ss *SyncService) FullSync() (*SyncResult, error) {
	libraryResult, err := ss.SyncLibrary()
	if err != nil {
		return nil, fmt.Errorf("library sync failed: %v", err)
	}

	historyResult, err := ss.SyncHistory()
	if err != nil {
		return nil, fmt.Errorf("history sync failed: %v", err)
	}

	result := &SyncResult{
		Success:  true,
		Message:  "Full sync completed",
		Uploaded: append([]string{}, libraryResult.Uploaded...),
		Updated:  append([]string{}, libraryResult.Updated...),
		Errors:   append([]string{}, libraryResult.Errors...),
	}

	result.Uploaded = append(result.Uploaded, historyResult.Uploaded...)
	result.Updated = append(result.Updated, historyResult.Updated...)
	result.Errors = append(result.Errors, historyResult.Errors...)

	if result.HasErrors() {
		result.Message = "Full sync completed with errors"
		result.Success = false
	}

	return result, nil
}

func (ss *SyncService) SearchAndMatchAnime(title string) ([]AnimeMatch, error) {
	animeList, err := ss.malSync.SearchAnime(title)
	if err != nil {
		return nil, fmt.Errorf("failed to search MAL: %v", err)
	}

	matches := make([]AnimeMatch, 0, len(animeList))
	for _, anime := range animeList {
		score := ss.matcher.calculateScore(
			strings.Fields(normalizeTitle(title)),
			anime,
			normalizeTitle(title),
		)

		matches = append(matches, AnimeMatch{
			MALID:   anime.ID,
			Title:   anime.Title,
			TitleEN: anime.AlternativeTitles.En,
			TitleJP: anime.AlternativeTitles.Ja,
			Score:   score,
		})
	}

	return matches, nil
}

func (ss *SyncService) AddToMAL(title string, status string, score int, watchedEpisodes int) error {
	match, err := ss.matcher.MatchAnime(title, ss.malSync)
	if err != nil {
		return fmt.Errorf("failed to match anime: %v", err)
	}

	return ss.malSync.UpdateAnimeList(match.MALID, status, score, watchedEpisodes)
}
