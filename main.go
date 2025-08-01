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
	"github.com/spf13/cobra"
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
	providers := []string{"animeflv", "tioanime"}
	prompt := &survey.Select{
		Message: "Selecciona el proveedor de anime:",
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
	// Convertir URL de episodio a URL de anime
	animeURL = strings.ReplaceAll(animeURL, "/episode/", "/anime/")
	animeURL = regexp.MustCompile(`-\d+$`).ReplaceAllString(animeURL, "")

	fmt.Println("Cargando información del anime desde:", animeURL)
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

	// Aquí irían los extractores específicos
	if strings.Contains(strings.ToLower(server.Name), "voe") {
		extractor := extractors.Voe
		err := extractor.GetURL(
			server.URL,
			extractor.MainURL(),
			func(sub extractors.SubtitleFile) {},
			func(link *extractors.ExtractorLink) {
				if videoURL == "" && strings.Contains(link.Name, "MP4") {
					videoURL = link.URL
				}
			},
		)
		if err != nil {
			return fmt.Errorf("error with Voe extractor: %v", err)
		}
		if videoURL == "" {
			return fmt.Errorf("could not extract Voe link")
		}
		// fmt.Println("VOE extractor no implementado en esta versión")
	}
	if server.Name == "Stape" {
		var err error
		videoURL, err = extractors.ExtractStreamTapeUrl(server.URL)
		if err != nil {
			return fmt.Errorf("error extracting StreamTape URL: %v", err)
		}
	}

	cmd := exec.Command("mpv",
		"--force-window=immediate",
		fmt.Sprintf("--http-header-fields=Referer: %s", server.URL),
		videoURL)

	return cmd.Start()
}

func (c *CLI) mainMenu() error {
	for {
		clearScreen()
		fmt.Printf("🎌 Anime CLI - Proveedor: %s\n\n", c.provider)

		var choice string
		prompt := &survey.Select{
			Message: "¿Qué deseas hacer?",
			Options: []string{
				"Ver últimos animes",
				"Buscar anime",
				"Cambiar proveedor",
				"Salir",
			},
		}

		err := survey.AskOne(prompt, &choice)
		if err != nil {
			return err
		}

		switch choice {
		case "Ver últimos animes":
			if err := c.showLatestAnime(); err != nil {
				clearScreen()
				fmt.Printf("❌ Error: %v\n", err)
				fmt.Println("Presiona Enter para continuar...")
				fmt.Scanln()
			}
		case "Buscar anime":
			if err := c.searchMenu(); err != nil {
				clearScreen()
				fmt.Printf("❌ Error: %v\n", err)
				fmt.Println("Presiona Enter para continuar...")
				fmt.Scanln()
			}
		case "Cambiar proveedor":
			if err := c.selectProvider(); err != nil {
				return err
			}
		case "Salir":
			clearScreen()
			fmt.Println("👋 ¡Hasta luego!")
			return nil
		}
	}
}

func (c *CLI) showLatestAnime() error {
	clearScreen()
	fmt.Printf("📡 Cargando últimos animes de %s...\n", c.provider)

	animeList, err := c.getLatestAnime()
	if err != nil {
		return err
	}

	for {
		clearScreen()
		fmt.Printf("📺 Últimos animes de %s\n\n", c.provider)

		options := make([]string, len(animeList)+1)
		for i, anime := range animeList {
			options[i] = fmt.Sprintf("%s - Episodio %s", anime.Title, anime.Episode)
		}
		options[len(animeList)] = "← Volver al menú principal"

		var choice string
		prompt := &survey.Select{
			Message:  "Selecciona un anime:",
			Options:  options,
			PageSize: 15,
		}

		err := survey.AskOne(prompt, &choice)
		if err != nil {
			return err
		}

		if strings.Contains(choice, "← Volver") {
			return nil
		}

		// Encontrar el anime seleccionado
		for _, anime := range animeList {
			expected := fmt.Sprintf("%s - Episodio %s", anime.Title, anime.Episode)
			if choice == expected {
				if err := c.showEpisodeDetails(anime); err != nil {
					clearScreen()
					fmt.Printf("❌ Error: %v\n", err)
					fmt.Println("Presiona Enter para continuar...")
					fmt.Scanln()
				}
				break
			}
		}
	}
}

func (c *CLI) searchMenu() error {
	clearScreen()
	fmt.Println("🔍 Búsqueda de Anime")

	var query string
	prompt := &survey.Input{
		Message: "Ingresa el nombre del anime a buscar:",
	}

	err := survey.AskOne(prompt, &query)
	if err != nil {
		return err
	}

	clearScreen()
	fmt.Printf("🔄 Buscando '%s'...\n", query)

	results, err := c.searchAnime(query)
	if err != nil {
		return err
	}

	if len(results) == 0 {
		clearScreen()
		fmt.Println("❌ No se encontraron resultados")
		fmt.Println("Presiona Enter para continuar...")
		fmt.Scanln()
		return nil
	}

	clearScreen()
	fmt.Printf("🎯 Resultados para '%s'\n\n", query)

	options := make([]string, len(results)+1)
	for i, result := range results {
		options[i] = fmt.Sprintf("%s (%s)", result.Name, result.Type)
	}
	options[len(results)] = "← Volver"

	var choice string
	selectPrompt := &survey.Select{
		Message:  "Selecciona un anime:",
		Options:  options,
		PageSize: 15,
	}

	err = survey.AskOne(selectPrompt, &choice)
	if err != nil {
		return err
	}

	if strings.Contains(choice, "← Volver") {
		return nil
	}

	// Encontrar resultado seleccionado
	for _, result := range results {
		expected := fmt.Sprintf("%s (%s)", result.Name, result.Type)
		if choice == expected {
			return c.showAnimeInfo(result.URL)
		}
	}

	return nil
}

func (c *CLI) showEpisodeDetails(anime AnimeItem) error {
	clearScreen()
	fmt.Printf("📡 Cargando servidores para: %s\n", anime.Title)

	details, err := c.getEpisodeDetails(anime.URL)
	if err != nil {
		return err
	}

	for {
		clearScreen()
		fmt.Printf("🎬 %s\n", details.Title)
		fmt.Printf("📺 Episodio: %s\n\n", details.NumberEpisode)

		allowedServers := []string{"stape", "streamtape", "voe", "yourupload"}
		var serverOptions []string
		var validServers []Server

		for _, server := range details.Servers {
			isAllowed := false
			for _, allowed := range allowedServers {
				if strings.Contains(strings.ToLower(server.Name), allowed) {
					isAllowed = true
					break
				}
			}

			if isAllowed {
				serverOptions = append(serverOptions, fmt.Sprintf("▶️  %s", server.Name))
				validServers = append(validServers, server)
			} else {
				serverOptions = append(serverOptions, fmt.Sprintf("❌ %s (No disponible)", server.Name))
			}
		}

		serverOptions = append(serverOptions, "ℹ️  Ver información del anime")
		serverOptions = append(serverOptions, "← Volver")

		var choice string
		prompt := &survey.Select{
			Message: "Selecciona un servidor:",
			Options: serverOptions,
		}

		err := survey.AskOne(prompt, &choice)
		if err != nil {
			return err
		}

		if strings.Contains(choice, "← Volver") {
			return nil
		}

		if strings.Contains(choice, "Ver información del anime") {
			if err := c.showAnimeInfo(anime.URL); err != nil {
				clearScreen()
				fmt.Printf("❌ Error: %v\n", err)
				fmt.Println("Presiona Enter para continuar...")
				fmt.Scanln()
			}
			continue
		}

		if strings.HasPrefix(choice, "❌") {
			clearScreen()
			fmt.Println("⚠️  Servidor no disponible")
			fmt.Println("Presiona Enter para continuar...")
			fmt.Scanln()
			continue
		}

		// Encontrar el servidor seleccionado
		for _, server := range validServers {
			expected := fmt.Sprintf("▶️  %s", server.Name)
			if choice == expected {
				clearScreen()
				fmt.Printf("🚀 Reproduciendo en %s...\n", server.Name)
				if err := c.playVideo(server); err != nil {
					fmt.Printf("❌ Error al reproducir: %v\n", err)
					fmt.Println("Presiona Enter para continuar...")
					fmt.Scanln()
				} else {
					fmt.Println("✅ Video iniciado correctamente")
					fmt.Println("Presiona Enter para continuar...")
					fmt.Scanln()
				}
				break
			}
		}
	}
}

func (c *CLI) showAnimeInfo(animeURL string) error {
	clearScreen()
	fmt.Println("📡 Cargando información del anime...")

	info, err := c.getAnimeInfo(animeURL)
	if err != nil {
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
		// Mostrar información del anime en la parte superior
		fmt.Printf("📺 %s\n", animeInfo.Title)
		if animeInfo.TitleAlt != animeInfo.Title {
			fmt.Printf("📝 Título alternativo: %s\n", animeInfo.TitleAlt)
		}
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
		fmt.Printf("📊 Estado: %s\n", status)
		fmt.Printf("🎭 Géneros: %s\n", strings.Join(animeInfo.Genres, ", "))
		fmt.Printf("\n📖 Sinopsis:\n%s\n\n", animeInfo.Synopsis)

		// Mostrar lista de episodios
		fmt.Printf("📺 Lista de Episodios\n")
		fmt.Printf("📄 Página %d de %d\n\n", currentPage+1, totalPages)

		start := currentPage * itemsPerPage
		end := start + itemsPerPage
		if end > len(episodes) {
			end = len(episodes)
		}

		currentEpisodes := episodes[start:end]
		options := make([]string, len(currentEpisodes))

		for i, ep := range currentEpisodes {
			options[i] = fmt.Sprintf("📺 Episodio %d: %s", ep.Num, ep.Name)
		}

		// Agregar opciones de navegación
		if currentPage > 0 {
			options = append(options, "⬅️  Página anterior")
		}
		if end < len(episodes) {
			options = append(options, "➡️  Página siguiente")
		}
		options = append(options, "← Volver")

		var choice string
		prompt := &survey.Select{
			Message:  "Selecciona un episodio:",
			Options:  options,
			PageSize: 15,
		}

		err := survey.AskOne(prompt, &choice)
		if err != nil {
			return err
		}

		if choice == "← Volver" {
			return nil
		}

		if choice == "⬅️  Página anterior" {
			currentPage--
			continue
		}

		if choice == "➡️  Página siguiente" {
			currentPage++
			continue
		}

		// Encontrar episodio seleccionado
		for _, ep := range currentEpisodes {
			expected := fmt.Sprintf("📺 Episodio %d: %s", ep.Num, ep.Name)
			if choice == expected {
				animeItem := AnimeItem{URL: ep.URL, Title: ep.Name}
				if err := c.showEpisodeDetails(animeItem); err != nil {
					clearScreen()
					fmt.Printf("❌ Error: %v\n", err)
					fmt.Println("Presiona Enter para continuar...")
					fmt.Scanln()
				}
				break
			}
		}
	}
}
func main() {
	cli := NewCLI()

	var rootCmd = &cobra.Command{
		Use:   "anime-cli",
		Short: "CLI interactivo para ver anime",
		Long:  "Un cliente de línea de comandos para navegar y reproducir anime desde diferentes proveedores",
		Run: func(cmd *cobra.Command, args []string) {
			clearScreen()
			fmt.Println("🎌 Bienvenido al CLI de Anime")
			fmt.Println()

			if err := cli.selectProvider(); err != nil {
				log.Fatal(err)
			}

			clearScreen()
			fmt.Printf("✅ Proveedor seleccionado: %s\n", cli.provider)
			// fmt.Println("Presiona Enter para continuar...")
			// fmt.Scanln()

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
			clearScreen()
			query := args[0]
			provider, _ := cmd.Flags().GetString("provider")

			if provider == "" {
				if err := cli.selectProvider(); err != nil {
					log.Fatal(err)
				}
			} else {
				cli.provider = provider
			}

			clearScreen()
			fmt.Printf("🔍 Buscando '%s' en %s...\n\n", query, cli.provider)

			results, err := cli.searchAnime(query)
			if err != nil {
				log.Fatal(err)
			}

			if len(results) == 0 {
				fmt.Println("❌ No se encontraron resultados")
				return
			}

			fmt.Printf("🎯 Resultados encontrados:\n\n")
			for i, result := range results {
				fmt.Printf("%d. %s (%s)\n", i+1, result.Name, result.Type)
			}
		},
	}

	searchCmd.Flags().StringP("provider", "p", "", "Proveedor de anime (animeflv, tioanime)")
	rootCmd.AddCommand(searchCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
