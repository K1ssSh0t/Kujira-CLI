package kujira_cli

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Modelo para los datos de la API (ejemplo con JSONPlaceholder)
type Post struct {
	ID     int    `json:"id"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	UserID int    `json:"userId"`
}

// Modelo para BubbleTea
type model struct {
	posts  []Post
	status string
	err    error
}

// Estilos con lipgloss
var (
	titleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1).
			Bold(true)

	itemStyle  = lipgloss.NewStyle().PaddingLeft(1)
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
)

// Comando para obtener datos de la API
type apiMsg struct {
	posts []Post
	err   error
}

func fetchData() tea.Msg {
	// Cambia esta URL por tu API real
	url := "https://jsonplaceholder.typicode.com/posts"
	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return apiMsg{err: err}
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return apiMsg{err: err}
	}

	var posts []Post
	if err := json.Unmarshal(body, &posts); err != nil {
		return apiMsg{err: err}
	}

	return apiMsg{posts: posts}
}

// Inicialización del modelo
func initialModel() model {
	return model{
		posts:  []Post{},
		status: "Cargando datos...",
		err:    nil,
	}
}

func (m model) Init() tea.Cmd {
	return func() tea.Msg {
		return fetchData()
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case apiMsg:
		if msg.err != nil {
			m.err = msg.err
			m.status = "Error!"
		} else {
			m.posts = msg.posts
			m.status = "Datos cargados!"
		}
	}
	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return errorStyle.Render(fmt.Sprintf("Error: %v", m.err))
	}

	if len(m.posts) == 0 {
		return m.status
	}

	s := titleStyle.Render("Posts de la API") + "\n\n"
	for _, post := range m.posts {
		s += fmt.Sprintf("%s%d: %s\n",
			itemStyle.Render("•"),
			post.ID,
			lipgloss.NewStyle().Foreground(lipgloss.Color("#04B575")).Render(post.Title),
		)
	}
	s += "\nPresiona 'q' para salir\n"
	return s
}

func main() {
	p := tea.NewProgram(initialModel())
	if err := p.Start(); err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
}
