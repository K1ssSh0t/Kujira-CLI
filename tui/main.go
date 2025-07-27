package tui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"kujira_cli/extractors"
)

type source int

const (
	none source = iota
	animeflv
	tioanime
)

func (s source) String() string {
	return [...]string{"none", "animeflv", "tioanime"}[s]
}

type appStage int

const (
	sourceSelection appStage = iota
	episodeList
	episodeDetail
	animeInfo
	search
)

type model struct {
	list                    list.Model
	viewport                viewport.Model
	searchInput             textinput.Model // New text input for search
	selectedSrc             source
	episodes                []extractors.LatestEpisode
	selectedEpisode         *extractors.LatestEpisode
	episodeDetail           *extractors.Episode
	anime                   *extractors.Anime
	status                  string
	err                     error
	currentStage            appStage
	selectedIndex           int // For server and episode navigation
	animeEpisodesPerPage    int
	currentAnimeEpisodePage int
}

func New() *model {
	// Initialize list and viewport with default values
	ti := textinput.New()
	ti.Placeholder = "Search..."
	ti.CharLimit = 100
	ti.Width = 50

	return &model{
		list:                    NewList([]list.Item{}, "Loading...", 30, 10),
		viewport:                NewViewport(50, 20),
		searchInput:             ti,
		currentStage:            sourceSelection, // Start at the source selection stage
		animeEpisodesPerPage:    10,
		currentAnimeEpisodePage: 0,
	}
}

func (m *model) Init() tea.Cmd {
	return tea.EnterAltScreen
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		h, v := AppStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
		m.viewport.Width = msg.Width - h - m.list.Width()
		m.viewport.Height = msg.Height - v

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "enter":
			switch m.currentStage {
			case sourceSelection:
				if m.selectedIndex == 0 {
					m.selectedSrc = animeflv
				} else {
					m.selectedSrc = tioanime
				}
				m.currentStage = episodeList
				m.status = "Cargando episodios..."
				return m, m.fetchEpisodes()
			case episodeList:
				selectedItem := m.list.SelectedItem().(Item)
				// Check if it's an anime from search results
				if strings.Contains(m.status, "Resultados de búsqueda:") && selectedItem.DescriptionValue == "Anime" {
					m.selectedEpisode = &extractors.LatestEpisode{Url: selectedItem.TitleValue} // Use TitleValue as URL for anime
					m.currentStage = animeInfo
					m.status = "Cargando información del anime..."
					m.selectedIndex = 0
					return m, m.fetchAnime()
				} else {
					for _, ep := range m.episodes {
						if ep.Title == selectedItem.TitleValue && ep.Episode == selectedItem.DescriptionValue {
							m.selectedEpisode = &ep
							break
						}
					}
					if m.selectedEpisode != nil {
						m.currentStage = episodeDetail
						m.status = "Cargando detalles del episodio..."
						m.selectedIndex = 0 // Reset index for server selection
						return m, m.fetchEpisodeDetail()
					}
				}
			case episodeDetail:
				if m.episodeDetail != nil && len(m.episodeDetail.Servers) > 0 {
					srv := m.episodeDetail.Servers[m.selectedIndex]
					m.status = fmt.Sprintf("Playing %s from %s...", m.episodeDetail.Title, srv.Name)
					return m, m.playVideo(srv.Name, srv.Url, srv.Source)
				}
			case animeInfo:
				if m.anime != nil && len(m.anime.Episodes) > 0 {
					ep := m.anime.Episodes[m.selectedIndex]
					m.selectedEpisode = &extractors.LatestEpisode{Url: ep.Url} // Create a dummy LatestEpisode for fetching detail
					m.currentStage = episodeDetail
					m.status = "Cargando detalles del episodio..."
					m.selectedIndex = 0 // Reset index for server selection
					return m, m.fetchEpisodeDetail()
				}
			case search:
				m.searchInput.Blur()
				m.status = "Buscando..."
				return m, m.fetchSearch(m.searchInput.Value())
			}
		case "esc":
			switch m.currentStage {
			case episodeList:
				m.currentStage = sourceSelection
				m.selectedIndex = 0
				m.episodes = nil
				m.status = "Selecciona una fuente:"
				return m, nil
			case episodeDetail:
				m.currentStage = episodeList
				m.episodeDetail = nil
				m.selectedEpisode = nil
				m.status = "Selecciona un episodio:"
				return m, nil
			case animeInfo:
				m.currentStage = episodeDetail
				m.anime = nil
				m.status = "Detalle del episodio:"
				return m, nil
			case search:
				m.currentStage = episodeList
				m.searchInput.SetValue("")
				m.searchInput.Blur()
				m.status = "Selecciona un episodio:"
				return m, nil
			}
		case "h":
			// Solo manejar 'h' como "atrás" si NO estamos en modo búsqueda
			if m.currentStage != search {
				switch m.currentStage {
				case episodeList:
					m.currentStage = sourceSelection
					m.selectedIndex = 0
					m.episodes = nil
					m.status = "Selecciona una fuente:"
					return m, nil
				case episodeDetail:
					m.currentStage = episodeList
					m.episodeDetail = nil
					m.selectedEpisode = nil
					m.status = "Selecciona un episodio:"
					return m, nil
				case animeInfo:
					m.currentStage = episodeDetail
					m.anime = nil
					m.status = "Detalle del episodio:"
					return m, nil
				}
			}
		case "i": // View anime info
			if m.currentStage == episodeDetail && m.selectedEpisode != nil {
				m.currentStage = animeInfo
				m.status = "Cargando información del anime..."
				m.selectedIndex = 0 // Reset index for episode selection
				return m, m.fetchAnime()
			}
		case "s": // Search
			if m.currentStage == episodeList {
				m.currentStage = search
				m.status = "Introduce tu búsqueda:"
				m.searchInput.Focus()
				return m, textinput.Blink
			}
		case "up", "k":
			switch m.currentStage {
			case sourceSelection:
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
			case episodeDetail:
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
			case animeInfo:
				if m.selectedIndex > 0 {
					m.selectedIndex--
				}
			}
		case "pgup":
			if m.currentStage == animeInfo && m.currentAnimeEpisodePage > 0 {
				m.currentAnimeEpisodePage--
				// No resetear el selectedIndex para mantener la posición
			}
		case "down", "j":
			switch m.currentStage {
			case sourceSelection:
				if m.selectedIndex < 1 {
					m.selectedIndex++
				}
			case episodeDetail:
				if m.episodeDetail != nil && m.selectedIndex < len(m.episodeDetail.Servers)-1 {
					m.selectedIndex++
				}
			case animeInfo:
				if m.anime != nil && m.selectedIndex < len(m.anime.Episodes)-1 {
					m.selectedIndex++
				}
			}
		case "pgdown":
			if m.currentStage == animeInfo && m.anime != nil {
				maxPage := (len(m.anime.Episodes) - 1) / m.animeEpisodesPerPage
				if m.currentAnimeEpisodePage < maxPage {
					m.currentAnimeEpisodePage++
					// No resetear el selectedIndex para mantener la posición
				}
			}
		case "left":
			if m.currentStage == animeInfo && m.currentAnimeEpisodePage > 0 {
				m.currentAnimeEpisodePage--
				// No resetear el selectedIndex para mantener la posición
			}
		case "right", "l":
			if m.currentStage == animeInfo && m.anime != nil {
				maxPage := (len(m.anime.Episodes) - 1) / m.animeEpisodesPerPage
				if m.currentAnimeEpisodePage < maxPage {
					m.currentAnimeEpisodePage++
					// No resetear el selectedIndex para mantener la posición
				}
			}
		}

		// Handle updates for sub-models based on current stage
		switch m.currentStage {
		case episodeList:
			m.list, cmd = m.list.Update(msg)
			cmds = append(cmds, cmd)
		case search:
			m.searchInput, cmd = m.searchInput.Update(msg)
			cmds = append(cmds, cmd)
		}

		return m, tea.Batch(cmds...)

	case episodesMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "Error al cargar episodios"
		} else {
			m.episodes = msg.episodes
			items := make([]list.Item, len(m.episodes))
			for i, ep := range m.episodes {
				items[i] = Item{TitleValue: ep.Title, DescriptionValue: ep.Episode}
			}
			m.list = NewList(items, "Latest Episodes", 20, 14)
			m.status = "Selecciona un episodio:"
		}
		return m, nil

	case episodeDetailMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "Error al cargar episodio"
		} else {
			m.episodeDetail = msg.episode
			m.status = "Detalle del episodio:"
		}
		return m, nil

	case animeMsg:
		if msg.err != nil {
			fmt.Println("DEBUG: Error fetching anime:", msg.err)
			m.err = msg.err
			m.status = "Error al cargar la información del anime"
		} else {
			m.anime = msg.anime
			m.status = "Información del anime:"
		}
		return m, nil

	case searchMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "Error al buscar"
		} else {
			var episodes []extractors.LatestEpisode
			for _, result := range msg.results {
				episodes = append(episodes, extractors.LatestEpisode{
					Title:   result.Url, // Use URL as Title for anime search results
					Image:   result.Image,
					Url:     result.Url,
					Episode: result.Type,
				})
			}
			m.episodes = episodes
			items := make([]list.Item, len(m.episodes))
			for i, ep := range m.episodes {
				items[i] = Item{TitleValue: ep.Title, DescriptionValue: ep.Episode}
			}
			m.list = NewList(items, "Search Results", 20, 14)
			m.currentStage = episodeList // Go back to episode list to display search results
			m.status = "Resultados de búsqueda:"
		}
		return m, nil
	case playVideoMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = errorStyle.Render(fmt.Sprintf("Error playing video: %v", msg.err))
		} else {
			m.status = "Video playback finished."
		}
		return m, nil
	}
	return m, nil
}
func (m *model) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	// Special case for source selection - show only centered provider selection
	if m.currentStage == sourceSelection {
		var s strings.Builder
		s.WriteString(fmt.Sprintf("%s\n\n", TitleStyle.Render("Kujira CLI")))
		options := []string{"AnimeFLV", "TioAnime"}
		for i, opt := range options {
			style := inactiveItemStyle
			cursor := "  "
			if m.selectedIndex == i {
				style = activeItemStyle
				cursor = "> "
			}
			line := fmt.Sprintf("%s%-15s", cursor, opt)
			s.WriteString(style.Render(line) + "\n")
		}
		s.WriteString("\nUsa Up/Down y Enter para seleccionar. 'q' para salir.")

		// Calculate available height considering frame size
		_, v := AppStyle.GetFrameSize()
		availableHeight := m.viewport.Height + m.list.Height() - v

		// Center the content with proper padding
		content := lipgloss.NewStyle().
			PaddingTop((availableHeight - 8) / 2). // 8 es la altura aproximada del contenido
			PaddingBottom(1).
			Render(boxStyle.Render(s.String()))

		return lipgloss.Place(
			m.viewport.Width+m.list.Width(),
			m.viewport.Height+m.list.Height(),
			lipgloss.Center,
			lipgloss.Center,
			content,
			lipgloss.WithWhitespaceChars(" "), // Evita caracteres especiales que puedan causar cortes
			lipgloss.WithWhitespaceForeground(lipgloss.Color("0")), // Color de fondo
		)
	}

	// Determine the content for the right column based on the current stage
	var rightColumnContent string
	switch m.currentStage {
	case sourceSelection:
		var s strings.Builder
		s.WriteString(fmt.Sprintf("%s\n\n", TitleStyle.Render("Kujira CLI")))
		options := []string{"AnimeFLV", "TioAnime"}
		for i, opt := range options {
			style := inactiveItemStyle
			cursor := "  "
			if m.selectedIndex == i {
				style = activeItemStyle
				cursor = "> "
			}
			// Usar un ancho fijo para alinear los textos
			line := fmt.Sprintf("%s%-15s", cursor, opt)
			s.WriteString(style.Render(line) + "\n")
		}
		s.WriteString("\nUsa Up/Down y Enter para seleccionar. 'q' para salir.")
		rightColumnContent = boxStyle.Render(s.String())

	case episodeDetail:
		if m.episodeDetail == nil {
			rightColumnContent = m.status
		} else {
			var s strings.Builder
			s.WriteString(TitleStyle.Render(fmt.Sprintf("Episode: %s (%s)", m.episodeDetail.Title, m.episodeDetail.Number)) + "\n\n")
			s.WriteString("Servers:\n")
			for i, srv := range m.episodeDetail.Servers {
				style := inactiveItemStyle
				cursor := "  "
				if m.selectedIndex == i {
					style = activeItemStyle
					cursor = "> "
				}
				line := fmt.Sprintf("%s%-10s: %s", cursor, srv.Name, srv.Url)
				s.WriteString(style.Render(line) + "\n")

			}
			rightColumnContent = boxStyle.Render(s.String() + fmt.Sprintf("%s", m.status) + "Enter: Play | I: Anime Info | Esc/H: Back | Q/Ctrl+C: Quit | Up/Down: Select Server")
		}

	case animeInfo:

		if m.anime == nil {
			rightColumnContent = m.status
		} else {
			var s strings.Builder
			s.WriteString(TitleStyle.Render(fmt.Sprintf("Anime: %s", m.anime.Title)) + "\n\n")
			s.WriteString(fmt.Sprintf("%s: %s\n", lipgloss.NewStyle().Width(20).Render("Alternative Title"), m.anime.TitleAlternative))
			s.WriteString(fmt.Sprintf("%s: %v\n", lipgloss.NewStyle().Width(20).Render("Status"), m.anime.Status))
			s.WriteString(fmt.Sprintf("%s: %s\n", lipgloss.NewStyle().Width(20).Render("Synopsis"), m.anime.Synopsis))
			s.WriteString(fmt.Sprintf("%s: %s\n", lipgloss.NewStyle().Width(20).Render("Genres"), strings.Join(m.anime.Genres, ", ")))
			s.WriteString("\nEpisodes:\n")

			// Reset pagination if episodes changed
			if len(m.anime.Episodes) == 0 {
				s.WriteString("No episodes available")
				rightColumnContent = boxStyle.Render(s.String())
				break
			}

			startIndex := m.currentAnimeEpisodePage * m.animeEpisodesPerPage
			endIndex := startIndex + m.animeEpisodesPerPage
			if endIndex > len(m.anime.Episodes) {
				endIndex = len(m.anime.Episodes)
			}

			// Reset selectedIndex and page if out of bounds
			if startIndex >= len(m.anime.Episodes) {
				m.currentAnimeEpisodePage = 0
				startIndex = 0
				endIndex = min(m.animeEpisodesPerPage, len(m.anime.Episodes))
			}

			if m.selectedIndex >= endIndex-startIndex {
				m.selectedIndex = 0
			}

			for i, ep := range m.anime.Episodes[startIndex:endIndex] {
				itemStyle := inactiveItemStyle
				cursor := "  "
				if m.selectedIndex == i {
					itemStyle = activeItemStyle
					cursor = "> "
				}
				line := fmt.Sprintf("%s%s", cursor, ep.Name)
				s.WriteString(itemStyle.Render(line) + "\n")
			}

			// Pagination info with navigation controls
			numPages := (len(m.anime.Episodes) + m.animeEpisodesPerPage - 1) / m.animeEpisodesPerPage
			if numPages > 1 {
				s.WriteString(fmt.Sprintf("\nPage %d/%d (←/→ to navigate)", m.currentAnimeEpisodePage+1, numPages))
			}

			// Add footer with controls
			footer := "\n\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
				"Enter: View Episode | Esc/H: Back | Q/Ctrl+C: Quit | "+
					"Up/Down: Select Episode | ←/→: Change Page")

			rightColumnContent = boxStyle.Render(s.String() + footer)
		}

	case search:
		rightColumnContent = TitleStyle.Render("Search") + "\n\n" + m.searchInput.View() + "\n" + fmt.Sprintf("\n%s\n", m.status) + "Enter: Search | Esc: Cancel | Q/Ctrl+C: Quit"

	case episodeList:
		// This case already handles the two-column layout
		if len(m.episodes) == 0 {
			// Calculate available height considering frame size
			_, v := AppStyle.GetFrameSize()
			availableHeight := m.viewport.Height + m.list.Height() - v

			// Show centered message with dynamic padding
			content := lipgloss.NewStyle().
				PaddingTop((availableHeight - 3) / 2). // 3 lines of content (message + padding)
				PaddingBottom(1).
				Render(boxStyle.Render("Cargando episodios..."))

			return lipgloss.Place(
				m.viewport.Width+m.list.Width(),
				m.viewport.Height+m.list.Height(),
				lipgloss.Center,
				lipgloss.Center,
				content,
				lipgloss.WithWhitespaceChars(" "),
				lipgloss.WithWhitespaceForeground(lipgloss.Color("0")),
			)
		}
		// Default two-column layout when episodes are loaded
		return lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), m.viewport.View()) +
			fmt.Sprintf("\n%s\n", m.status) +
			"Enter: Select Episode | S: Search | Q/Ctrl+C: Quit"
	}

	// Combine the left column (list view) and the right column content
	return lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), rightColumnContent)
}

type episodesMsg struct {
	episodes []extractors.LatestEpisode
	err      error
}

type episodeDetailMsg struct {
	episode *extractors.Episode
	err     error
}

type animeMsg struct {
	anime *extractors.Anime
	err   error
}

type searchMsg struct {
	results []extractors.SearchResult
	err     error
}

type playVideoMsg struct {
	err error
}

func (m *model) fetchEpisodes() tea.Cmd {
	return func() tea.Msg {
		episodes, err := extractors.FetchLatestEpisodes(m.selectedSrc.String())
		if err != nil {
			return episodesMsg{err: err}
		}
		return episodesMsg{episodes: episodes}
	}
}

func (m *model) fetchEpisodeDetail() tea.Cmd {
	if m.selectedEpisode == nil {
		return nil
	}
	episodeID := extractEpisodeID(m.selectedEpisode.Url)
	return func() tea.Msg {
		episode, err := extractors.FetchEpisodeDetail(m.selectedSrc.String(), episodeID)
		if err != nil {
			return episodeDetailMsg{err: err}
		}
		return episodeDetailMsg{episode: episode}
	}
}

func (m *model) fetchAnime() tea.Cmd {
	if m.selectedEpisode == nil {
		return nil
	}
	animeID := extractAnimeID(m.selectedEpisode.Url)
	fmt.Println("DEBUG: Anime ID extracted:", animeID)
	return func() tea.Msg {
		anime, err := extractors.FetchAnime(m.selectedSrc.String(), animeID)
		if err != nil {
			return animeMsg{err: err}
		}
		return animeMsg{anime: anime}
	}
}

func (m *model) fetchSearch(query string) tea.Cmd {
	return func() tea.Msg {
		results, err := extractors.FetchSearch(m.selectedSrc.String(), query)
		if err != nil {
			return searchMsg{err: err}
		}
		return searchMsg{results: results}
	}
}

func (m *model) playVideo(serverName, serverURL, sourceURL string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch strings.ToLower(serverName) {
		case "voe":
			var videoURL string
			extractor := extractors.Voe
			err := extractor.GetURL(
				serverURL,
				extractor.MainURL(),
				func(sub extractors.SubtitleFile) {},
				func(link *extractors.ExtractorLink) {
					if videoURL == "" && strings.Contains(link.Name, "MP4") {
						videoURL = link.URL
					}
				},
			)
			if err != nil {
				return playVideoMsg{err: fmt.Errorf("error with Voe extractor: %v", err)}
			}
			if videoURL == "" {
				return playVideoMsg{err: fmt.Errorf("could not extract Voe link")}
			}
			cmd = exec.Command("mpv", videoURL)
		case "stape":
			videoURL, err := extractors.ExtractStreamTapeUrl(serverURL)
			if err != nil {
				return playVideoMsg{err: fmt.Errorf("error extracting StreamTape URL: %v", err)}
			}
			cmd = exec.Command("mpv", videoURL)
		default:
			if strings.Contains(strings.ToLower(serverURL), "yourupload") {
				referer := fmt.Sprintf("Referer: %s", serverURL)
				cmd = exec.Command("mpv", "--http-header-fields="+referer, sourceURL)
			} else {
				cmd = exec.Command("mpv", sourceURL)
			}
		}

		if cmd == nil {
			return playVideoMsg{err: fmt.Errorf("no valid player command for server %s", serverName)}
		}

		err := cmd.Run()
		if err != nil {
			return playVideoMsg{err: fmt.Errorf("error executing mpv: %v", err)}
		}
		return playVideoMsg{err: nil}
	}
}

func extractEpisodeID(url string) string {
	parts := strings.Split(url, "/")
	return parts[len(parts)-1]
}

func extractAnimeID(url string) string {
	// If it's already an anime slug (e.g., from search results)
	if strings.Contains(url, "/anime/") {
		parts := strings.Split(url, "/anime/")
		if len(parts) > 1 {
			return parts[1]
		}
	}

	// For episode URLs (e.g., https://animeflv.net/ver/naruto-shippuden-123)
	// Extract the last segment (e.g., "naruto-shippuden-123")
	parts := strings.Split(url, "/")
	episodeSlug := parts[len(parts)-1]

	// Find the last hyphen, which typically separates the anime slug from the episode number
	lastHyphenIndex := strings.LastIndex(episodeSlug, "-")
	if lastHyphenIndex != -1 {
		// Return the part before the last hyphen
		return episodeSlug[:lastHyphenIndex]
	}

	// If no hyphen is found, assume the whole slug is the anime ID (e.g., for movies or specials)
	return episodeSlug
}
