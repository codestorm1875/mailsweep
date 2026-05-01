package tui

import "github.com/charmbracelet/lipgloss"

var (
	cyan    = lipgloss.Color("#00D4FF")
	red     = lipgloss.Color("#FF4D6A")
	yellow  = lipgloss.Color("#FFD93D")
	green   = lipgloss.Color("#6BCB77")
	dimGray = lipgloss.Color("#666666")
	white   = lipgloss.Color("#FFFFFF")
)

var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan).
			Padding(0, 1)

	selectedRowStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(white).
				Background(cyan)

	sizeLargeStyle  = lipgloss.NewStyle().Bold(true).Foreground(red)
	sizeMediumStyle = lipgloss.NewStyle().Foreground(yellow)
	sizeSmallStyle  = lipgloss.NewStyle().Foreground(green)

	barHighStyle   = lipgloss.NewStyle().Foreground(red)
	barMediumStyle = lipgloss.NewStyle().Foreground(yellow)
	barLowStyle    = lipgloss.NewStyle().Foreground(cyan)

	checkedStyle   = lipgloss.NewStyle().Bold(true).Foreground(green)
	uncheckedStyle = lipgloss.NewStyle().Foreground(dimGray)

	statusStyle = lipgloss.NewStyle().
			Foreground(dimGray).
			Italic(true).
			Padding(0, 1)

	confirmStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(red).
			Padding(1, 3).
			Align(lipgloss.Center)

	footerStyle = lipgloss.NewStyle().
			Foreground(dimGray).
			Padding(0, 1)

	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(cyan).
			Underline(true)

	normalRowStyle = lipgloss.NewStyle()
)
