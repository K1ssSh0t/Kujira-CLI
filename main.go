package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"kujira_cli/extractors"
	"kujira_cli/sync"

	"github.com/AlecAivazis/survey/v2"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// Variables de versión que se pueden establecer en tiempo de compilación
var (
	// Versión de la aplicación
	version = "dev"
	// Fecha de compilación
	buildDate = "fecha-no-especificada"
	// Commit de Git
	gitCommit = "sin-commit"
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

	subHeaderStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	dubHeaderStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true)
	latHeaderStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true)
	unknownHeaderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Bold(true)

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

// --- DATA STRUCTURES ---

type AnimeItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Img     string `json:"img"`
	Episode string `json:"episode"`
}

type Server struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Source   string `json:"source"`
	Language string `json:"language"`
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

// --- CACHING ---

const cacheDuration = time.Hour * 1 // Cache for 1 hour

type CacheEntry struct {
	Timestamp time.Time
	Data      []byte
}

func getCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".kujira_cli", "cache"), nil
}

func ensureCacheDir() error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}
	return os.MkdirAll(cacheDir, 0755)
}

func getCacheKey(s string) string {
	h := sha1.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *CLI) readCache(key string) ([]byte, bool) {
	cacheDir, err := getCacheDir()
	if err != nil {
		return nil, false
	}
	filePath := filepath.Join(cacheDir, key)

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, false
	}

	var entry CacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, false
	}

	if time.Since(entry.Timestamp) > cacheDuration {
		return nil, false
	}

	return entry.Data, true
}

func (c *CLI) writeCache(key string, data []byte) error {
	cacheDir, err := getCacheDir()
	if err != nil {
		return err
	}
	filePath := filepath.Join(cacheDir, key)

	entry := CacheEntry{
		Timestamp: time.Now(),
		Data:      data,
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, jsonData, 0644)
}

// --- HISTORY ---

func getHistoryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, ".kujira_cli")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "history.json"), nil
}

func (c *CLI) ReadHistory() ([]sync.HistoryItem, error) {
	path, err := getHistoryPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []sync.HistoryItem{}, nil // Return empty slice if file doesn't exist
		}
		return nil, err
	}

	var history []sync.HistoryItem
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	return history, nil
}

func (c *CLI) addToHistory(item sync.HistoryItem) error {
	history, err := c.ReadHistory()
	if err != nil {
		return err
	}

	// Remove duplicates, keeping the latest one
	newHistory := []sync.HistoryItem{item}
	for _, h := range history {
		if h.URL != item.URL {
			newHistory = append(newHistory, h)
		}
	}

	// Optional: limit history size
	if len(newHistory) > 100 {
		newHistory = newHistory[:100]
	}

	// Sort by timestamp descending
	sort.Slice(newHistory, func(i, j int) bool {
		return newHistory[i].Timestamp.After(newHistory[j].Timestamp)
	})

	data, err := json.MarshalIndent(newHistory, "", "  ")
	if err != nil {
		return err
	}

	path, err := getHistoryPath()
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// --- LIBRARY ---

func getLibraryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	configDir := filepath.Join(home, ".kujira_cli")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return "", err
	}
	return filepath.Join(configDir, "library.json"), nil
}

func (c *CLI) ReadLibrary() ([]sync.LibraryItem, error) {
	path, err := getLibraryPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []sync.LibraryItem{}, nil
		}
		return nil, err
	}

	var library []sync.LibraryItem
	if err := json.Unmarshal(data, &library); err != nil {
		return nil, err
	}
	return library, nil
}

func (c *CLI) saveLibrary(library []sync.LibraryItem) error {
	data, err := json.MarshalIndent(library, "", "  ")
	if err != nil {
		return err
	}

	path, err := getLibraryPath()
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func (c *CLI) updateLibraryItem(item sync.LibraryItem) error {
	library, err := c.ReadLibrary()
	if err != nil {
		return err
	}

	// Check if item exists and update it, or add new
	found := false
	for i, libItem := range library {
		if libItem.URL == item.URL {
			library[i] = item
			found = true
			break
		}
	}

	if !found {
		library = append(library, item)
	}

	return c.saveLibrary(library)
}

func (c *CLI) removeFromLibrary(url string) error {
	library, err := c.ReadLibrary()
	if err != nil {
		return err
	}

	newLibrary := []sync.LibraryItem{}
	for _, item := range library {
		if item.URL != url {
			newLibrary = append(newLibrary, item)
		}
	}

	return c.saveLibrary(newLibrary)
}

func (c *CLI) getLibraryItem(url string) (*sync.LibraryItem, error) {
	library, err := c.ReadLibrary()
	if err != nil {
		return nil, err
	}

	for _, item := range library {
		if item.URL == url {
			return &item, nil
		}
	}
	return nil, nil
}

// --- API CALLS ---

func (c *CLI) makeRequest(endpoint string) ([]byte, error) {
	cacheKey := getCacheKey(c.provider + ":" + endpoint)
	if data, found := c.readCache(cacheKey); found {
		return data, nil
	}

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Do not cache empty or error responses
	if resp.StatusCode == http.StatusOK && len(body) > 2 { // len > 2 to avoid caching "{}" or "[]"
		c.writeCache(cacheKey, body)
	}

	return body, nil
}

func (c *CLI) selectProvider() error {
	clearScreen()
	providers := []string{"animeflv", "tioanime", "animeav1", "monoschinos"}
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

// --- VIDEO PLAYER ---

func (c *CLI) playVideo(server Server, details *EpisodeDetails, animeURL string) error {
	videoURL := server.Source
	referer := server.URL
	name := strings.ToLower(server.Name)
	switch {
	case strings.Contains(name, "voe"):
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
	case strings.Contains(name, "stape"), strings.Contains(name, "streamtape"):
		var err error
		videoURL, err = extractors.ExtractStreamTapeUrl(server.URL)
		if err != nil {
			return fmt.Errorf("error extracting StreamTape URL: %v", err)
		}
	case strings.Contains(name, "mp4upload"):
		var err error
		videoURL, referer, err = extractors.ExtractMp4UploadUrl(server.URL, server.URL)
		if err != nil {
			return fmt.Errorf("error extracting Mp4Upload URL: %v", err)
		}
	case strings.Contains(name, "uqload"):
		var err error
		videoURL, referer, err = extractors.ExtractUqloadUrl(server.URL, server.URL)
		if err != nil {
			return fmt.Errorf("error extracting Uqload URL: %v", err)
		}
	}

	cmd := exec.Command("mpv", "--force-window=immediate",
		fmt.Sprintf("--http-header-fields=Referer: %s", referer),
		fmt.Sprintf("--user-agent=%s", extractors.UserAgent),
		videoURL)
	err := cmd.Start()
	if err == nil {
		historyItem := sync.HistoryItem{
			Title:     details.Title,
			Episode:   details.NumberEpisode,
			URL:       animeURL,
			Provider:  c.provider,
			Timestamp: time.Now(),
		}
		if historyErr := c.addToHistory(historyItem); historyErr != nil {
			fmt.Println(errorStyle.Render("Advertencia: No se pudo guardar en el historial:"), historyErr)
		}

		// Auto-sync episode
		go func() {
			if err := c.autoSyncEpisode(details.Title, details.NumberEpisode); err != nil {
				// Errors are logged in the function
			}
		}()
	}
	return err
}

// --- UI MENUS ---

func (c *CLI) mainMenu() error {
	for {
		clearScreen()
		mainTitle := titleStyle.Render("🎌 Kujira CLI")
		providerText := infoStyle.Render(fmt.Sprintf("Proveedor actual: %s", c.provider))
		fmt.Println(appStyle.Render(lipgloss.JoinVertical(lipgloss.Center, mainTitle, providerText)))

		var choice string
		prompt := &survey.Select{
			Message: "¿Qué deseas hacer?",
			Options: []string{"Ver últimos animes", "Buscar anime", "Mi Biblioteca", "Historial", "Sincronizar con MAL", "Cambiar proveedor", "Salir"},
		}
		survey.AskOne(prompt, &choice)

		switch choice {
		case "Ver últimos animes":
			c.showLatestAnime()
		case "Buscar anime":
			c.searchMenu()
		case "Mi Biblioteca":
			c.showLibraryMenu()
		case "Historial":
			c.showHistoryMenu()
		case "Sincronizar con MAL":
			c.showSyncMenu()
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
		// Agrupar por Language: "" => SUB (compat), display con fallback Desconocido si viene explícito
		normalizeLang := func(s string) string {
			t := strings.TrimSpace(s)
			if t == "" {
				return "SUB"
			}
			return strings.ToUpper(t)
		}
		styleForLang := func(lang string) lipgloss.Style {
			switch lang {
			case "SUB":
				return subHeaderStyle
			case "DUB":
				return dubHeaderStyle
			case "LAT":
				return latHeaderStyle
			default:
				if lang == "Desconocido" {
					return unknownHeaderStyle
				}
				return infoStyle
			}
		}
		isAllowedServer := func(name string) bool {
			n := strings.ToLower(name)
			for _, a := range []string{"stape", "streamtape", "voe", "yourupload", "mp4upload", "uqload"} {
				if strings.Contains(n, a) {
					return true
				}
			}
			return false
		}

		grouped := map[string][]Server{}
		order := []string{}
		seenLang := map[string]bool{}
		for _, s := range details.Servers {
			lang := normalizeLang(s.Language)
			// Si API trae explícitamente "Desconocido" ya viene normalizado distinto; respetar
			if strings.TrimSpace(s.Language) == "" {
				lang = "SUB"
			}
			if s.Language != "" && strings.ToUpper(strings.TrimSpace(s.Language)) == "DESCONOCIDO" {
				lang = "Desconocido"
			}
			grouped[lang] = append(grouped[lang], s)
			if !seenLang[lang] {
				seenLang[lang] = true
				order = append(order, lang)
			}
		}
		// Orden estable: SUB, DUB, LAT primero
		priority := map[string]int{"SUB": 0, "DUB": 1, "LAT": 2, "Desconocido": 99}
		sort.Slice(order, func(i, j int) bool {
			pi, oi := priority[order[i]]
			pj, oj := priority[order[j]]
			if oi && oj {
				if pi != pj {
					return pi < pj
				}
				return order[i] < order[j]
			}
			if oi != oj {
				return oi
			}
			return order[i] < order[j]
		})

		var serverOptions []string
		validServersMap := map[string]Server{}
		var validServers []Server
		for _, lang := range order {
			servers := grouped[lang]
			validCount := 0
			for _, s := range servers {
				if isAllowedServer(s.Name) {
					validCount++
				}
			}
			header := styleForLang(lang).Render(fmt.Sprintf("%s — %d", lang, validCount))
			serverOptions = append(serverOptions, header)
			for _, s := range servers {
				label := fmt.Sprintf("▶️  %s [%s]", s.Name, lang)
				if isAllowedServer(s.Name) {
					opt := serverAvailableStyle.Render(label)
					serverOptions = append(serverOptions, opt)
					validServersMap[opt] = s
					validServers = append(validServers, s)
				} else {
					serverOptions = append(serverOptions, serverUnavailableStyle.Render(fmt.Sprintf("❌ %s [%s] (No disponible)", s.Name, lang)))
				}
			}
		}

		serverOptions = append(serverOptions, listNavStyle.Render("ℹ️  Ver información del anime"), listNavStyle.Render("← Volver"))

		var choice string
		prompt := &survey.Select{
			Message:  titleStyle.Render(fmt.Sprintf("%s - %s", details.Title, details.NumberEpisode)),
			Help:     "Selecciona un servidor para reproducir",
			Options:  serverOptions,
			PageSize: 15,
		}
		survey.AskOne(prompt, &choice)

		if strings.Contains(choice, "—") {
			continue
		}
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

		if server, ok := validServersMap[choice]; ok {
			_ = validServers
			clearScreen()
			fmt.Println(infoStyle.Render(fmt.Sprintf("🚀 Reproduciendo en %s...", server.Name)))
			if err := c.playVideo(server, details, anime.URL); err != nil {
				fmt.Println(errorStyle.Render("Error al reproducir:"), err)
			} else {
				fmt.Println(infoStyle.Render("✅ Video iniciado correctamente."))
			}
			fmt.Println(infoStyle.Render("Presiona Enter para continuar..."))
			fmt.Scanln()
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

	return c.showEpisodesList(info.Episodes, info, animeURL)
}

func (c *CLI) showEpisodesList(episodes []Episode, animeInfo *AnimeInfo, animeURL string) error {
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
		options := make([]string, 0, itemsPerPage+4)
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
		navOptions = append(navOptions, listNavStyle.Render("📚 Gestionar en biblioteca"))
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
		case listNavStyle.Render("📚 Gestionar en biblioteca"):
			c.manageLibraryStatus(animeInfo, animeURL)
			continue
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

func (c *CLI) showHistoryMenu() error {
	clearScreen()
	fmt.Println(infoStyle.Render("📡 Cargando historial..."))
	history, err := c.ReadHistory()
	if err != nil {
		fmt.Println(errorStyle.Render("Error al leer el historial:"), err)
		return err
	}

	if len(history) == 0 {
		clearScreen()
		fmt.Println(boxStyle.Render(infoStyle.Render("El historial está vacío.")))
		fmt.Println(infoStyle.Render("Presiona Enter para continuar..."))
		fmt.Scanln()
		return nil
	}

	for {
		clearScreen()
		options := make([]string, len(history)+1)
		optionMap := make(map[string]sync.HistoryItem)

		for i, item := range history {
			optionKey := fmt.Sprintf("%s %s - %s (%s) - %s",
				listBulletStyle.String(),
				listEpisodeStyle.Render(item.Episode),
				listAnimeTitleStyle.Render(item.Title),
				infoStyle.Render(item.Provider),
				infoStyle.Render(item.Timestamp.Format("2006-01-02 15:04")),
			)
			options[i] = optionKey
			optionMap[optionKey] = item
		}
		options[len(history)] = listNavStyle.Render("← Volver al menú principal")

		var choice string
		prompt := &survey.Select{
			Message:  titleStyle.Render("Historial de Episodios"),
			Options:  options,
			PageSize: 15,
		}
		survey.AskOne(prompt, &choice)

		if strings.Contains(choice, "Volver") {
			return nil
		}

		if selectedItem, ok := optionMap[choice]; ok {
			if c.provider != selectedItem.Provider {
				fmt.Println(infoStyle.Render(fmt.Sprintf("Cambiando al proveedor: %s", selectedItem.Provider)))
				c.provider = selectedItem.Provider
			}

			animeItem := AnimeItem{
				Title:   selectedItem.Title,
				URL:     selectedItem.URL,
				Episode: selectedItem.Episode,
			}
			c.showEpisodeDetails(animeItem)
		}
	}
}

func (c *CLI) showLibraryMenu() error {
	for {
		clearScreen()
		library, err := c.ReadLibrary()
		if err != nil {
			fmt.Println(errorStyle.Render("Error al leer la biblioteca:"), err)
			return err
		}

		counts := make(map[sync.LibraryStatus]int)
		for _, item := range library {
			counts[item.Status]++
		}

		options := []string{
			fmt.Sprintf("Todos (%d)", len(library)),
			fmt.Sprintf("Viendo (%d)", counts[sync.StatusWatching]),
			fmt.Sprintf("Para ver (%d)", counts[sync.StatusPlanToWatch]),
			fmt.Sprintf("Completado (%d)", counts[sync.StatusCompleted]),
			fmt.Sprintf("Dropeado (%d)", counts[sync.StatusDropped]),
			fmt.Sprintf("En espera (%d)", counts[sync.StatusOnHold]),
			"← Volver al menú principal",
		}

		var choice string
		prompt := &survey.Select{
			Message: titleStyle.Render("📚 Mi Biblioteca"),
			Options: options,
		}
		survey.AskOne(prompt, &choice)

		if strings.Contains(choice, "Volver") {
			return nil
		}

		var selectedStatus sync.LibraryStatus
		if strings.HasPrefix(choice, "Todos") {
			selectedStatus = sync.StatusAll
		} else if strings.HasPrefix(choice, "Viendo") {
			selectedStatus = sync.StatusWatching
		} else if strings.HasPrefix(choice, "Para ver") {
			selectedStatus = sync.StatusPlanToWatch
		} else if strings.HasPrefix(choice, "Completado") {
			selectedStatus = sync.StatusCompleted
		} else if strings.HasPrefix(choice, "Dropeado") {
			selectedStatus = sync.StatusDropped
		} else if strings.HasPrefix(choice, "En espera") {
			selectedStatus = sync.StatusOnHold
		}

		c.showLibraryCategory(selectedStatus)
	}
}

func (c *CLI) showLibraryCategory(status sync.LibraryStatus) error {
	for {
		clearScreen()
		library, err := c.ReadLibrary()
		if err != nil {
			return err
		}

		var items []sync.LibraryItem
		for _, item := range library {
			if status == sync.StatusAll || item.Status == status {
				items = append(items, item)
			}
		}

		if len(items) == 0 {
			clearScreen()
			fmt.Println(boxStyle.Render(infoStyle.Render(fmt.Sprintf("No hay animes en '%s'.", status))))
			fmt.Println(infoStyle.Render("Presiona Enter para continuar..."))
			fmt.Scanln()
			return nil
		}

		options := make([]string, len(items)+1)
		optionMap := make(map[string]sync.LibraryItem)

		for i, item := range items {
			var optionKey string
			if status == sync.StatusAll {
				optionKey = fmt.Sprintf("%s %s - %s (%s)",
					listBulletStyle.String(),
					listAnimeTitleStyle.Render(item.Title),
					infoStyle.Render(string(item.Status)),
					infoStyle.Render(item.Provider),
				)
			} else {
				optionKey = fmt.Sprintf("%s %s (%s)",
					listBulletStyle.String(),
					listAnimeTitleStyle.Render(item.Title),
					infoStyle.Render(item.Provider),
				)
			}
			options[i] = optionKey
			optionMap[optionKey] = item
		}
		options[len(items)] = listNavStyle.Render("← Volver")

		var choice string
		prompt := &survey.Select{
			Message:  titleStyle.Render(fmt.Sprintf("📚 %s", status)),
			Options:  options,
			PageSize: 15,
		}
		survey.AskOne(prompt, &choice)

		if strings.Contains(choice, "Volver") {
			return nil
		}

		if selectedItem, ok := optionMap[choice]; ok {
			if c.provider != selectedItem.Provider {
				fmt.Println(infoStyle.Render(fmt.Sprintf("Cambiando al proveedor: %s", selectedItem.Provider)))
				c.provider = selectedItem.Provider
			}

			// We need to fetch anime info to get episodes, but showAnimeInfo takes URL.
			// showAnimeInfo will call showEpisodesList which allows managing library status.
			c.showAnimeInfo(selectedItem.URL)
		}
	}
}

func (c *CLI) manageLibraryStatus(animeInfo *AnimeInfo, animeURL string) error {
	for {
		clearScreen()
		currentStatus := "No en la biblioteca"
		existingItem, err := c.getLibraryItem(animeURL)
		if err == nil && existingItem != nil {
			currentStatus = string(existingItem.Status)
		}

		fmt.Println(titleStyle.Render("📚 Gestionar en Biblioteca"))
		fmt.Println(infoStyle.Render(fmt.Sprintf("Anime: %s", animeInfo.Title)))
		fmt.Println(infoStyle.Render(fmt.Sprintf("Estado actual: %s", currentStatus)))

		options := []string{
			"Viendo",
			"Para ver",
			"Completado",
			"Dropeado",
			"En espera",
			"Eliminar de la biblioteca",
			"← Volver",
		}

		var choice string
		prompt := &survey.Select{
			Message: "Selecciona un estado:",
			Options: options,
		}
		survey.AskOne(prompt, &choice)

		if choice == "← Volver" {
			return nil
		}

		if choice == "Eliminar de la biblioteca" {
			if err := c.removeFromLibrary(animeURL); err != nil {
				fmt.Println(errorStyle.Render("Error al eliminar:"), err)
				time.Sleep(time.Second * 2)
			} else {
				fmt.Println(infoStyle.Render("✅ Eliminado de la biblioteca"))
				time.Sleep(time.Second * 1)
			}
			continue
		}

		var newStatus sync.LibraryStatus
		switch choice {
		case "Viendo":
			newStatus = sync.StatusWatching
		case "Para ver":
			newStatus = sync.StatusPlanToWatch
		case "Completado":
			newStatus = sync.StatusCompleted
		case "Dropeado":
			newStatus = sync.StatusDropped
		case "En espera":
			newStatus = sync.StatusOnHold
		}

		newItem := sync.LibraryItem{
			Title:     animeInfo.Title,
			URL:       animeURL,
			Image:     animeInfo.Img,
			Status:    newStatus,
			Provider:  c.provider,
			UpdatedAt: time.Now(),
		}

		if err := c.updateLibraryItem(newItem); err != nil {
			fmt.Println(errorStyle.Render("Error al actualizar:"), err)
			time.Sleep(time.Second * 2)
		} else {
			fmt.Println(infoStyle.Render(fmt.Sprintf("✅ Guardado como: %s", newStatus)))

			// Auto-sync library status
			go func() {
				c.autoSync(animeInfo.Title, string(newStatus))
			}()

			time.Sleep(time.Second * 1)
		}
	}
}

// --- SYNC MENUS ---

func (c *CLI) showSyncMenu() error {
	for {
		clearScreen()
		config, err := sync.ReadSyncConfig()
		if err != nil {
			fmt.Println(errorStyle.Render("Error al leer configuración:"), err)
			time.Sleep(time.Second * 2)
			return nil
		}

		syncStatus := "No configurado"
		if config.MAL.AccessToken != "" {
			if time.Until(config.MAL.TokenExpiry) > 0 {
				syncStatus = fmt.Sprintf("Configurado (expira en %s)", time.Until(config.MAL.TokenExpiry).Round(time.Minute))
			} else {
				syncStatus = "Token expirado - necesita autenticación"
			}
		}

		fmt.Println(titleStyle.Render("🔄 Sincronización con MyAnimeList"))
		fmt.Println(infoStyle.Render(fmt.Sprintf("Estado: %s", syncStatus)))
		fmt.Println(infoStyle.Render(fmt.Sprintf("Auto-sync: %v", config.AutoSync)))

		var choice string
		options := []string{
			"Configurar MAL",
			"Probar conexión",
			"Sincronizar biblioteca",
			"Sincronizar historial",
			"Sincronización completa",
			"Activar/Desactivar auto-sync",
			"← Volver",
		}

		if config.MAL.AccessToken == "" {
			options = []string{"Configurar MAL", "← Volver"}
		}

		prompt := &survey.Select{
			Message: "Selecciona una opción:",
			Options: options,
		}
		survey.AskOne(prompt, &choice)

		if strings.Contains(choice, "Volver") {
			return nil
		}

		if choice == "Configurar MAL" {
			if err := c.configureMAL(); err != nil {
				fmt.Println(errorStyle.Render("Error:"), err)
				time.Sleep(time.Second * 2)
			}
		} else if choice == "Probar conexión" {
			if err := c.testMALConnection(); err != nil {
				fmt.Println(errorStyle.Render("Error:"), err)
			} else {
				fmt.Println(infoStyle.Render("✅ Conexión exitosa!"))
			}
			time.Sleep(time.Second * 2)
		} else if choice == "Sincronizar biblioteca" {
			if err := c.syncLibraryToMAL(); err != nil {
				fmt.Println(errorStyle.Render("Error:"), err)
			}
			time.Sleep(time.Second * 3)
		} else if choice == "Sincronizar historial" {
			if err := c.syncHistoryToMAL(); err != nil {
				fmt.Println(errorStyle.Render("Error:"), err)
			}
			time.Sleep(time.Second * 3)
		} else if choice == "Sincronización completa" {
			if err := c.fullSyncToMAL(); err != nil {
				fmt.Println(errorStyle.Render("Error:"), err)
			}
			time.Sleep(time.Second * 3)
		} else if choice == "Activar/Desactivar auto-sync" {
			config.AutoSync = !config.AutoSync
			if err := sync.SaveSyncConfig(config); err != nil {
				fmt.Println(errorStyle.Render("Error al guardar configuración:"), err)
			} else {
				fmt.Println(infoStyle.Render(fmt.Sprintf("Auto-sync: %v", config.AutoSync)))
			}
			time.Sleep(time.Second * 1)
		}
	}
}

func (c *CLI) configureMAL() error {
	clearScreen()
	log.Printf("Conectar con MyAnimeList")
	log.Printf("La autenticación se maneja a través del servidor Kujira.")
	log.Printf("No necesitas crear tu propia aplicación de API.")

	// Check backend status first
	log.Printf("Verificando conexión con el servidor...")
	status, err := sync.CheckBackendStatus()
	if err != nil {
		return fmt.Errorf("no se pudo conectar con el servidor: %v", err)
	}

	if !status.Configured {
		return fmt.Errorf("el servidor no está configurado para MAL: %s", status.Message)
	}

	log.Printf("Servidor listo. Iniciando autenticación OAuth2...")

	// Get auth URL from backend
	authResp, err := sync.GetAuthURLFromBackend()
	if err != nil {
		return fmt.Errorf("error al obtener URL de autenticación: %v", err)
	}

	// Start local callback server
	tokenChan, server, err := sync.StartMALAuthServer()
	if err != nil {
		return fmt.Errorf("error al iniciar servidor de autenticación: %v", err)
	}
	defer server.Close()

	fmt.Printf("\nAbre este enlace en tu navegador:\n\n%s\n\n", authResp.AuthURL)
	log.Printf("Esperando autorización...")

	// Wait for callback with tokens
	tokenResp := <-tokenChan

	log.Printf("Tokens recibidos! Guardando configuración...")

	config, err := sync.ReadSyncConfig()
	if err != nil {
		return fmt.Errorf("error al leer configuración: %v", err)
	}

	expiry := time.Now().Add(time.Duration(tokenResp.ExpiresIn) * time.Second)

	config.Provider = sync.ProviderMyAnimeList
	config.MAL.AccessToken = tokenResp.AccessToken
	config.MAL.RefreshToken = tokenResp.RefreshToken
	config.MAL.TokenExpiry = expiry

	// Get username
	malSync, err := sync.NewMALSync(&config.MAL)
	if err == nil {
		if user, err := malSync.GetUser(); err == nil {
			config.MAL.Username = user.Name
			log.Printf("Conectado como: %s", user.Name)
		}
	}

	if err := sync.SaveSyncConfig(config); err != nil {
		return fmt.Errorf("error al guardar configuración: %v", err)
	}

	log.Printf("Configuración completada!")
	return nil
}

func (c *CLI) testMALConnection() error {
	config, err := sync.ReadSyncConfig()
	if err != nil {
		return fmt.Errorf("error al leer configuración: %v", err)
	}

	malSync, err := sync.NewMALSync(&config.MAL)
	if err != nil {
		return fmt.Errorf("error al crear cliente MAL: %v", err)
	}

	user, err := malSync.GetUser()
	if err != nil {
		return fmt.Errorf("error al conectar con MAL: %v", err)
	}

	fmt.Println(infoStyle.Render(fmt.Sprintf("Usuario: %s (%d)", user.Name, user.ID)))
	fmt.Println(infoStyle.Render(fmt.Sprintf("Joined: %s", user.JoinedAt.Format("2006-01-02"))))
	fmt.Println(infoStyle.Render(fmt.Sprintf("Anime entries: %d", user.AnimeStatistics.NumItems)))

	return nil
}

func (c *CLI) syncLibraryToMAL() error {
	config, err := sync.ReadSyncConfig()
	if err != nil {
		return err
	}

	syncService, err := sync.NewSyncService(config, c.ReadLibrary, c.ReadHistory)
	if err != nil {
		return fmt.Errorf("error al crear servicio de sincronización: %v", err)
	}

	clearScreen()
	fmt.Println(infoStyle.Render("📤 Sincronizando biblioteca con MAL..."))

	result, err := syncService.SyncLibrary()
	if err != nil {
		return fmt.Errorf("error durante sincronización: %v", err)
	}

	if result.Success {
		fmt.Println(infoStyle.Render(fmt.Sprintf("✅ %d anime(s) sincronizados", len(result.Uploaded))))
		for _, title := range result.Uploaded {
			fmt.Println(infoStyle.Render(fmt.Sprintf("  • %s", title)))
		}
	} else {
		fmt.Println(errorStyle.Render("❌ Sincronización con errores"))
		for _, errMsg := range result.Errors {
			fmt.Println(errorStyle.Render(fmt.Sprintf("  • %s", errMsg)))
		}
	}

	if err := sync.SaveSyncConfig(config); err != nil {
		log.Printf("Warning: Could not save updated config: %v", err)
	}

	return nil
}

func (c *CLI) syncHistoryToMAL() error {
	config, err := sync.ReadSyncConfig()
	if err != nil {
		return err
	}

	syncService, err := sync.NewSyncService(config, c.ReadLibrary, c.ReadHistory)
	if err != nil {
		return fmt.Errorf("error al crear servicio de sincronización: %v", err)
	}

	clearScreen()
	fmt.Println(infoStyle.Render("📤 Sincronizando historial con MAL..."))

	result, err := syncService.SyncHistory()
	if err != nil {
		return fmt.Errorf("error durante sincronización: %v", err)
	}

	if result.Success {
		fmt.Println(infoStyle.Render(fmt.Sprintf("✅ %d actualización(es) de episodios", len(result.Updated))))
		for _, update := range result.Updated {
			fmt.Println(infoStyle.Render(fmt.Sprintf("  • %s", update)))
		}
	} else {
		fmt.Println(errorStyle.Render("❌ Sincronización con errores"))
		for _, errMsg := range result.Errors {
			fmt.Println(errorStyle.Render(fmt.Sprintf("  • %s", errMsg)))
		}
	}

	if err := sync.SaveSyncConfig(config); err != nil {
		log.Printf("Warning: Could not save updated config: %v", err)
	}

	return nil
}

func (c *CLI) fullSyncToMAL() error {
	config, err := sync.ReadSyncConfig()
	if err != nil {
		return err
	}

	syncService, err := sync.NewSyncService(config, c.ReadLibrary, c.ReadHistory)
	if err != nil {
		return fmt.Errorf("error al crear servicio de sincronización: %v", err)
	}

	clearScreen()
	fmt.Println(infoStyle.Render("🔄 Sincronización completa con MAL..."))
	fmt.Println(infoStyle.Render("Incluyendo biblioteca e historial..."))

	result, err := syncService.FullSync()
	if err != nil {
		return fmt.Errorf("error durante sincronización: %v", err)
	}

	if result.Success {
		fmt.Println(infoStyle.Render("✅ Sincronización exitosa"))
		fmt.Println(infoStyle.Render(fmt.Sprintf("  Biblioteca: %d anime(s)", len(result.Uploaded))))
		fmt.Println(infoStyle.Render(fmt.Sprintf("  Historial: %d actualización(es)", len(result.Updated))))
	} else {
		fmt.Println(errorStyle.Render("⚠️ Sincronización completada con errores"))
		fmt.Println(infoStyle.Render(fmt.Sprintf("  Exitos: %d + %d", len(result.Uploaded), len(result.Updated))))
		fmt.Println(infoStyle.Render(fmt.Sprintf("  Errores: %d", len(result.Errors))))
		for _, errMsg := range result.Errors[:min(5, len(result.Errors))] {
			fmt.Println(errorStyle.Render(fmt.Sprintf("  • %s", errMsg)))
		}
		if len(result.Errors) > 5 {
			fmt.Println(infoStyle.Render(fmt.Sprintf("  ... y %d más", len(result.Errors)-5)))
		}
	}

	config.LastSyncAt = time.Now()
	if err := sync.SaveSyncConfig(config); err != nil {
		log.Printf("Warning: Could not save updated config: %v", err)
	}

	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (c *CLI) autoSync(title string, libraryStatus string) error {
	config, err := sync.ReadSyncConfig()
	if err != nil || !config.AutoSync || config.Provider != sync.ProviderMyAnimeList {
		return nil
	}

	syncService, err := sync.NewSyncService(config, c.ReadLibrary, c.ReadHistory)
	if err != nil {
		log.Printf("Warning: Could not create sync service for auto-sync: %v", err)
		return nil
	}

	malStatus := sync.MALStatusFromLibrary(libraryStatus)
	if err := syncService.AddToMAL(title, string(malStatus), 0, 0); err != nil {
		log.Printf("Warning: Auto-sync failed for '%s': %v", title, err)
	}

	log.Printf("Auto-synced '%s' to MAL with status: %s", title, malStatus)
	return nil
}

func (c *CLI) autoSyncEpisode(title string, episodeStr string) error {
	config, err := sync.ReadSyncConfig()
	if err != nil || !config.AutoSync || config.Provider != sync.ProviderMyAnimeList {
		return nil
	}

	syncService, err := sync.NewSyncService(config, c.ReadLibrary, c.ReadHistory)
	if err != nil {
		log.Printf("Warning: Could not create sync service for auto-sync: %v", err)
		return nil
	}

	var episodeNum int
	n, _ := fmt.Sscanf(episodeStr, "Episodio %d", &episodeNum)
	if n != 1 {
		n, _ = fmt.Sscanf(episodeStr, "%d", &episodeNum)
	}

	if n != 1 {
		log.Printf("Warning: Could not parse episode number from '%s'", episodeStr)
		return nil
	}

	// Always set status to 'watching' when updating progress, unless it was completed, but usually watching is safe for episode updates
	// Actually SyncService.AddToMAL takes status. If we just want to update episode, we might need to be careful not to overwrite status if we don't know it.
	// However, usually if you watch an episode, you are "Watching" it.
	if err := syncService.AddToMAL(title, string(sync.SyncStatusWatching), 0, episodeNum); err != nil {
		log.Printf("Warning: Auto-sync episode failed for '%s': %v", title, err)
	} else {
		log.Printf("Auto-synced '%s' Episode %d to MAL", title, episodeNum)
	}

	return nil
}

func main() {
	cli := NewCLI()
	var rootCmd = &cobra.Command{
		Use:     "kujira-cli",
		Short:   "CLI interactivo para ver anime",
		Long:    "Un cliente de línea de comandos para navegar y reproducir anime desde diferentes proveedores.\n\nCompilado el " + buildDate + " (commit: " + gitCommit + ")",
		Version: version,
		Run: func(cmd *cobra.Command, args []string) {
			if err := ensureCacheDir(); err != nil {
				log.Fatalf("Error al crear el directorio de caché: %v", err)
			}
			if err := cli.selectProvider(); err != nil {
				log.Fatal(err)
			}
			if err := cli.mainMenu(); err != nil {
				log.Fatal(err)
			}
		},
	}

	// Configurar la plantilla de versión
	rootCmd.SetVersionTemplate(`{{printf "%s: %s - versión %s\n" .Name .Short .Version}}
Compilado el: {{.Version}}
Compilación: {{.Name}}-{{.Version}}
`)

	// Agregar flags globales
	rootCmd.PersistentFlags().StringP("provider", "p", "", "Proveedor de anime a usar")
	rootCmd.Flags().BoolP("version", "v", false, "Mostrar información de versión")

	var searchCmd = &cobra.Command{
		Use:   "search [query]",
		Short: "Buscar anime directamente",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if err := ensureCacheDir(); err != nil {
				log.Fatalf("Failed to create cache directory: %v", err)
			}
			cli.provider, _ = cmd.Flags().GetString("provider")
			if cli.provider == "" {
				if err := cli.selectProvider(); err != nil {
					log.Fatal(err)
				}
			}
			// This is a bit of a hack. We need to get the query from args.
			// The searchMenu function asks for it again. Let's refactor this later.
			cli.searchMenu()
		},
	}

	searchCmd.Flags().StringP("provider", "p", "", "Proveedor de anime (animeflv, tioanime, animeav1, monoschinos)")
	rootCmd.AddCommand(searchCmd)

	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
