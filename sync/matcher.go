package sync

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/nstratos/go-myanimelist/mal"
)

type TitleMatcher struct {
	animeCache map[int]string
}

func NewTitleMatcher() *TitleMatcher {
	return &TitleMatcher{
		animeCache: make(map[int]string),
	}
}

func normalizeTitle(title string) string {
	title = strings.ToLower(title)
	title = strings.TrimSpace(title)

	title = strings.ReplaceAll(title, "-", " ")
	title = strings.ReplaceAll(title, "_", " ")
	title = strings.ReplaceAll(title, ".", " ")
	title = strings.ReplaceAll(title, ":", " ")
	title = strings.ReplaceAll(title, ";", " ")

	var result []rune
	for _, r := range title {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsSpace(r) {
			result = append(result, r)
		}
	}

	words := strings.Fields(string(result))
	return strings.Join(words, " ")
}

func (tm *TitleMatcher) findBestMatch(kujiraTitle string, animeList []mal.Anime) (*AnimeMatch, error) {
	if len(animeList) == 0 {
		return nil, fmt.Errorf("no anime found in MAL")
	}

	normalizedKujira := normalizeTitle(kujiraTitle)
	kujiraWords := strings.Fields(normalizedKujira)

	var bestMatch *AnimeMatch
	bestScore := 0.0

	for _, anime := range animeList {
		score := tm.calculateScore(kujiraWords, anime, normalizedKujira)

		if score > bestScore {
			bestScore = score
			bestMatch = &AnimeMatch{
				MALID:   anime.ID,
				Title:   anime.Title,
				TitleEN: anime.AlternativeTitles.En,
				TitleJP: anime.AlternativeTitles.Ja,
				Score:   score,
			}
		}
	}

	if bestScore < 0.3 {
		return nil, fmt.Errorf("no good match found (best score: %.2f)", bestScore)
	}

	return bestMatch, nil
}

func (tm *TitleMatcher) calculateScore(kujiraWords []string, anime mal.Anime, normalizedKujira string) float64 {
	titles := []string{anime.Title}
	if anime.AlternativeTitles.En != "" {
		titles = append(titles, anime.AlternativeTitles.En)
	}
	if anime.AlternativeTitles.Ja != "" {
		titles = append(titles, anime.AlternativeTitles.Ja)
	}
	for _, syn := range anime.AlternativeTitles.Synonyms {
		if syn != "" {
			titles = append(titles, syn)
		}
	}

	maxScore := 0.0
	for _, title := range titles {
		normalized := normalizeTitle(title)
		titleWords := strings.Fields(normalized)
		score := tm.calculateTitleScore(kujiraWords, titleWords, normalizedKujira, normalized)
		if score > maxScore {
			maxScore = score
		}
	}

	return maxScore
}

func (tm *TitleMatcher) calculateTitleScore(kujiraWords, malWords []string, normalizedKujira, normalizedMAL string) float64 {
	if normalizedKujira == normalizedMAL {
		return 1.0
	}

	if len(kujiraWords) == 0 || len(malWords) == 0 {
		return 0.0
	}

	matches := 0
	for _, kw := range kujiraWords {
		for _, mw := range malWords {
			if kw == mw && len(kw) > 2 {
				matches++
				break
			}
		}
	}

	wordScore := float64(matches) / float64(max(len(kujiraWords), len(malWords)))

	if len(normalizedKujira) < len(normalizedMAL) {
		if strings.Contains(normalizedMAL, normalizedKujira) && len(normalizedKujira) > 3 {
			return 0.9
		}
	} else if strings.Contains(normalizedKujira, normalizedMAL) && len(normalizedMAL) > 3 {
		return 0.85
	}

	return wordScore * 0.8
}

func (tm *TitleMatcher) MatchAnime(kujiraTitle string, malSync *MALSync) (*AnimeMatch, error) {
	animeList, err := malSync.SearchAnime(kujiraTitle)
	if err != nil {
		return nil, fmt.Errorf("failed to search MAL: %v", err)
	}

	return tm.findBestMatch(kujiraTitle, animeList)
}

func (tm *TitleMatcher) MatchAnimeByID(kujiraTitle string, malSync *MALSync, malID int) (*AnimeMatch, error) {
	anime, err := malSync.GetAnimeDetails(malID)
	if err != nil {
		return nil, fmt.Errorf("failed to get MAL anime details: %v", err)
	}

	return &AnimeMatch{
		MALID:   anime.ID,
		Title:   anime.Title,
		TitleEN: anime.AlternativeTitles.En,
		TitleJP: anime.AlternativeTitles.Ja,
		Score:   1.0,
	}, nil
}
