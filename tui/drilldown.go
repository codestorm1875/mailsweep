package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	gmailpkg "github.com/codestorm1875/mailsweep/gmail"
	"github.com/codestorm1875/mailsweep/util"
)

type DrilldownModel struct {
	sender   gmailpkg.SenderGroup
	cursor   int
	selected map[int]bool
}

func NewDrilldownModel(sender gmailpkg.SenderGroup) DrilldownModel {
	return DrilldownModel{
		sender:   sender,
		cursor:   0,
		selected: make(map[int]bool),
	}
}

func (m DrilldownModel) SelectedMessages() []gmailpkg.Message {
	var msgs []gmailpkg.Message
	for i, sel := range m.selected {
		if sel && i < len(m.sender.Messages) {
			msgs = append(msgs, m.sender.Messages[i])
		}
	}
	return msgs
}

func (m DrilldownModel) Update(msg tea.Msg) (DrilldownModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.sender.Messages)-1 {
				m.cursor++
			}
		case " ":
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "a":
			// If everything is checked, uncheck all — otherwise check all
			allSelected := true
			for i := range m.sender.Messages {
				if !m.selected[i] {
					allSelected = false
					break
				}
			}
			for i := range m.sender.Messages {
				m.selected[i] = !allSelected
			}
		}
	}
	return m, nil
}

func (m DrilldownModel) View() string {
	var b strings.Builder

	title := titleStyle.Render(
		fmt.Sprintf("← %s", m.sender.Email))
	count := statusStyle.Render(
		fmt.Sprintf("%d emails", m.sender.Count))
	b.WriteString(fmt.Sprintf("%s%s\n\n", title, count))

	header := fmt.Sprintf("     %-40s %-14s %8s", "Subject", "Date", "Size")
	b.WriteString(headerStyle.Render(header) + "\n")

	for i, msg := range m.sender.Messages {
		checkbox := uncheckedStyle.Render("[ ]")
		if m.selected[i] {
			checkbox = checkedStyle.Render("[x]")
		}

		subject := truncate(msg.Subject, 40)
		date := truncate(msg.Date, 14)
		size := util.FormatSize(msg.Size)
		coloredSize := sizeStyle(msg.Size)(size)

		row := fmt.Sprintf(" %s %-40s %-14s %8s",
			checkbox, subject, date, coloredSize)

		if i == m.cursor {
			b.WriteString(selectedRowStyle.Render(row) + "\n")
		} else {
			b.WriteString(normalRowStyle.Render(row) + "\n")
		}
	}

	selCount := 0
	for _, s := range m.selected {
		if s {
			selCount++
		}
	}
	if selCount > 0 {
		b.WriteString("\n")
		b.WriteString(statusStyle.Render(
			fmt.Sprintf("%d email(s) selected", selCount)))
	}

	b.WriteString("\n")
	b.WriteString(footerStyle.Render(
		"↑↓/jk navigate   space toggle   a select all   d delete   esc back"))

	return b.String()
}
