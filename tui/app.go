package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	gmailpkg "github.com/codestorm1875/mailsweep/gmail"
	"google.golang.org/api/gmail/v1"
)

type AppState int

const (
	StateLoading AppState = iota
	StateLeaderboard
	StateDrilldown
	StateConfirm
)

type App struct {
	state          AppState
	client         *gmailpkg.Client
	email          string
	snapshot       gmailpkg.MailboxSnapshot
	prefs          gmailpkg.Preferences
	senders        []gmailpkg.SenderGroup
	err            error
	loadingText    string
	prefetched     map[string]gmailpkg.SenderGroup
	prefetching    map[string]bool
	prefetchLRU    []string
	prefetchWanted map[string]bool

	leaderboard LeaderboardModel
	drilldown   DrilldownModel
	confirm     ConfirmModel
	confirmBack AppState

	fetched  int
	total    int
	progress chan progressMsg

	width  int
	height int
}

func NewApp(service *gmail.Service, email string) App {
	client := gmailpkg.NewClient(service)
	return App{
		state:          StateLoading,
		client:         client,
		email:          email,
		progress:       make(chan progressMsg, 100),
		prefetched:     make(map[string]gmailpkg.SenderGroup),
		prefetching:    make(map[string]bool),
		prefetchWanted: make(map[string]bool),
	}
}

const senderPrefetchWindow = 3
const senderPrefetchCacheLimit = 8

func (a App) Init() tea.Cmd {
	if prefs, err := gmailpkg.LoadPreferences(); err == nil {
		a.prefs = prefs
	}
	if snapshot, err := gmailpkg.LoadCache(); err == nil && (len(snapshot.Senders) > 0 || snapshot.HistoryID != "" || !snapshot.ScannedAt.IsZero()) {
		return func() tea.Msg {
			return fetchDoneMsg{snapshot: snapshot, prefs: a.prefs, err: nil}
		}
	}
	return tea.Batch(a.syncEmails(), waitForProgress(a.progress))
}

func waitForProgress(c chan progressMsg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-c
		if !ok {
			return nil
		}
		return msg
	}
}

func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.leaderboard.SetSize(msg.Width, msg.Height)
		a.drilldown.SetSize(msg.Width, msg.Height)
		return a, nil
	case fetchDoneMsg:
		a.snapshot = msg.snapshot
		a.prefs = msg.prefs
		a.senders = msg.snapshot.Senders
		a.err = msg.err
		a.fetched = 0
		a.total = 0
		a.loadingText = ""
		a.prefetched = make(map[string]gmailpkg.SenderGroup)
		a.prefetching = make(map[string]bool)
		a.prefetchLRU = nil
		a.prefetchWanted = make(map[string]bool)
		if a.err == nil {
			_ = gmailpkg.SaveCache(a.snapshot)
		}
		a.state = StateLeaderboard
		a.leaderboard = NewLeaderboardModel(a.snapshot, a.email, a.width, a.height)
		a.leaderboard.ApplyPreferences(a.prefs.SortMode, a.prefs.FilterQuery)
		return a, a.prefetchVisibleSenders()
	case progressMsg:
		a.fetched = msg.fetched
		a.total = msg.total
		return a, waitForProgress(a.progress)
	case deleteResultMsg:
		a.err = msg.err
		a.fetched = 0
		a.total = 0
		a.loadingText = ""
		a.prefetched = make(map[string]gmailpkg.SenderGroup)
		a.prefetching = make(map[string]bool)
		a.prefetchLRU = nil
		a.prefetchWanted = make(map[string]bool)
		if a.err != nil {
			a.state = StateDrilldown
			return a, nil
		}

		a.snapshot = gmailpkg.UpdateSnapshotAfterDelete(a.snapshot, msg.ids)
		a.senders = a.snapshot.Senders
		_ = gmailpkg.SaveCache(a.snapshot)
		a.leaderboard = NewLeaderboardModel(a.snapshot, a.email, a.width, a.height)
		a.leaderboard.ApplyPreferences(a.prefs.SortMode, a.prefs.FilterQuery)
		a.state = StateLeaderboard
		return a, nil
	case senderLoadedMsg:
		a.err = msg.err
		if a.err != nil {
			delete(a.prefetching, msg.email)
			if msg.activate {
				a.fetched = 0
				a.total = 0
				a.loadingText = ""
				a.state = StateLeaderboard
			}
			return a, nil
		}
		delete(a.prefetching, msg.email)
		if msg.activate || a.prefetchWanted[msg.email] {
			a.rememberPrefetched(msg.email, msg.sender)
		}
		if msg.activate {
			a.fetched = 0
			a.total = 0
			a.loadingText = ""
			a.drilldown = NewDrilldownModel(msg.sender, a.width, a.height)
			a.state = StateDrilldown
		}
		return a, nil
	case senderDeleteReadyMsg:
		a.err = msg.err
		a.fetched = 0
		a.total = 0
		a.loadingText = ""
		if a.err != nil {
			a.state = StateLeaderboard
			return a, nil
		}
		a.rememberPrefetched(msg.sender.Email, msg.sender)
		a.confirm = NewConfirmModelWithAge(msg.sender, msg.sender.Messages, AgeFilter(a.prefs.LastAgeFilter))
		a.confirmBack = StateLeaderboard
		a.state = StateConfirm
		return a, nil
	}

	switch a.state {
	case StateLeaderboard:
		return a.updateLeaderboard(msg)
	case StateDrilldown:
		return a.updateDrilldown(msg)
	case StateConfirm:
		return a.updateConfirm(msg)
	}

	return a, nil
}

func (a App) View() string {
	if a.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press 'q' to quit.\n", a.err)
	}

	switch a.state {
	case StateLoading:
		return a.viewLoading()
	case StateLeaderboard:
		return a.leaderboard.View()
	case StateDrilldown:
		return a.drilldown.View()
	case StateConfirm:
		return a.confirm.View()
	default:
		return ""
	}
}

type fetchDoneMsg struct {
	snapshot gmailpkg.MailboxSnapshot
	prefs    gmailpkg.Preferences
	err      error
}

type progressMsg struct {
	fetched int
	total   int
}

type deleteResultMsg struct {
	ids []string
	err error
}

type senderLoadedMsg struct {
	email    string
	sender   gmailpkg.SenderGroup
	err      error
	activate bool
}

type senderDeleteReadyMsg struct {
	sender gmailpkg.SenderGroup
	err    error
}

func (a *App) syncEmails() tea.Cmd {
	return func() tea.Msg {
		snapshot, err := a.client.SyncMailbox(a.snapshot, func(fetched, total int) {
			select {
			case a.progress <- progressMsg{fetched: fetched, total: total}:
			default:
			}
		})
		return fetchDoneMsg{snapshot: snapshot, prefs: a.prefs, err: err}
	}
}

func (a App) updateLeaderboard(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			return a, tea.Quit
		case "enter":
			if idx := a.leaderboard.SelectedIndex(); idx >= 0 && idx < len(a.senders) {
				sender := a.senders[idx]
				if prefetched, ok := a.prefetched[sender.Email]; ok {
					a.touchPrefetched(sender.Email)
					a.drilldown = NewDrilldownModel(prefetched, a.width, a.height)
					a.state = StateDrilldown
					return a, nil
				}
				return a.startDrilldownLoad(sender)
			}
			return a, nil
		case "D":
			if idx := a.leaderboard.SelectedIndex(); idx >= 0 && idx < len(a.senders) {
				sender := a.senders[idx]
				if prefetched, ok := a.prefetched[sender.Email]; ok {
					a.touchPrefetched(sender.Email)
					a.confirm = NewConfirmModelWithAge(prefetched, prefetched.Messages, AgeFilter(a.prefs.LastAgeFilter))
					a.confirmBack = StateLeaderboard
					a.state = StateConfirm
					return a, nil
				}
				return a.startSenderDeleteLoad(sender)
			}
			return a, nil
		case "r":
			a.state = StateLoading
			a.fetched = 0
			a.total = 0
			a.loadingText = "Refreshing mailbox..."
			// Clear any stale progress messages
			for len(a.progress) > 0 {
				<-a.progress
			}
			return a, tea.Batch(a.syncEmails(), waitForProgress(a.progress))
		}
	}

	var cmd tea.Cmd
	prevIdx := a.leaderboard.SelectedIndex()
	a.leaderboard, cmd = a.leaderboard.Update(msg)
	a.savePreferencesFromUI()
	if a.leaderboard.SelectedIndex() != prevIdx {
		return a, tea.Batch(cmd, a.prefetchVisibleSenders())
	}
	return a, cmd
}

func (a App) updateDrilldown(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			a.state = StateLeaderboard
			return a, nil
		case "d":
			selected := a.drilldown.SelectedMessages()
			if len(selected) > 0 {
				a.state = StateConfirm
				a.confirm = NewConfirmModelWithAge(a.drilldown.sender, selected, AgeFilter(a.prefs.LastAgeFilter))
				a.confirmBack = StateDrilldown
			}
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.drilldown, cmd = a.drilldown.Update(msg)
	return a, cmd
}

func (a App) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "y", "Y":
			return a.startDelete()
		case "n", "N", "esc":
			a.state = a.confirmBack
			return a, nil
		case "enter":
			if a.confirm.cursor == 0 {
				return a.startDelete()
			}
			a.state = a.confirmBack
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.confirm, cmd = a.confirm.Update(msg)
	a.savePreferencesFromUI()
	return a, cmd
}

func (a App) startDelete() (tea.Model, tea.Cmd) {
	filtered := a.confirm.FilteredMessages()
	ids := make([]string, len(filtered))
	for i, m := range filtered {
		ids[i] = m.ID
	}
	a.state = StateLoading
	a.fetched = 0
	a.total = 0
	a.loadingText = "Moving selected emails to Trash..."
	return a, func() tea.Msg {
		err := a.client.BatchDelete(ids)
		return deleteResultMsg{ids: ids, err: err}
	}
}

func (a App) startDrilldownLoad(sender gmailpkg.SenderGroup) (tea.Model, tea.Cmd) {
	if prefetched, ok := a.prefetched[sender.Email]; ok {
		a.touchPrefetched(sender.Email)
		a.drilldown = NewDrilldownModel(prefetched, a.width, a.height)
		a.state = StateDrilldown
		return a, nil
	}

	a.state = StateLoading
	a.fetched = 0
	a.total = 0
	a.loadingText = fmt.Sprintf("Loading messages from %s...", sender.Email)
	return a, func() tea.Msg {
		hydrated, err := a.hydrateSender(sender)
		if err != nil {
			return senderLoadedMsg{email: sender.Email, err: err, activate: true}
		}
		return senderLoadedMsg{email: sender.Email, sender: hydrated, activate: true}
	}
}

func (a App) startSenderDeleteLoad(sender gmailpkg.SenderGroup) (tea.Model, tea.Cmd) {
	a.state = StateLoading
	a.fetched = 0
	a.total = 0
	a.loadingText = fmt.Sprintf("Loading messages from %s...", sender.Email)
	return a, func() tea.Msg {
		hydrated, err := a.hydrateSender(sender)
		if err != nil {
			return senderDeleteReadyMsg{err: err}
		}
		return senderDeleteReadyMsg{sender: hydrated}
	}
}

func (a App) prefetchVisibleSenders() tea.Cmd {
	ordered := a.leaderboard.PrefetchCandidates(senderPrefetchWindow)
	if len(ordered) == 0 {
		a.prefetchWanted = make(map[string]bool)
		return nil
	}
	a.prefetchWanted = make(map[string]bool, len(ordered))
	for _, sender := range ordered {
		a.prefetchWanted[sender.Email] = true
	}

	cmds := make([]tea.Cmd, 0, len(ordered))
	for _, sender := range ordered {
		if cmd := a.prefetchSender(sender); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	return tea.Batch(cmds...)
}

func (a App) prefetchSender(sender gmailpkg.SenderGroup) tea.Cmd {
	if _, ok := a.prefetched[sender.Email]; ok {
		return nil
	}
	if a.prefetching[sender.Email] {
		return nil
	}

	a.prefetching[sender.Email] = true
	return func() tea.Msg {
		hydrated, err := a.hydrateSender(sender)
		if err != nil {
			return senderLoadedMsg{email: sender.Email, err: err}
		}
		return senderLoadedMsg{email: sender.Email, sender: hydrated}
	}
}

func (a *App) rememberPrefetched(email string, sender gmailpkg.SenderGroup) {
	a.prefetched[email] = sender
	for i := range a.senders {
		if a.senders[i].Email == email {
			a.senders[i] = sender
			break
		}
	}
	for i := range a.leaderboard.senders {
		if a.leaderboard.senders[i].Email == email {
			a.leaderboard.senders[i] = sender
			break
		}
	}
	a.touchPrefetched(email)

	for len(a.prefetchLRU) > senderPrefetchCacheLimit {
		evict := a.prefetchLRU[0]
		a.prefetchLRU = a.prefetchLRU[1:]
		if a.prefetching[evict] {
			a.prefetchLRU = append(a.prefetchLRU, evict)
			continue
		}
		delete(a.prefetched, evict)
	}
}

func (a *App) touchPrefetched(email string) {
	for i, cached := range a.prefetchLRU {
		if cached == email {
			a.prefetchLRU = append(a.prefetchLRU[:i], a.prefetchLRU[i+1:]...)
			break
		}
	}
	a.prefetchLRU = append(a.prefetchLRU, email)
}

func (a App) viewLoading() string {
	return renderLoading(a.loadingText, a.fetched, a.total)
}

func (a App) hydrateSender(sender gmailpkg.SenderGroup) (gmailpkg.SenderGroup, error) {
	messages, err := a.client.FetchMessagesForSender(sender.Email)
	if err != nil {
		return sender, err
	}

	sender.Messages = messages
	sender.Count = len(messages)
	var totalSize int64
	for _, msg := range messages {
		totalSize += msg.Size
	}
	sender.TotalSize = totalSize

	return sender, nil
}

func (a *App) savePreferencesFromUI() {
	a.prefs.SortMode = a.leaderboard.SortMode()
	a.prefs.FilterQuery = a.leaderboard.FilterQuery()
	a.prefs.LastAgeFilter = a.confirm.AgeFilter()
	_ = gmailpkg.SavePreferences(a.prefs)
}
