package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	gmailpkg "github.com/codestorm1875/mailsweep/gmail"
	"github.com/codestorm1875/mailsweep/util"
)

type LeaderboardSortMode int

const (
	SortBySize LeaderboardSortMode = iota
	SortByCount
	SortBySender
)

type LeaderboardModel struct {
	senders     []gmailpkg.SenderGroup
	email       string
	historyID   string
	scannedAt   time.Time
	totalEmails int
	totalSize   int64
	filterQuery string
	filtering   bool
	filtered    []int
	sortMode    LeaderboardSortMode
	cursor      int
	maxSize     int64
	width       int
	height      int
}

func NewLeaderboardModel(snapshot gmailpkg.MailboxSnapshot, email string, width, height int) LeaderboardModel {
	var maxSize int64
	if len(snapshot.Senders) > 0 {
		maxSize = snapshot.Senders[0].TotalSize
	}
	return LeaderboardModel{
		senders:     snapshot.Senders,
		email:       email,
		historyID:   snapshot.HistoryID,
		scannedAt:   snapshot.ScannedAt,
		totalEmails: snapshot.TotalEmails,
		totalSize:   snapshot.TotalSize,
		filtered:    allSenderIndexes(snapshot.Senders),
		sortMode:    SortBySize,
		cursor:      0,
		maxSize:     maxSize,
		width:       width,
		height:      height,
	}
}

func (m *LeaderboardModel) ApplyPreferences(sortMode int, filterQuery string) {
	if sortMode >= int(SortBySize) && sortMode <= int(SortBySender) {
		m.sortMode = LeaderboardSortMode(sortMode)
	}
	m.filterQuery = filterQuery
	m.applyFilter()
}

func (m LeaderboardModel) SortMode() int {
	return int(m.sortMode)
}

func (m LeaderboardModel) FilterQuery() string {
	return m.filterQuery
}

func (m *LeaderboardModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m LeaderboardModel) SelectedIndex() int {
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return -1
	}
	return m.filtered[m.cursor]
}

func (m LeaderboardModel) PrefetchCandidates(window int) []gmailpkg.SenderGroup {
	if len(m.filtered) == 0 || window <= 0 {
		return nil
	}

	start := m.cursor - 1
	if start < 0 {
		start = 0
	}
	end := start + window
	if end > len(m.filtered) {
		end = len(m.filtered)
		start = end - window
		if start < 0 {
			start = 0
		}
	}

	candidates := make([]gmailpkg.SenderGroup, 0, end-start)
	if m.cursor >= start && m.cursor < end {
		candidates = append(candidates, m.senders[m.filtered[m.cursor]])
	}
	for i := start; i < end; i++ {
		if i == m.cursor {
			continue
		}
		candidates = append(candidates, m.senders[m.filtered[i]])
	}
	return candidates
}

func (m LeaderboardModel) Update(msg tea.Msg) (LeaderboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filtering {
			switch msg.String() {
			case "esc":
				m.filtering = false
			case "enter":
				m.filtering = false
			case "backspace":
				if len(m.filterQuery) > 0 {
					m.filterQuery = m.filterQuery[:len(m.filterQuery)-1]
					m.applyFilter()
				}
			case "ctrl+w":
				m.filterQuery = ""
				m.applyFilter()
			default:
				if msg.Type == tea.KeyRunes {
					m.filterQuery += msg.String()
					m.applyFilter()
				}
			}
			return m, nil
		}

		switch msg.String() {
		case "/":
			m.filtering = true
		case "s":
			m.cycleSort()
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
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

	stats := statusStyle.Render(
		fmt.Sprintf("Total scanned: %d emails    Total size: %s",
			m.totalEmails, util.FormatSize(m.totalSize)))
	b.WriteString(stats + "\n")

	refreshMode := "full scan"
	if m.historyID != "" {
		refreshMode = "incremental sync ready"
	}
	cacheAge := "cache not timestamped"
	if !m.scannedAt.IsZero() {
		cacheAge = "last updated: " + m.scannedAt.Local().Format("2006-01-02 15:04:05")
	}
	b.WriteString(statusStyle.Render(fmt.Sprintf("%s    %s", cacheAge, refreshMode)) + "\n\n")

	sortLabel := "sort: " + m.sortModeLabel()
	filterLabel := "filter: /"
	if m.filterQuery != "" || m.filtering {
		mode := ""
		if m.filtering {
			mode = " (typing)"
		}
		filterLabel = fmt.Sprintf("filter: %s%s", m.filterQuery, mode)
	}
	b.WriteString(statusStyle.Render(
		fmt.Sprintf("Showing %d of %d senders    %s    %s", len(m.filtered), len(m.senders), sortLabel, filterLabel),
	) + "\n\n")

	if selected := m.selectedSender(); selected != nil {
		if len(selected.Messages) > 0 {
			b.WriteString(statusStyle.Render(
				fmt.Sprintf(
					"Selected: %s    older than 30d: %d    90d: %d    180d: %d    365d: %d",
					truncate(selected.Email, 24),
					countMessagesOlderThan(selected.Messages, Age30Days),
					countMessagesOlderThan(selected.Messages, Age90Days),
					countMessagesOlderThan(selected.Messages, Age180Days),
					countMessagesOlderThan(selected.Messages, Age365Days),
				),
			) + "\n\n")
		} else {
			b.WriteString(statusStyle.Render(
				fmt.Sprintf("Selected: %s    age preview loading in background...", truncate(selected.Email, 24)),
			) + "\n\n")
		}
	}

	header := fmt.Sprintf("  %-4s %-35s %8s %8s   %-14s",
		"#", "Sender", "Emails", "Size", "Bar")
	b.WriteString(headerStyle.Render(header) + "\n")

	pageSize := m.height - 12
	if pageSize < 5 {
		pageSize = 5
	}

	start := m.cursor - (pageSize / 2)
	if start < 0 {
		start = 0
	}
	end := start + pageSize
	if end > len(m.filtered) {
		end = len(m.filtered)
		start = end - pageSize
		if start < 0 {
			start = 0
		}
	}

	if len(m.filtered) == 0 {
		b.WriteString(statusStyle.Render("No senders match the current filter.") + "\n\n")
		b.WriteString(footerStyle.Render("↑↓/jk navigate   / filter   s sort   D delete sender   r refresh   q quit"))
		return b.String()
	}

	for i := start; i < end; i++ {
		idx := m.filtered[i]
		s := m.senders[idx]
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
	if m.filtering {
		b.WriteString(footerStyle.Render("type to filter   backspace edit   enter finish   esc cancel typing"))
	} else {
		b.WriteString(footerStyle.Render("↑↓/jk navigate   / filter   s sort   enter drill in   D delete sender   r refresh   q quit"))
	}

	return b.String()
}

func (m *LeaderboardModel) applyFilter() {
	selected := m.SelectedIndex()
	query := strings.ToLower(strings.TrimSpace(m.filterQuery))
	if query == "" {
		m.filtered = allSenderIndexes(m.senders)
	} else {
		filtered := make([]int, 0, len(m.senders))
		for i, sender := range m.senders {
			if strings.Contains(strings.ToLower(sender.Email), query) ||
				strings.Contains(strings.ToLower(sender.Sender), query) {
				filtered = append(filtered, i)
			}
		}
		m.filtered = filtered
	}
	m.sortFiltered()

	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	m.restoreSelection(selected)
}

func (m *LeaderboardModel) cycleSort() {
	selected := m.SelectedIndex()
	m.sortMode = (m.sortMode + 1) % 3
	m.sortFiltered()
	if len(m.filtered) == 0 {
		m.cursor = 0
		return
	}
	m.restoreSelection(selected)
}

func allSenderIndexes(senders []gmailpkg.SenderGroup) []int {
	indexes := make([]int, len(senders))
	for i := range senders {
		indexes[i] = i
	}
	return indexes
}

func (m LeaderboardModel) selectedSender() *gmailpkg.SenderGroup {
	idx := m.SelectedIndex()
	if idx < 0 || idx >= len(m.senders) {
		return nil
	}
	return &m.senders[idx]
}

func (m *LeaderboardModel) sortFiltered() {
	sort.SliceStable(m.filtered, func(i, j int) bool {
		left := m.senders[m.filtered[i]]
		right := m.senders[m.filtered[j]]

		switch m.sortMode {
		case SortByCount:
			if left.Count != right.Count {
				return left.Count > right.Count
			}
			if left.TotalSize != right.TotalSize {
				return left.TotalSize > right.TotalSize
			}
		case SortBySender:
			leftName := strings.ToLower(left.Email)
			rightName := strings.ToLower(right.Email)
			if leftName != rightName {
				return leftName < rightName
			}
			if left.TotalSize != right.TotalSize {
				return left.TotalSize > right.TotalSize
			}
		default:
			if left.TotalSize != right.TotalSize {
				return left.TotalSize > right.TotalSize
			}
			if left.Count != right.Count {
				return left.Count > right.Count
			}
		}

		return strings.ToLower(left.Email) < strings.ToLower(right.Email)
	})
}

func (m *LeaderboardModel) restoreSelection(selected int) {
	if selected >= 0 {
		for i, idx := range m.filtered {
			if idx == selected {
				m.cursor = i
				return
			}
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = len(m.filtered) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

func (m LeaderboardModel) sortModeLabel() string {
	switch m.sortMode {
	case SortByCount:
		return "count"
	case SortBySender:
		return "sender"
	default:
		return "size"
	}
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
