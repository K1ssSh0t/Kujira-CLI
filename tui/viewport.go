package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
)

var (
	viewportTitleStyle = lipgloss.NewStyle().
				Background(lipgloss.Color("62")).
				Foreground(lipgloss.Color("#FAFAFA")).
				Padding(0, 1)

	viewportInfoStyle = lipgloss.NewStyle().
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("62")).
				PaddingLeft(1)
)

func NewViewport(width, height int) viewport.Model {
	v := viewport.New(width, height)
	v.Style = viewportInfoStyle
	return v
}

func (m model) viewportView() string {
	if m.selectedEpisode == nil {
		return ""
	}

	var s string
	s += fmt.Sprintf("Title: %s\n", m.selectedEpisode.Title)
	s += fmt.Sprintf("Episode: %s\n", m.selectedEpisode.Episode)
	s += fmt.Sprintf("URL: %s\n", m.selectedEpisode.Url)

	return viewportTitleStyle.Render("Episode Info") + "\n" + s
}