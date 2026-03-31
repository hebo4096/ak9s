package tui

import "github.com/charmbracelet/lipgloss"

var (
	// Colors
	primaryColor   = lipgloss.Color("#7D56F4")
	secondaryColor = lipgloss.Color("#5A9E6F")
	errorColor     = lipgloss.Color("#FF6B6B")
	mutedColor     = lipgloss.Color("#626262")
	highlightColor = lipgloss.Color("#FFFFFF")
	selectedBg = lipgloss.Color("#3D3D5C")

	// Logo (no background)
	logoStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor)

	// Status bar
	statusStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(0, 1)

	// List items
	itemStyle = lipgloss.NewStyle().
			Padding(0, 2)

	selectedItemStyle = lipgloss.NewStyle().
				Foreground(highlightColor).
				Background(selectedBg).
				Bold(true).
				Padding(0, 2)

	// Detail view
	headerStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1)

	labelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#AAAAAA")).
			Width(22)

	valueStyle = lipgloss.NewStyle().
			Foreground(highlightColor)

	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(secondaryColor).
			MarginTop(1)

	// Power state
	runningStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	stoppedStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// Help
	helpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Padding(1, 2)

	// Command input
	commandStyle = lipgloss.NewStyle().
			Foreground(highlightColor).
			Background(lipgloss.Color("#2D2D4E")).
			Padding(0, 1)

	// Status message
	statusMsgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFD700")).
			Bold(true)
)


