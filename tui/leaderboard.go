package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	gmailpkg "github.com/codestorm1875/mailsweep/gmail"
	"github.com/codestorm1875/mailsweep/util"
)

type LeaderboardModel struct {
	senders []gmailpkg.SenderGroup
	email   string
	cursor  int
	maxSize int64
	width   int
	height  int
}

func NewLeaderboardModel(senders []gmailpkg.SenderGroup, email string, width, height int) LeaderboardModel {
	var maxSize int64
	if len(senders) > 0 {
		maxSize = senders[0].TotalSize
	}
	return LeaderboardModel{
		senders: senders,
		email:   email,
		cursor:  0,
		maxSize: maxSize,
		width:   width,
		height:  height,
	}
}

func (m *LeaderboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m LeaderboardModel) SelectedIndex() int {
	return m.cursor
}

func (m LeaderboardModel) Update(msg tea.Msg) (LeaderboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.senders)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m LeaderboardModel) View() string {
	var b strings.Builder

	title := titleStyle.Render("mailsweep")
	user := statusStyle.Render(fmt.Sprintf("logged in as: %s", m.email))
	b.WriteString(fmt.Sprintf("%s%s\n", title, user))

	var totalEmails int
	var totalSize int64
	for _, s := range m.senders {
		totalEmails += s.Count
		totalSize += s.TotalSize
	}
	stats := statusStyle.Render(
		fmt.Sprintf("Total scanned: %d emails    Total size: %s",
			totalEmails, util.FormatSize(totalSize)))
	b.WriteString(stats + "\n\n")

	header := fmt.Sprintf("  %-4s %-35s %8s %8s   %-14s",
		"#", "Sender", "Emails", "Size", "Bar")
	b.WriteString(headerStyle.Render(header) + "\n")

	pageSize := m.height - 8
	if pageSize < 5 {
		pageSize = 5
	}

	start := m.cursor - (pageSize / 2)
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > len(m.senders) {
		end = len(m.senders)
		start = end - pageSize
		if start < 0 {
			start = 0
		}
	}

	for i := start; i < end; i++ {
		s := m.senders[i]
		rank := fmt.Sprintf("%-4d", i+1)
		sender := truncate(s.Email, 35)
		emails := fmt.Sprintf("%d", s.Count)
		size := util.FormatSize(s.TotalSize)
		bar := renderBar(s.TotalSize, m.maxSize, 14)

		row := fmt.Sprintf("  %s %-35s %8s %8s   %s",
			rank, sender, emails, size, bar)

		if i == m.cursor {
			b.WriteString(selectedRowStyle.Render(row) + "\n")
		} else {
			b.WriteString(normalRowStyle.Render(row) + "\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(footerStyle.Render("↑↓/jk navigate   enter drill in   r refresh   q quit"))

	return b.String()
}

func renderBar(size, maxSize int64, width int) string {
	if maxSize == 0 {
		return strings.Repeat("░", width)
	}
	filled := int(float64(size) / float64(maxSize) * float64(width))
	if filled < 1 && size > 0 {
		filled = 1
	}
	empty := width - filled

	filledStr := strings.Repeat("█", filled)
	emptyStr := strings.Repeat("░", empty)

	ratio := float64(size) / float64(maxSize)
	var styledFilled string
	switch {
	case ratio > 0.66:
		styledFilled = barHighStyle.Render(filledStr)
	case ratio > 0.33:
		styledFilled = barMediumStyle.Render(filledStr)
	default:
		styledFilled = barLowStyle.Render(filledStr)
	}

	return styledFilled + emptyStr
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

func sizeStyle(size int64) func(string) string {
	var style func(strs ...string) string
	switch {
	case size > 100*1024*1024:
		style = sizeLargeStyle.Render
	case size > 10*1024*1024:
		style = sizeMediumStyle.Render
	default:
		style = sizeSmallStyle.Render
	}
	return func(s string) string { return style(s) }
}
