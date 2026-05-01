package tui

import (
	"fmt"
	
	tea "github.com/charmbracelet/bubbletea"

	gmailpkg "github.com/codestorm1875/mailsweep/gmail"
	"google.golang.org/api/gmail/v1"
)

type AppState int

const (
	StateLoading     AppState = iota
	StateLeaderboard
	StateDrilldown
	StateConfirm
)

type App struct {
	state   AppState
	client  *gmailpkg.Client
	email   string
	senders []gmailpkg.SenderGroup
	err     error

	leaderboard LeaderboardModel
	drilldown   DrilldownModel
	confirm     ConfirmModel

	fetched  int
	total    int
	progress chan progressMsg

	width  int
	height int
}

func NewApp(service *gmail.Service, email string) App {
	client := gmailpkg.NewClient(service)
	return App{
		state:    StateLoading,
		client:   client,
		email:    email,
		progress: make(chan progressMsg, 100),
	}
}

func (a App) Init() tea.Cmd {
	if senders, err := gmailpkg.LoadCache(); err == nil && len(senders) > 0 {
		return func() tea.Msg {
			return fetchDoneMsg{senders: senders, err: nil}
		}
	}
	return tea.Batch(a.fetchEmails(), waitForProgress(a.progress))
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
		a.senders = msg.senders
		a.err = msg.err
		if a.err == nil && len(a.senders) > 0 {
			_ = gmailpkg.SaveCache(a.senders)
		}
		a.state = StateLeaderboard
		a.leaderboard = NewLeaderboardModel(a.senders, a.email, a.width, a.height)
		return a, nil
	case progressMsg:
		a.fetched = msg.fetched
		a.total = msg.total
		return a, waitForProgress(a.progress)
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
	senders []gmailpkg.SenderGroup
	err     error
}

type progressMsg struct {
	fetched int
	total   int
}

type deleteResultMsg struct {
	err error
}

func (a *App) fetchEmails() tea.Cmd {
	return func() tea.Msg {
		senders, err := a.client.FetchAllMessages(func(fetched, total int) {
			select {
			case a.progress <- progressMsg{fetched: fetched, total: total}:
			default:
			}
		})
		return fetchDoneMsg{senders: senders, err: err}
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
				a.state = StateDrilldown
				a.drilldown = NewDrilldownModel(a.senders[idx], a.width, a.height)
			}
			return a, nil
		case "r":
			a.state = StateLoading
			// Clear any stale progress messages
			for len(a.progress) > 0 {
				<-a.progress
			}
			return a, tea.Batch(a.fetchEmails(), waitForProgress(a.progress))
		}
	}

	var cmd tea.Cmd
	a.leaderboard, cmd = a.leaderboard.Update(msg)
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
				a.confirm = NewConfirmModel(a.drilldown.sender, selected)
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
			ids := make([]string, len(a.confirm.messages))
			for i, m := range a.confirm.messages {
				ids[i] = m.ID
			}
			a.state = StateLoading
			return a, func() tea.Msg {
				err := a.client.BatchDelete(ids)
				if err != nil {
					return deleteResultMsg{err: err}
				}
				// Re-scan the mailbox after deleting
				for len(a.progress) > 0 {
					<-a.progress
				}
				senders, fetchErr := a.client.FetchAllMessages(func(fetched, total int) {
					select {
					case a.progress <- progressMsg{fetched: fetched, total: total}:
					default:
					}
				})
				return fetchDoneMsg{senders: senders, err: fetchErr}
			}
		case "n", "N", "esc":
			a.state = StateDrilldown
			return a, nil
		}
	}

	var cmd tea.Cmd
	a.confirm, cmd = a.confirm.Update(msg)
	return a, cmd
}

func (a App) viewLoading() string {
	if a.total > 0 {
		return renderLoading(a.fetched, a.total)
	}
	return renderLoading(0, 0)
}
