package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/codestorm1875/mailsweep/auth"
	"github.com/codestorm1875/mailsweep/tui"
)

func main() {
	client, email, err := auth.GetGmailClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	p := tea.NewProgram(tui.NewApp(client, email), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running mailsweep: %v\n", err)
		os.Exit(1)
	}
}
