package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	gmailpkg "github.com/codestorm1875/mailsweep/gmail"
)

type ConfirmModel struct {
	sender   string
	messages []gmailpkg.Message
	cursor   int
}

func NewConfirmModel(sender gmailpkg.SenderGroup, messages []gmailpkg.Message) ConfirmModel {
	return ConfirmModel{
		sender:   sender.Email,
		messages: messages,
		cursor:   1, // defaults to "No" so you don't accidentally delete
	}
}

func (m ConfirmModel) Update(msg tea.Msg) (ConfirmModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "left", "h":
			m.cursor = 0
		case "right", "l":
			m.cursor = 1
		case "tab":
			m.cursor = (m.cursor + 1) % 2
		}
	}
	return m, nil
}

func (m ConfirmModel) View() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("\n  Delete %d email(s) from\n", len(m.messages)))
	b.WriteString(fmt.Sprintf("  %s?\n\n", m.sender))
	b.WriteString("  This cannot be undone.\n\n")

	yesLabel := "  [ Yes ]  "
	noLabel := "  [ No ]  "

	if m.cursor == 0 {
		yesLabel = selectedRowStyle.Render("  [ Yes ]  ")
	} else {
		noLabel = selectedRowStyle.Render("  [ No ]  ")
	}

	b.WriteString(fmt.Sprintf("    %s    %s\n", yesLabel, noLabel))
	b.WriteString("\n")
	b.WriteString(footerStyle.Render("y confirm   n/esc cancel   ←→ switch"))

	return confirmStyle.Render(b.String())
}
