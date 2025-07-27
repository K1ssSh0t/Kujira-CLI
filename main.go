package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"kujira_cli/tui"
)

func main() {
	m := tui.New()
	p := tea.NewProgram(m, tea.WithAltScreen())

	if err := p.Start(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}