package tui

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/hebo4096/ak9s/internal/azure"
)

type viewState int

const (
	viewList viewState = iota
	viewDetail
	viewCommand
	viewConfirm
	viewSubscription
)

// Messages
type subsLoadedMsg struct {
	subs     []azure.Subscription
	userInfo azure.UserInfo
}

type clustersLoadedMsg struct {
	clusters []azure.Cluster
}

type errMsg struct {
	err error
}

type actionLogMsg struct {
	message string
}

type actionAllDoneMsg struct{}

type actionBatchDoneMsg struct {
	logs []string
}

type tickMsg time.Time

// Model is the main TUI model.
type Model struct {
	client        *azure.Client
	state         viewState
	list          listView
	detail        detailView
	loading       bool
	err           error
	width         int
	height        int
	commandBuf    string
	statusMsg     string
	logs          []string
	runningOps    int
	userInfo      azure.UserInfo
	subscriptions []azure.Subscription
	completions   []string
	completionIdx int
	confirmMsg    string
	pendingAction func() (Model, tea.Cmd)
	selectedSub string
	subCursor   int
}

// New creates a new TUI model.
func New(client *azure.Client) Model {
	return Model{
		client:  client,
		loading: true,
		width:   120,
		height:  30,
	}
}

func (m Model) Init() tea.Cmd {
	return m.loadSubscriptions
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(30*time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.list.height = msg.Height
		m.detail.maxLines = msg.Height - 14
		return m, nil

	case subsLoadedMsg:
		m.loading = false
		m.subscriptions = msg.subs
		m.userInfo = msg.userInfo
		if len(msg.subs) == 1 {
			// Only one subscription, skip selection
			m.selectedSub = msg.subs[0].ID
			m.state = viewList
			m.loading = true
			return m, tea.Batch(m.loadSelectedClusters, m.tickCmd())
		}
		m.state = viewSubscription
		m.subCursor = 0
		return m, nil

	case clustersLoadedMsg:
		m.loading = false
		cursor := m.list.cursor
		offset := m.list.offset
		m.list = newListView(msg.clusters)
		m.list.height = m.height
		if cursor < len(msg.clusters) {
			m.list.cursor = cursor
			m.list.offset = offset
		}
		return m, nil

	case tickMsg:
		if !m.loading && m.runningOps == 0 && m.state != viewSubscription {
			return m, tea.Batch(m.loadSelectedClusters, m.tickCmd())
		}
		return m, m.tickCmd()

	case actionLogMsg:
		m.logs = append(m.logs, msg.message)
		if len(m.logs) > 10 {
			m.logs = m.logs[len(m.logs)-10:]
		}
		return m, nil

	case actionBatchDoneMsg:
		m.logs = msg.logs
		m.runningOps--
		if m.runningOps <= 0 {
			m.runningOps = 0
			m.loading = true
			return m, m.loadSelectedClusters
		}
		return m, nil

	case actionAllDoneMsg:
		m.runningOps--
		if m.runningOps <= 0 {
			m.runningOps = 0
			m.loading = true
			return m, m.loadSelectedClusters
		}
		return m, nil

	case errMsg:
		m.loading = false
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Subscription selection mode
	if m.state == viewSubscription {
		switch msg.String() {
		case "up", "k":
			if m.subCursor > 0 {
				m.subCursor--
			}
		case "down", "j":
			if m.subCursor < len(m.subscriptions)-1 {
				m.subCursor++
			}
		case "enter", " ":
			if m.subCursor < len(m.subscriptions) {
				m.selectedSub = m.subscriptions[m.subCursor].ID
				m.state = viewList
				m.loading = true
				return m, tea.Batch(m.loadSelectedClusters, m.tickCmd())
			}
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	// Confirmation mode
	if m.state == viewConfirm {
		switch msg.String() {
		case "y", "Y":
			if m.pendingAction != nil {
				result, cmd := m.pendingAction()
				result.state = viewList
				result.confirmMsg = ""
				result.pendingAction = nil
				return result, cmd
			}
			m.state = viewList
			return m, nil
		case "n", "N", "esc":
			m.state = viewList
			m.confirmMsg = ""
			m.pendingAction = nil
			m.statusMsg = "Delete cancelled."
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		}
		return m, nil
	}

	// Command input mode
	if m.state == viewCommand {
		switch msg.String() {
		case "esc":
			m.state = viewList
			m.commandBuf = ""
			m.completions = nil
			m.completionIdx = 0
			return m, nil
		case "enter":
			m.completions = nil
			m.completionIdx = 0
			return m.executeCommand()
		case "tab":
			m.tabComplete()
			return m, nil
		case "backspace":
			if len(m.commandBuf) > 0 {
				m.commandBuf = m.commandBuf[:len(m.commandBuf)-1]
			}
			m.completions = nil
			m.completionIdx = 0
			if len(m.commandBuf) == 0 {
				m.state = viewList
			}
			return m, nil
		case "ctrl+c":
			return m, tea.Quit
		default:
			if len(msg.String()) == 1 {
				m.commandBuf += msg.String()
				m.completions = nil
				m.completionIdx = 0
			}
			return m, nil
		}
	}

	switch msg.String() {
	case "ctrl+c", "q":
		if m.state == viewDetail {
			m.state = viewList
			return m, nil
		}
		return m, tea.Quit

	case "/":
		if m.state == viewList {
			m.state = viewCommand
			m.commandBuf = "/"
			m.statusMsg = ""
			return m, nil
		}

	case "up", "k":
		switch m.state {
		case viewList:
			m.list.up()
		case viewDetail:
			m.detail.up()
		}

	case "down", "j":
		switch m.state {
		case viewList:
			m.list.down()
		case viewDetail:
			m.detail.down()
		}

	case "enter":
		if m.state == viewList {
			if selected := m.list.selected(); selected != nil {
				m.detail = newDetailView(*selected)
				m.detail.maxLines = m.height - 2
				m.state = viewDetail
			}
		}

	case "esc", "backspace":
		if m.state == viewDetail {
			m.state = viewList
		}

	case "r":
		if m.state == viewList {
			m.loading = true
			m.statusMsg = ""
			return m, m.loadSelectedClusters
		}
	}

	return m, nil
}

func (m Model) executeCommand() (tea.Model, tea.Cmd) {
	cmd := m.commandBuf
	m.state = viewList
	m.commandBuf = ""

	// /help command
	if cmd == "/help" {
		m.statusMsg = ""
		m.logs = []string{
			"Available Commands:",
			"",
			"  /help                  Show this help",
			"  /switch                Switch subscription",
			"  /start NAME/RG         Start a cluster",
			"  /stop NAME/RG          Stop a cluster",
			"  /delete NAME/RG        Delete a cluster (with confirmation)",
			"  /stop /bulk            Stop all clusters",
			"  /delete /bulk          Delete all clusters (with confirmation)",
			"",
			"  Multiple targets: /start NAME1/RG1 NAME2/RG2",
			"  Tab: command & cluster name completion",
		}
		return m, nil
	}

	// /switch command - switch subscription
	if cmd == "/switch" {
		m.state = viewSubscription
		m.subCursor = 0
		return m, nil
	}

	// /stop /bulk - stop all clusters in parallel
	if cmd == "/stop /bulk" {
		clusters := m.list.clusters
		if len(clusters) == 0 {
			m.statusMsg = "No clusters to stop"
			return m, nil
		}
		m.runningOps++
		m.logs = nil
		return m, m.runParallel("Stopping", clusters, func(ctx context.Context, c azure.Cluster) error {
			return m.client.StopCluster(ctx, c.SubscriptionID, c.ResourceGroup, c.Name)
		})
	}

	// /delete /bulk - delete all clusters in parallel
	if cmd == "/delete /bulk" {
		clusters := m.list.clusters
		if len(clusters) == 0 {
			m.statusMsg = "No clusters to delete"
			return m, nil
		}
		names := make([]string, len(clusters))
		for i, c := range clusters {
			names[i] = c.Name + "/" + c.ResourceGroup
		}
		var confirmLines []string
		confirmLines = append(confirmLines, "")
		confirmLines = append(confirmLines, fmt.Sprintf("  The following %d cluster(s) will be deleted:", len(clusters)))
		confirmLines = append(confirmLines, "")
		for _, name := range names {
			confirmLines = append(confirmLines, "    • "+name)
		}
		confirmLines = append(confirmLines, "")
		confirmLines = append(confirmLines, "  Do you really want to execute this? (y/n)")
		m.state = viewConfirm
		m.confirmMsg = strings.Join(confirmLines, "\n")
		m.pendingAction = func() (Model, tea.Cmd) {
			m.runningOps++
			m.logs = nil
			return m, m.runParallel("Deleting", clusters, func(ctx context.Context, c azure.Cluster) error {
				return m.client.DeleteCluster(ctx, c.SubscriptionID, c.ResourceGroup, c.Name)
			})
		}
		return m, nil
	}

	// /start NAME/RG, /stop NAME/RG, /delete NAME/RG
	if strings.HasPrefix(cmd, "/start ") || strings.HasPrefix(cmd, "/stop ") || strings.HasPrefix(cmd, "/delete ") {
		parts := strings.SplitN(cmd, " ", 2)
		action := parts[0]
		args := strings.Fields(parts[1])

		var targets []azure.Cluster
		var notFound []string
		for _, arg := range args {
			argParts := strings.SplitN(arg, "/", 2)
			if len(argParts) != 2 || argParts[0] == "" || argParts[1] == "" {
				m.statusMsg = fmt.Sprintf("Invalid format: %s. Usage: %s NAME/RG [NAME/RG ...]", arg, action)
				return m, nil
			}
			found := false
			for _, c := range m.list.clusters {
				if strings.EqualFold(c.Name, argParts[0]) && strings.EqualFold(c.ResourceGroup, argParts[1]) {
					targets = append(targets, c)
					found = true
					break
				}
			}
			if !found {
				notFound = append(notFound, arg)
			}
		}
		if len(notFound) > 0 {
			m.statusMsg = fmt.Sprintf("Cluster(s) not found: %s", strings.Join(notFound, ", "))
			return m, nil
		}
		if len(targets) == 0 {
			m.statusMsg = "No targets specified"
			return m, nil
		}

		actionLabel := "Starting"
		actionFn := func(ctx context.Context, c azure.Cluster) error {
			return m.client.StartCluster(ctx, c.SubscriptionID, c.ResourceGroup, c.Name)
		}
		if action == "/stop" {
			actionLabel = "Stopping"
			actionFn = func(ctx context.Context, c azure.Cluster) error {
				return m.client.StopCluster(ctx, c.SubscriptionID, c.ResourceGroup, c.Name)
			}
		} else if action == "/delete" {
			actionLabel = "Deleting"
			actionFn = func(ctx context.Context, c azure.Cluster) error {
				return m.client.DeleteCluster(ctx, c.SubscriptionID, c.ResourceGroup, c.Name)
			}

			// Show confirmation for delete
			names := make([]string, len(targets))
			for i, t := range targets {
				names[i] = t.Name + "/" + t.ResourceGroup
			}
			var confirmLines []string
			confirmLines = append(confirmLines, "")
			confirmLines = append(confirmLines, fmt.Sprintf("  The following %d cluster(s) will be deleted:", len(targets)))
			confirmLines = append(confirmLines, "")
			for _, name := range names {
				confirmLines = append(confirmLines, "    • "+name)
			}
			confirmLines = append(confirmLines, "")
			confirmLines = append(confirmLines, "  Do you really want to execute this? (y/n)")
			m.state = viewConfirm
			m.confirmMsg = strings.Join(confirmLines, "\n")
			finalTargets := targets
			m.pendingAction = func() (Model, tea.Cmd) {
				m.runningOps++
				m.logs = nil
				return m, m.runParallel("Deleting", finalTargets, func(ctx context.Context, c azure.Cluster) error {
					return m.client.DeleteCluster(ctx, c.SubscriptionID, c.ResourceGroup, c.Name)
				})
			}
			return m, nil
		}

		m.runningOps++
		m.logs = nil
		return m, m.runParallel(actionLabel, targets, actionFn)
	}

	m.statusMsg = fmt.Sprintf("Unknown command: %s (type /help for usage)", cmd)
	return m, nil
}

// runParallel runs an action on multiple clusters concurrently, sending log messages via a tea.Program.
func (m Model) runParallel(actionLabel string, clusters []azure.Cluster, fn func(context.Context, azure.Cluster) error) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		type result struct {
			label   string
			err     error
		}
		resCh := make(chan result, len(clusters))

		var wg sync.WaitGroup
		for _, c := range clusters {
			wg.Add(1)
			go func(c azure.Cluster) {
				defer wg.Done()
				label := fmt.Sprintf("%s/%s", c.Name, c.ResourceGroup)
				err := fn(ctx, c)
				resCh <- result{label: label, err: err}
			}(c)
		}

		go func() {
			wg.Wait()
			close(resCh)
		}()

		var logs []string
		for _, c := range clusters {
			logs = append(logs, fmt.Sprintf("%s %s/%s ...", actionLabel, c.Name, c.ResourceGroup))
		}

		successes := 0
		failures := 0
		for r := range resCh {
			if r.err != nil {
				logs = append(logs, fmt.Sprintf("✗ %s %s failed: %v", actionLabel, r.label, r.err))
				failures++
			} else {
				logs = append(logs, fmt.Sprintf("✓ %s %s done", actionLabel, r.label))
				successes++
			}
		}

		logs = append(logs, fmt.Sprintf("Completed: %d succeeded, %d failed", successes, failures))

		return actionBatchDoneMsg{logs: logs}
	}
}

func (m *Model) tabComplete() {
	// Build completions on first tab press
	if m.completions == nil {
		m.completions = m.buildCompletions()
		m.completionIdx = 0
		if len(m.completions) == 0 {
			return
		}
		m.commandBuf = m.completions[0]
		return
	}

	// Cycle through completions on subsequent tab presses
	if len(m.completions) == 0 {
		return
	}
	m.completionIdx = (m.completionIdx + 1) % len(m.completions)
	m.commandBuf = m.completions[m.completionIdx]
}

func (m *Model) buildCompletions() []string {
	buf := m.commandBuf

	// Base commands
	baseCommands := []string{"/start", "/stop", "/stop /bulk", "/delete", "/delete /bulk", "/switch", "/help"}

	// If just typing the command prefix, complete commands
	if !strings.Contains(buf, " ") || buf == "/stop " || buf == "/stop /b" || buf == "/delete " || buf == "/delete /b" {
		var matches []string
		for _, cmd := range baseCommands {
			if strings.HasPrefix(cmd, buf) {
				matches = append(matches, cmd)
			}
		}
		if len(matches) > 0 {
			return matches
		}
	}

	// If typing /start or /stop with a space, complete with cluster NAME/RG
	if strings.HasPrefix(buf, "/start ") || (strings.HasPrefix(buf, "/stop ") && !strings.HasPrefix(buf, "/stop /")) || (strings.HasPrefix(buf, "/delete ") && !strings.HasPrefix(buf, "/delete /")) {
		parts := strings.SplitN(buf, " ", 2)
		prefix := parts[0]
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}

		var matches []string
		for _, c := range m.list.clusters {
			candidate := c.Name + "/" + c.ResourceGroup
			if arg == "" || strings.HasPrefix(strings.ToLower(candidate), strings.ToLower(arg)) {
				matches = append(matches, prefix+" "+candidate)
			}
		}
		return matches
	}

	return nil
}

func (m Model) renderSubscriptionView() string {
	var b strings.Builder

	b.WriteString(sectionStyle.Render("Select Subscription"))
	b.WriteString("\n")
	b.WriteString(statusStyle.Render(strings.Repeat("─", 90)))
	b.WriteString("\n")
	b.WriteString(statusStyle.Render("  Select a subscription to manage AKS clusters:"))
	b.WriteString("\n\n")

	for i, sub := range m.subscriptions {
		cursor := "  "
		if i == m.subCursor {
			cursor = "> "
		}
		line := fmt.Sprintf("%s%s (%s)", cursor, sub.Name, sub.ID)
		if i == m.subCursor {
			b.WriteString(selectedItemStyle.Render(line))
		} else {
			b.WriteString(statusStyle.Render(line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("↑/k: up | ↓/j: down | enter/space: select | ctrl+c: quit"))
	return b.String()
}

const ak9sLogo = `    _    _  ___   ____
   / \  | |/ / _ \/ ___|
  / _ \ | ' / (_) \___ \
 / ___ \| . \ \__, |___) |
/_/   \_\_|\_\  /_/|____/`

func renderLogo() string {
	logo := logoStyle.Render(ak9sLogo)
	tagline := statusStyle.Render("  Manage your AKS Clusters")
	return lipgloss.JoinHorizontal(lipgloss.Center, logo, "  ", tagline)
}

func (m Model) renderHeader() string {
	var b strings.Builder
	b.WriteString(renderLogo())
	b.WriteString("\n\n")

	line := strings.Repeat("─", 90)

	b.WriteString(sectionStyle.Render("Azure Environment"))
	b.WriteString("\n")
	b.WriteString(statusStyle.Render(line))
	b.WriteString("\n")

	// Show the selected subscription
	for _, sub := range m.subscriptions {
		if sub.ID == m.selectedSub {
			b.WriteString(statusStyle.Render(fmt.Sprintf("   Subscription: %s (%s)", sub.Name, sub.ID)))
			b.WriteString("\n")
			tid := sub.TenantID
			if tid == "" {
				tid = m.userInfo.TenantID
			}
			if tid != "" {
				b.WriteString(statusStyle.Render(fmt.Sprintf("   Tenant ID:    %s", tid)))
				b.WriteString("\n")
			}
			break
		}
	}

	b.WriteString(statusStyle.Render(fmt.Sprintf("   User:         %s (%s)", m.userInfo.UPN, m.userInfo.Name)))
	b.WriteString("\n")
	b.WriteString(statusStyle.Render(line))
	b.WriteString("\n\n")
	return b.String()
}

func (m Model) View() string {
	if m.loading && m.state != viewSubscription {
		return renderLogo() + "\n\n" +
			statusStyle.Render("  Loading AKS clusters...  (authenticating with Azure)")
	}

	if m.err != nil {
		return renderLogo() + "\n\n" +
			stoppedStyle.Render("  Error: "+m.err.Error()) + "\n\n" +
			helpStyle.Render("r: retry | q: quit")
	}

	header := m.renderHeader()

	var view string
	switch m.state {
	case viewDetail:
		// Use a large maxLines, then trim the final output to fit terminal
		m.detail.maxLines = 200
		full := header + m.detail.render(m.width)
		return fitToHeight(full, m.width, m.height)
	case viewSubscription:
		return renderLogo() + "\n\n" + m.renderSubscriptionView()
	case viewCommand:
		view = header + m.list.renderBody(m.width)
		view += "\n" + commandStyle.Render(m.commandBuf+"█")
		view += "  " + helpStyle.Render("tab: complete | enter: execute | esc: cancel")
	case viewConfirm:
		view = header + m.list.renderBody(m.width)
		view += "\n" + stoppedStyle.Render(m.confirmMsg)
	default:
		view = header + m.list.renderBody(m.width)
	}

	if m.statusMsg != "" {
		view += "\n" + statusMsgStyle.Render("  "+m.statusMsg)
	}

	// Build the Operation Logs section
	var logsSection string
	logsSection += sectionStyle.Render("Operation Logs")
	logsSection += "\n" + statusStyle.Render(strings.Repeat("─", 90))

	if len(m.logs) > 0 {
		for _, log := range m.logs {
			logsSection += "\n" + statusStyle.Render("  "+log)
		}
	}

	if m.runningOps > 0 {
		logsSection += "\n" + statusStyle.Render("  (processing...)")
	}

	// Calculate padding to push Operation Logs to a fixed position
	viewLines := strings.Count(view, "\n") + 1
	logsLines := strings.Count(logsSection, "\n") + 1
	totalUsed := viewLines + logsLines
	padding := m.height - totalUsed
	if padding < 1 {
		padding = 1
	}
	view += strings.Repeat("\n", padding)
	view += logsSection

	return view
}

func (m Model) loadSubscriptions() tea.Msg {
	ctx := context.Background()

	subs, err := m.client.ListSubscriptions(ctx)
	if err != nil {
		return errMsg{err: err}
	}

	userInfo := m.client.GetUserInfo(ctx)

	return subsLoadedMsg{subs: subs, userInfo: userInfo}
}

func (m Model) loadSelectedClusters() tea.Msg {
	ctx := context.Background()

	var targetSubs []azure.Subscription
	for _, sub := range m.subscriptions {
		if sub.ID == m.selectedSub {
			targetSubs = append(targetSubs, sub)
			break
		}
	}

	clusters, err := m.client.ListClusters(ctx, targetSubs)
	if err != nil {
		return errMsg{err: err}
	}

	return clustersLoadedMsg{clusters: clusters}
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// countVisualLines counts how many visual lines a string occupies,
// accounting for line wrapping at the given terminal width.
func countVisualLines(s string, width int) int {
	if width <= 0 {
		width = 120
	}
	lines := strings.Split(s, "\n")
	total := 0
	for _, line := range lines {
		stripped := ansiRegex.ReplaceAllString(line, "")
		runeLen := len([]rune(stripped))
		if runeLen == 0 {
			total++
		} else {
			total += (runeLen + width - 1) / width
		}
	}
	return total
}

// fitToHeight truncates each line to terminal width and keeps only
// enough lines to fit within the terminal height.
func fitToHeight(s string, width, height int) string {
	if height <= 0 {
		return s
	}
	if width <= 0 {
		width = 120
	}

	rawLines := strings.Split(s, "\n")
	var result []string
	visualCount := 0

	for _, line := range rawLines {
		// Strip ANSI to measure visible length
		stripped := ansiRegex.ReplaceAllString(line, "")
		runeLen := len([]rune(stripped))

		// How many visual lines this would take
		vLines := 1
		if runeLen > width {
			vLines = (runeLen + width - 1) / width
		}

		if visualCount+vLines > height {
			// Only add if we have room for at least 1 line
			if visualCount < height {
				result = append(result, line)
			}
			break
		}
		result = append(result, line)
		visualCount += vLines
	}

	return strings.Join(result, "\n")
}
