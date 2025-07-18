package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Estructuras para los datos al obtenr los utlimos episodios
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
	Title            string   `json:"title"`
	TitleAlternative string   `json:"titleAlternative"`
	Image            string   `json:"img"`
	Status           string   `json:"status"`
	Synopsis         string   `json:"synopsis"`
	Genres           []string `json:"genres"`
}

// Modelo para los datos de la API (ejemplo con JSONPlaceholder)
type source int

const (
	none source = iota
	animeflv
	tioanime
)

type model struct {
	stage         int // 0: menu, 1: episodes, 2: episode detail
	selectedSrc   source
	episodes      []LatestEpisode
	selectedIndex int
	episodeDetail *Episode
	status        string
	err           error
}

// Estilos con lipgloss
var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true)

	itemStyle           = lipgloss.NewStyle().PaddingLeft(1)
	errorStyle          = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	cursorStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700"))
	inactiveServerStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#777777")) // Color gris para servidores inactivos
)

// Comando para obtener datos de la API
type episodesMsg struct {
	episodes []LatestEpisode
	err      error
}

type episodeDetailMsg struct {
	episode *Episode
	err     error
}

func fetchEpisodes(src source) tea.Msg {
	var url string
	if src == animeflv {
		url = "https://kujira.unusualoticramo.workers.dev/animeflv/latest" // Cambia por tu API real
	} else {
		url = "https://kujira.unusualoticramo.workers.dev/tioanime/latest" // Cambia por tu API real
	}
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return episodesMsg{err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return episodesMsg{err: err}
	}
	var episodes []LatestEpisode
	if err := json.Unmarshal(body, &episodes); err != nil {
		return episodesMsg{err: err}
	}
	return episodesMsg{episodes: episodes}
}

func fetchEpisodeDetail(src source, episodeID string) tea.Msg {
	var url string
	if src == animeflv {
		url = fmt.Sprintf("https://kujira.unusualoticramo.workers.dev/animeflv/episode/%s", episodeID)
	} else {
		url = fmt.Sprintf("https://kujira.unusualoticramo.workers.dev/tioanime/episode/%s", episodeID)
	}
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return episodeDetailMsg{err: err}
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return episodeDetailMsg{err: err}
	}
	var ep Episode
	if err := json.Unmarshal(body, &ep); err != nil {
		return episodeDetailMsg{err: err}
	}
	return episodeDetailMsg{episode: &ep}
}

// Inicialización del modelo
func initialModel() model {
	return model{
		stage:         0,
		selectedSrc:   none,
		selectedIndex: 0,
		status:        "Selecciona una fuente:",
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func extractEpisodeID(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.stage {
		case 0: // Menu
			switch msg.String() {
			case "down", "j":
				if m.selectedIndex < 1 {
					m.selectedIndex++
				}
			case "up", "k":
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
			case "enter":
				if m.selectedIndex == 0 {
					m.selectedSrc = animeflv
				} else {
					m.selectedSrc = tioanime
				}
				m.stage = 1
				m.status = "Cargando episodios..."
				return m, func() tea.Msg { return fetchEpisodes(m.selectedSrc) }
			case "q", "ctrl+c":
				return m, tea.Quit
			}
		case 1: // Lista de episodios
			switch msg.String() {
			case "down", "j":
				if m.selectedIndex < len(m.episodes)-1 {
					m.selectedIndex++
				}
			case "up", "k":
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
			case "enter":
				ep := m.episodes[m.selectedIndex]
				episodeID := extractEpisodeID(ep.Url) // Extrae solo el ID del episodio
				m.stage = 2
				m.status = "Cargando episodio..."
				return m, func() tea.Msg { return fetchEpisodeDetail(m.selectedSrc, episodeID) }
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc", "left", "h":
				m.stage = 0
				m.selectedIndex = 0
				m.episodes = nil
				m.status = "Selecciona una fuente:"
			}
		case 2: // Detalle de episodio
			switch msg.String() {
			case "down", "j":
				// Solo avanzar si hay más servidores y el siguiente es seleccionable
				if m.selectedIndex < len(m.episodeDetail.Servers)-1 {
					nextIndex := m.selectedIndex + 1
					// Buscar el siguiente servidor con Source
					for nextIndex < len(m.episodeDetail.Servers) &&
						(m.episodeDetail.Servers[nextIndex].Source == "" ||
							strings.ToLower(m.episodeDetail.Servers[nextIndex].Source) == "unknown") {
						nextIndex++
					}
					if nextIndex < len(m.episodeDetail.Servers) {
						m.selectedIndex = nextIndex
					}
				}
			case "up", "k":
				// Solo retroceder si hay servidores anteriores seleccionables
				if m.selectedIndex > 0 {
					prevIndex := m.selectedIndex - 1
					// Buscar el servidor anterior con Source
					for prevIndex >= 0 &&
						(m.episodeDetail.Servers[prevIndex].Source == "" ||
							strings.ToLower(m.episodeDetail.Servers[prevIndex].Source) == "unknown") {
						prevIndex--
					}
					if prevIndex >= 0 {
						m.selectedIndex = prevIndex
					}
				}
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc", "left", "h":
				m.stage = 1
				m.episodeDetail = nil
				m.status = "Cargando episodios..."
				return m, func() tea.Msg { return fetchEpisodes(m.selectedSrc) }
			}
		}
	case episodesMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "Error al cargar episodios"
		} else {
			m.episodes = msg.episodes
			m.selectedIndex = 0
			m.status = "Selecciona un episodio:"
		}
	case episodeDetailMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "Error al cargar episodio"
		} else {
			m.episodeDetail = msg.episode
			m.status = "Detalle del episodio:"
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}
	switch m.stage {
	case 0:
		s := titleStyle.Render("Kujira CLI") + "\n\n"
		options := []string{"AnimeFLV", "TioAnime"}
		for i, opt := range options {
			cursor := "  "
			if m.selectedIndex == i {
				cursor = cursorStyle.Render("➤ ")
			}
			s += fmt.Sprintf("%s%s\n", cursor, opt)
		}
		s += "\nUsa ↑/↓ y Enter para seleccionar. 'q' para salir."
		return s
	case 1:
		s := titleStyle.Render("Últimos Episodios") + "\n\n"
		for i, ep := range m.episodes {
			cursor := "  "
			if m.selectedIndex == i {
				cursor = cursorStyle.Render("➤ ")
			}
			s += fmt.Sprintf("%s%s [%s]\n", cursor, ep.Title, ep.Episode)
		}
		s += "\nEnter para ver episodio, 'esc' para volver, 'q' para salir."
		return s
	case 2:
		if m.episodeDetail == nil {
			return m.status
		}
		ep := m.episodeDetail
		s := titleStyle.Render(fmt.Sprintf("Episodio: %s (%s)", ep.Title, ep.Number)) + "\n\n"
		for i, srv := range ep.Servers {
			cursor := "  "
			sourceValid := srv.Source != "" && strings.ToLower(srv.Source) != "unknown"
			if m.selectedIndex == i && sourceValid {
				cursor = cursorStyle.Render("➤ ")
			}

			serverText := fmt.Sprintf("Servidor: %s\nURL: %s", srv.Name, srv.Url)
			if sourceValid {
				serverText += fmt.Sprintf("\nFuente: %s", srv.Source)
			}

			// Aplica estilo diferente si no tiene Source válido
			if !sourceValid {
				serverText = inactiveServerStyle.Render(serverText)
			}
			s += fmt.Sprintf("%s%s\n\n", cursor, serverText)
		}
		s += "\n'esc' para volver, 'q' para salir."
		return s
	}
	return m.status
}

func main() {
	p := tea.NewProgram(initialModel())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
