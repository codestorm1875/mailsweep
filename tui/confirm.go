package tui

import (
	"fmt"
	"net/mail"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	gmailpkg "github.com/codestorm1875/mailsweep/gmail"
)

type AgeFilter int

const (
	AgeAny AgeFilter = iota
	Age30Days
	Age90Days
	Age180Days
	Age365Days
)

type ConfirmModel struct {
	sender    string
	messages  []gmailpkg.Message
	cursor    int
	ageFilter AgeFilter
}

func NewConfirmModel(sender gmailpkg.SenderGroup, messages []gmailpkg.Message) ConfirmModel {
	return ConfirmModel{
		sender:    sender.Email,
		messages:  messages,
		cursor:    1, // defaults to "No" so you don't accidentally delete
		ageFilter: AgeAny,
	}
}

func NewConfirmModelWithAge(sender gmailpkg.SenderGroup, messages []gmailpkg.Message, ageFilter AgeFilter) ConfirmModel {
	model := NewConfirmModel(sender, messages)
	if ageFilter >= AgeAny && ageFilter <= Age365Days {
		model.ageFilter = ageFilter
	}
	return model
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
		case "[":
			if m.ageFilter > AgeAny {
				m.ageFilter--
			}
		case "]":
			if m.ageFilter < Age365Days {
				m.ageFilter++
			}
		}
	}
	return m, nil
}

func (m ConfirmModel) FilteredMessages() []gmailpkg.Message {
	if m.ageFilter == AgeAny {
		return m.messages
	}

	cutoff := ageFilterCutoff(m.ageFilter)
	filtered := make([]gmailpkg.Message, 0, len(m.messages))
	for _, msg := range m.messages {
		parsed, err := mail.ParseDate(msg.Date)
		if err != nil {
			continue
		}
		if parsed.Before(cutoff) {
			filtered = append(filtered, msg)
		}
	}
	return filtered
}

func countMessagesOlderThan(messages []gmailpkg.Message, filter AgeFilter) int {
	if filter == AgeAny {
		return len(messages)
	}

	cutoff := ageFilterCutoff(filter)
	count := 0
	for _, msg := range messages {
		parsed, err := mail.ParseDate(msg.Date)
		if err != nil {
			continue
		}
		if parsed.Before(cutoff) {
			count++
		}
	}
	return count
}

func (m ConfirmModel) View() string {
	var b strings.Builder
	filtered := m.FilteredMessages()

	b.WriteString(fmt.Sprintf("\n  Move %d email(s) to Trash from\n", len(filtered)))
	b.WriteString(fmt.Sprintf("  %s?\n\n", m.sender))
	b.WriteString("  Gmail will move these messages to Trash.\n")
	b.WriteString(fmt.Sprintf("  Age filter: %s\n\n", m.ageFilter.label()))

	if len(filtered) == 0 {
		b.WriteString("  No messages match the current age filter.\n\n")
	}

	yesLabel := "  [ Yes ]  "
	noLabel := "  [ No ]  "

	if m.cursor == 0 {
		yesLabel = selectedRowStyle.Render("  [ Yes ]  ")
	} else {
		noLabel = selectedRowStyle.Render("  [ No ]  ")
	}

	b.WriteString(fmt.Sprintf("    %s    %s\n", yesLabel, noLabel))
	b.WriteString("\n")
	b.WriteString(footerStyle.Render("y confirm   n/esc cancel   ←→ switch   [ ] age filter"))

	return confirmStyle.Render(b.String())
}

func (m ConfirmModel) AgeFilter() int {
	return int(m.ageFilter)
}

func (a AgeFilter) label() string {
	switch a {
	case Age30Days:
		return "older than 30 days"
	case Age90Days:
		return "older than 90 days"
	case Age180Days:
		return "older than 180 days"
	case Age365Days:
		return "older than 365 days"
	default:
		return "all messages"
	}
}

func (a AgeFilter) duration() time.Duration {
	switch a {
	case Age30Days:
		return 30 * 24 * time.Hour
	case Age90Days:
		return 90 * 24 * time.Hour
	case Age180Days:
		return 180 * 24 * time.Hour
	case Age365Days:
		return 365 * 24 * time.Hour
	default:
		return 0
	}
}

func ageFilterCutoff(filter AgeFilter) time.Time {
	return time.Now().Add(-filter.duration())
}
