package tui

import "github.com/charmbracelet/lipgloss"

var (
	AppStyle = lipgloss.NewStyle().Padding(1, 2)

	TitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFDF5")).
			Background(lipgloss.Color("#25A065")).
			Padding(0, 1)

	StatusMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.AdaptiveColor{Light: "#04B575", Dark: "#04B575"}).
				Render

	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))

	// New styles for improved UI
	boxStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2)

	activeItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("170")).Bold(true)
	inactiveItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)
