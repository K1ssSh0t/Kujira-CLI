package extractors

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	baseURL = "https://kujira.unusualoticramo.workers.dev"
)

type LatestEpisode struct {
	Title   string `json:"title"`
	Image   string `json:"img"`
	Url     string `json:"url"`
	Episode string `json:"episode"`
}

// Estructuras para los datos de un episodio
type Episode struct {
	Title   string    `json:"title"`
	Number  string    `json:"numberEpisode"`
	Servers []Servers `json:"servers"`
}

// Estructura para los servidores de un episodio
type Servers struct {
	Name   string `json:"name"`
	Url    string `json:"url"`	
	Source string `json:"source"`
}

// Estructura para los datos de un anime de animeflv
type Anime struct {
	Title            string      `json:"title"`
	TitleAlternative string      `json:"title_alt"`
	Image            string      `json:"img"`
	Status           interface{} `json:"status"`
	Synopsis         string      `json:"synopsis"`
	Genres           []string    `json:"genres"`
	Episodes         []Episodes  `json:"episodes"`
}

func (a *Anime) UnmarshalJSON(data []byte) error {
	type Alias Anime
	aux := &struct {
		Status interface{} `json:"status"`
		*Alias
	}{
		Alias: (*Alias)(a),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	switch v := aux.Status.(type) {
	case string:
		a.Status = v
	case bool:
		if v {
			a.Status = "En emisión"
		} else {
			a.Status = "Finalizado"
		}
	default:
		a.Status = "Desconocido"
	}
	return nil
}

type Episodes struct {
	Name   string `json:"name"`
	Url    string `json:"url"`
	Number string `json:"number"`
}

type SearchResult struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	Url   string `json:"url"`
	Type  string `json:"type"`
}

func FetchLatestEpisodes(source string) ([]LatestEpisode, error) {
	url := fmt.Sprintf("%s/%s/latest", baseURL, source)
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var episodes []LatestEpisode
	if err := json.NewDecoder(resp.Body).Decode(&episodes); err != nil {
		return nil, err
	}

	return episodes, nil
}

func FetchEpisodeDetail(source, episodeID string) (*Episode, error) {
	url := fmt.Sprintf("%s/%s/episode/%s", baseURL, source, episodeID)
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var ep Episode
	if err := json.NewDecoder(resp.Body).Decode(&ep); err != nil {
		return nil, err
	}
	return &ep, nil
}

func FetchAnime(source, animeID string) (*Anime, error) {
	url := fmt.Sprintf("%s/%s/anime/%s", baseURL, source, animeID)
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var anime Anime
	if err := json.NewDecoder(resp.Body).Decode(&anime); err != nil {
		return nil, err
	}
	return &anime, nil
}

func FetchSearch(source, query string) ([]SearchResult, error) {
	url := fmt.Sprintf("%s/%s/search?title=%s", baseURL, source, query)
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var results []SearchResult
	if err := json.Unmarshal(body, &results); err != nil {
		return nil, err
	}
	return results, nil
}