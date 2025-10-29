package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"kujira_cli/extractors"

	"github.com/AlecAivazis/survey/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// --- STYLES ---
var (
	appStyle = lipgloss.NewStyle().Padding(1, 2)

	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#874BFD")).
			Bold(true).
			Padding(0, 1, 0, 1)

	infoStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("248"))

	// Estilos para las listas
	listHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")).
			PaddingBottom(1)

	listDoneStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("40")).
			SetString("✓")

	listBulletStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("63")).
			SetString("•")

	listEpisodeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("214")).
				Bold(true)

	listAnimeTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	listNavStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("33"))

	// Estilos para servidores
	serverAvailableStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#32CD32"))
	serverUnavailableStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000")).
			Bold(true)

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2)

	synopsisStyle = lipgloss.NewStyle().
			Width(80).
			Padding(1)
)

type AnimeItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Img     string `json:"img"`
	Episode string `json:"episode"`
}

type Server struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Source string `json:"source"`
}

type EpisodeDetails struct {
	Title         string   `json:"title"`
	NumberEpisode string   `json:"numberEpisode"`
	Servers       []Server `json:"servers"`
}

type Episode struct {
	Name      string `json:"name"`
	Thumbnail string `json:"thumbnail"`
	URL       string `json:"url"`
	Num       int    `json:"num"`
}

type Related struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Img   string `json:"img"`
}

type AnimeInfo struct {
	Title    string      `json:"title"`
	TitleAlt string      `json:"title_alt"`
	Img      string      `json:"img"`
	Status   interface{} `json:"status"`
	Synopsis string      `json:"synopsis"`
	Genres   []string    `json:"genres"`
	Episodes []Episode   `json:"episodes"`
	Related  []Related   `json:"related"`
}

type SearchResult struct {
	Name  string `json:"name"`
	Image string `json:"image"`
	URL   string `json:"url"`
	Type  string `json:"type"`
}

type CLI struct {
	baseURL  string
	client   *http.Client
	provider string
}

func NewCLI() *CLI {
	return &CLI{
		baseURL: "https://kujira.unusualoticramo.workers.dev",
		client:  &http.Client{},
	}
}

func clearScreen() {
	cmd := exec.Command("clear")
	if strings.Contains(strings.ToLower(os.Getenv("OS")), "windows") {
		cmd = exec.Command("cmd", "/c", "cls")
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func (c *CLI) makeRequest(endpoint string) ([]byte, error) {
	req, err := http.NewRequest("GET", c.baseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (c *CLI) selectProvider() error {
	clearScreen()
	providers := []string{"animeflv", "tioanime","animeav1","monoschinos"}
	prompt := &survey.Select{
		Message: titleStyle.Render("Selecciona el proveedor de anime:"),
		Options: providers,
	}
	return survey.AskOne(prompt, &c.provider)
}

func (c *CLI) getLatestAnime() ([]AnimeItem, error) {
	data, err := c.makeRequest(fmt.Sprintf("/%s/latest", c.provider))
	if err != nil {
		return nil, err
	}

	var animeList []AnimeItem
	err = json.Unmarshal(data, &animeList)
	return animeList, err
}

func (c *CLI) searchAnime(query string) ([]SearchResult, error) {
	endpoint := fmt.Sprintf("/%s/search?title=%s", c.provider, url.QueryEscape(query))
	data, err := c.makeRequest(endpoint)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	err = json.Unmarshal(data, &results)
	return results, err
}

func (c *CLI) getEpisodeDetails(animeURL string) (*EpisodeDetails, error) {
	data, err := c.makeRequest(animeURL)
	if err != nil {
		return nil, err
	}

	var details EpisodeDetails
	err = json.Unmarshal(data, &details)
	return &details, err
}

func (c *CLI) getAnimeInfo(animeURL string) (*AnimeInfo, error) {
	if strings.Contains(animeURL, "/episode/") {
		animeURL = strings.ReplaceAll(animeURL, "/episode/", "/anime/")
		animeURL = regexp.MustCompile(`-\d+$`).ReplaceAllString(animeURL, "")
	}
	data, err := c.makeRequest(animeURL)
	if err != nil {
		return nil, err
	}

	var info AnimeInfo
	err = json.Unmarshal(data, &info)
	return &info, err
}

func (c *CLI) playVideo(server Server) error {
	videoURL := server.Source
	if strings.Contains(strings.ToLower(server.Name), "voe") {
		extractor := extractors.Voe
		err := extractor.GetURL(server.URL, extractor.MainURL(), func(sub extractors.SubtitleFile) {}, func(link *extractors.ExtractorLink) {
			if (videoURL == "" || videoURL == "unknown") && strings.Contains(link.Name, "MP4") {
				videoURL = link.URL
			}
		})
		if err != nil {
			return fmt.Errorf("error with Voe extractor: %v", err)
		}
		if videoURL == "" {
			return fmt.Errorf("could not extract Voe link")
		}
	} else if server.Name == "Stape" || server.Name == "StreamTape" {
		var err error
		videoURL, err = extractors.ExtractStreamTapeUrl(server.URL)
		if err != nil {
			return fmt.Errorf("error extracting StreamTape URL: %v", err)
		}
	}

	cmd := exec.Command("mpv", "--force-window=immediate", fmt.Sprintf("--http-header-fields=Referer: %s", server.URL), videoURL)
	return cmd.Start()
}

func (c *CLI) mainMenu() error {
	for {
		clearScreen()
		mainTitle := titleStyle.Render("🎌 Kujira CLI")
		providerText := infoStyle.Render(fmt.Sprintf("Proveedor actual: %s", c.provider))
		fmt.Println(appStyle.Render(lipgloss.JoinVertical(lipgloss.Center, mainTitle, providerText)))

		var choice string
		prompt := &survey.Select{
			Message: "¿Qué deseas hacer?",
			Options: []string{"Ver últimos animes", "Buscar anime", "Cambiar proveedor", "Salir"},
		}
		survey.AskOne(prompt, &choice)

		switch choice {
		case "Ver últimos animes":
			c.showLatestAnime()
		case "Buscar anime":
			c.searchMenu()
		case "Cambiar proveedor":
			if err := c.selectProvider(); err != nil {
				return err
			}
		case "Salir":
			clearScreen()
			fmt.Println(infoStyle.Render("👋 ¡Hasta luego!"))
			return nil
		}
	}
}

func (c *CLI) showLatestAnime() error {
	clearScreen()
	fmt.Println(infoStyle.Render(fmt.Sprintf("📡 Cargando últimos animes de %s...", c.provider)))
	animeList, err := c.getLatestAnime()
	if err != nil {
		fmt.Println(errorStyle.Render("Error:"), err)
		return err
	}

	for {
		clearScreen()
		options := make([]string, len(animeList)+1)
		// Usamos un mapa para poder recuperar el objeto AnimeItem después de la selección
		optionMap := make(map[string]AnimeItem)

		for _, anime := range animeList {
			optionKey := fmt.Sprintf("%s %s - %s",
				listBulletStyle.String(),
				listEpisodeStyle.Render("Episodio "+anime.Episode),
				listAnimeTitleStyle.Render(anime.Title),
			)
			options[len(optionMap)] = optionKey
			optionMap[optionKey] = anime
		}
		options[len(animeList)] = listNavStyle.Render("← Volver al menú principal")

		var choice string
		prompt := &survey.Select{
			Message:  titleStyle.Render("Últimos Animes"),
			Options:  options,
			PageSize: 15,
		}
		survey.AskOne(prompt, &choice)

		if strings.Contains(choice, "Volver") {
			return nil
		}

		if selectedAnime, ok := optionMap[choice]; ok {
			c.showEpisodeDetails(selectedAnime)
		}
	}
}

func (c *CLI) searchMenu() error {
	clearScreen()
	var query string
	prompt := &survey.Input{
		Message: titleStyle.Render("🔍 Ingresa el nombre del anime a buscar:"),
	}
	survey.AskOne(prompt, &query)

	clearScreen()
	fmt.Println(infoStyle.Render(fmt.Sprintf("🔄 Buscando '%s'...", query)))
	results, err := c.searchAnime(query)
	if err != nil {
		fmt.Println(errorStyle.Render("Error:"), err)
		return err
	}

	if len(results) == 0 {
		clearScreen()
		fmt.Println(boxStyle.Render(errorStyle.Render("No se encontraron resultados para '") + query + errorStyle.Render("'")))
		fmt.Println(infoStyle.Render("Presiona Enter para continuar..."))
		fmt.Scanln()
		return nil
	}

	clearScreen()
	options := make([]string, len(results)+1)
	optionMap := make(map[string]SearchResult)

	for _, result := range results {
		optionKey := fmt.Sprintf("%s %s %s",
			listBulletStyle.String(),
			listAnimeTitleStyle.Render(result.Name),
			infoStyle.Render("("+result.Type+")"),
		)
		options[len(optionMap)] = optionKey
		optionMap[optionKey] = result
	}
	options[len(results)] = listNavStyle.Render("← Volver")

	var choice string
	selectPrompt := &survey.Select{
		Message:  titleStyle.Render(fmt.Sprintf("Resultados para '%s'", query)),
		Options:  options,
		PageSize: 15,
	}
	survey.AskOne(selectPrompt, &choice)

	if strings.Contains(choice, "Volver") {
		return nil
	}

	if selectedResult, ok := optionMap[choice]; ok {
		return c.showAnimeInfo(selectedResult.URL)
	}
	return nil
}

func (c *CLI) showEpisodeDetails(anime AnimeItem) error {
	clearScreen()
	fmt.Println(infoStyle.Render(fmt.Sprintf("📡 Cargando servidores para: %s", anime.Title)))
	details, err := c.getEpisodeDetails(anime.URL)
	if err != nil {
		fmt.Println(errorStyle.Render("Error:"), err)
		return err
	}

	for {
		clearScreen()
		var serverOptions []string
		var validServers []Server

		allowedServers := []string{"stape", "streamtape", "voe", "yourupload"}
		for _, server := range details.Servers {
			isAllowed := false
			for _, allowed := range allowedServers {
				if strings.Contains(strings.ToLower(server.Name), allowed) {
					isAllowed = true
					break
				}
			}
			if isAllowed {
				serverOptions = append(serverOptions, serverAvailableStyle.Render(fmt.Sprintf("▶️  %s", server.Name)))
				validServers = append(validServers, server)
			} else {
				serverOptions = append(serverOptions, serverUnavailableStyle.Render(fmt.Sprintf("❌ %s (No disponible)", server.Name)))
			}
		}

		serverOptions = append(serverOptions, listNavStyle.Render("ℹ️  Ver información del anime"), listNavStyle.Render("← Volver"))

		var choice string
		prompt := &survey.Select{
			Message:  titleStyle.Render(fmt.Sprintf("%s - %s", details.Title, details.NumberEpisode)),
			Help:     "Selecciona un servidor para reproducir",
			Options:  serverOptions,
			PageSize: 10,
		}
		survey.AskOne(prompt, &choice)

		switch choice {
		case listNavStyle.Render("← Volver"):
			return nil
		case listNavStyle.Render("ℹ️  Ver información del anime"):
			c.showAnimeInfo(anime.URL)
			continue
		}

		if strings.Contains(choice, "(No disponible)") {
			clearScreen()
			fmt.Println(boxStyle.Render(errorStyle.Render("Servidor no disponible. Por favor, elige otro.")))
			fmt.Println(infoStyle.Render("Presiona Enter para continuar..."))
			fmt.Scanln()
			continue
		}

		for _, server := range validServers {
			expected := serverAvailableStyle.Render(fmt.Sprintf("▶️  %s", server.Name))
			if choice == expected {
				clearScreen()
				fmt.Println(infoStyle.Render(fmt.Sprintf("🚀 Reproduciendo en %s...", server.Name)))
				if err := c.playVideo(server); err != nil {
					fmt.Println(errorStyle.Render("Error al reproducir:"), err)
				} else {
					fmt.Println(infoStyle.Render("✅ Video iniciado correctamente."))
				}
				fmt.Println(infoStyle.Render("Presiona Enter para continuar..."))
				fmt.Scanln()
				break
			}
		}
	}
}

func (c *CLI) showAnimeInfo(animeURL string) error {
	clearScreen()
	fmt.Println(infoStyle.Render("📡 Cargando información del anime..."))
	info, err := c.getAnimeInfo(animeURL)
	if err != nil {
		fmt.Println(errorStyle.Render("Error:"), err)
		return err
	}

	return c.showEpisodesList(info.Episodes, info)
}

func (c *CLI) showEpisodesList(episodes []Episode, animeInfo *AnimeInfo) error {
	const itemsPerPage = 10
	currentPage := 0
	totalPages := (len(episodes) + itemsPerPage - 1) / itemsPerPage

	for {
		clearScreen()

		status := "Desconocido"
		switch v := animeInfo.Status.(type) {
		case bool:
			if v {
				status = "En emisión"
			} else {
				status = "Finalizado"
			}
		case string:
			status = v
		}

		infoBlock := lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Render(animeInfo.Title),
			infoStyle.Render("Estado: "+status),
			infoStyle.Render("Géneros: "+strings.Join(animeInfo.Genres, ", ")),
			synopsisStyle.Render(animeInfo.Synopsis),
		)

		fmt.Println(boxStyle.Render(infoBlock))

		start := currentPage * itemsPerPage
		end := start + itemsPerPage
		if end > len(episodes) {
			end = len(episodes)
		}

		currentEpisodes := episodes[start:end]
		options := make([]string, 0, itemsPerPage+3)
		optionMap := make(map[string]Episode)

		for _, ep := range currentEpisodes {
			optionKey := fmt.Sprintf("%s %s: %s",
				listBulletStyle.String(),
				listEpisodeStyle.Render(fmt.Sprintf("Episodio %d", ep.Num)),
				listAnimeTitleStyle.Render(ep.Name),
			)
			options = append(options, optionKey)
			optionMap[optionKey] = ep
		}

		navOptions := []string{}
		if currentPage > 0 {
			navOptions = append(navOptions, listNavStyle.Render("⬅️  Página anterior"))
		}
		if end < len(episodes) {
			navOptions = append(navOptions, listNavStyle.Render("➡️  Página siguiente"))
		}
		navOptions = append(navOptions, listNavStyle.Render("← Volver"))
		options = append(options, navOptions...)

		var choice string
		prompt := &survey.Select{
			Message:  fmt.Sprintf("Selecciona un episodio (Página %d de %d)", currentPage+1, totalPages),
			Options:  options,
			PageSize: itemsPerPage + len(navOptions),
		}
		survey.AskOne(prompt, &choice)

		switch choice {
		case listNavStyle.Render("← Volver"):
			return nil
		case listNavStyle.Render("⬅️  Página anterior"):
			if currentPage > 0 {
				currentPage--
			}
			continue
		case listNavStyle.Render("➡️  Página siguiente"):
			if end < len(episodes) {
				currentPage++
			}
			continue
		}

		if selectedEpisode, ok := optionMap[choice]; ok {
			animeItem := AnimeItem{URL: selectedEpisode.URL, Title: selectedEpisode.Name}
			c.showEpisodeDetails(animeItem)
		}
	}
}

func main() {
	cli := NewCLI()
	var rootCmd = &cobra.Command{
		Use:   "kujira-cli",
		Short: "CLI interactivo para ver anime",
		Long:  "Un cliente de línea de comandos para navegar y reproducir anime desde diferentes proveedores.",
		Run: func(cmd *cobra.Command, args []string) {
			if err := cli.selectProvider(); err != nil {
				log.Fatal(err)
			}
			if err := cli.mainMenu(); err != nil {
				log.Fatal(err)
			}
		},
	}

	var searchCmd = &cobra.Command{
		Use:   "search [query]",
		Short: "Buscar anime directamente",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			cli.provider, _ = cmd.Flags().GetString("provider")
			if cli.provider == "" {
				if err := cli.selectProvider(); err != nil {
					log.Fatal(err)
				}
			}
			cli.searchMenu()
		},
	}

	searchCmd.Flags().StringP("provider", "p", "", "Proveedor de anime (animeflv, tioanime, animeav1, monoschinos)")
	rootCmd.AddCommand(searchCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
