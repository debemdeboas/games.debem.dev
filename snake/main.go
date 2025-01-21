package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	snake "github.com/debemdeboas/games.debem.dev/snake/game"
	"golang.org/x/term"
)

func main() {
	m := initialModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v", err)
		os.Exit(1)
	}
}

func initialModel() tea.Model {
	txtStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder())
	quitStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("8"))
	foodStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("9"))
	snakeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("10"))
	borderStyle := lipgloss.NewStyle().SetString("  ")

	// Get terminal width and height
	w, h, _ := term.GetSize(int(os.Stdout.Fd()))

	return snake.NewModel(
		os.Getenv("TERM"),
		"256",
		w,
		h,
		"dark",
		txtStyle,
		quitStyle,
		foodStyle,
		snakeStyle,
		borderStyle,
	)
}
