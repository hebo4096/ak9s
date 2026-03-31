package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hebo4096/ak9s/internal/azure"
)

type listView struct {
	clusters []azure.Cluster
	cursor int
	height int
	page     int
	perPage  int
}

func newListView(clusters []azure.Cluster) listView {
	return listView{
		clusters: clusters,
		height:   20,
		perPage:  7,
	}
}

func (l *listView) up() {
	if l.cursor > l.pageStart() {
		l.cursor--
	}
}

func (l *listView) down() {
	if l.cursor < l.pageEnd()-1 {
		l.cursor++
	}
}

func (l *listView) nextPage() {
	totalPages := l.totalPages()
	if l.page < totalPages-1 {
		l.page++
	} else {
		l.page = 0
	}
	l.cursor = l.pageStart()
}

func (l *listView) prevPage() {
	if l.page > 0 {
		l.page--
	} else {
		l.page = l.totalPages() - 1
	}
	l.cursor = l.pageStart()
}

func (l *listView) pageStart() int {
	return l.page * l.perPage
}

func (l *listView) pageEnd() int {
	end := l.pageStart() + l.perPage
	if end > len(l.clusters) {
		end = len(l.clusters)
	}
	return end
}

func (l *listView) totalPages() int {
	if len(l.clusters) == 0 {
		return 1
	}
	return (len(l.clusters) + l.perPage - 1) / l.perPage
}

func (l *listView) selected() *azure.Cluster {
	if len(l.clusters) == 0 {
		return nil
	}
	return &l.clusters[l.cursor]
}

func (l *listView) renderBody(width int) string {
	var b strings.Builder

	if len(l.clusters) == 0 {
		b.WriteString(statusStyle.Render("  No AKS clusters found."))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("q: quit"))
		return b.String()
	}

	// AKS Clusters section
	b.WriteString(sectionStyle.Render("AKS Clusters"))
	b.WriteString("\n")
	b.WriteString(statusStyle.Render(strings.Repeat("─", min(width, 120))))
	b.WriteString("\n")

	// Column headers
	sep := " │ "
	header := fmt.Sprintf(" %-2s%-25s%s%-25s%s%-15s%s%-12s%s%-10s%s%-10s",
		"", "RESOURCENAME", sep, "RESOURCEGROUP", sep, "POWERSTATE", sep, "LOCATION", sep, "VERSION", sep, "STATUS")
	b.WriteString(lipglossRender(headerStyle, header))
	b.WriteString("\n")

	// Separator
	b.WriteString(" " + strings.Repeat("─", min(width-1, 119)))
	b.WriteString("\n")
	// Items
	start := l.pageStart()
	end := l.pageEnd()

	for i := start; i < end; i++ {
		c := l.clusters[i]
		var stateIcon string
		switch c.PowerState {
		case "Running":
			stateIcon = runningStyle.Render("●")
		case "Stopped":
			stateIcon = stoppedStyle.Render("●")
		default:
			stateIcon = "●"
		}

		// Pad powerState manually to handle ANSI color codes
		var powerState string
		switch c.PowerState {
		case "Running":
			powerState = runningStyle.Render(padRight(c.PowerState, 15))
		case "Stopped":
			powerState = stoppedStyle.Render(padRight(c.PowerState, 15))
		default:
			powerState = padRight(c.PowerState, 15)
		}

		// Build line without color for powerState when selected
		if i == l.cursor {
			plainLine := fmt.Sprintf(" > %-25s%s%-25s%s%-15s%s%-12s%s%-10s%s%-10s",
				truncate(c.Name, 25), sep,
				truncate(c.ResourceGroup, 25), sep,
				c.PowerState, sep,
				c.Location, sep,
				c.KubernetesVersion, sep,
				c.ProvisioningState,
			)
			// Pad to full width
			if len(plainLine) < width-4 {
				plainLine += strings.Repeat(" ", width-4-len(plainLine))
			}
			b.WriteString(selectedItemStyle.Render(plainLine))
		} else {
			line := fmt.Sprintf("%s %-25s%s%-25s%s%s%s%-12s%s%-10s%s%-10s",
				stateIcon,
				truncate(c.Name, 25), sep,
				truncate(c.ResourceGroup, 25), sep,
				powerState, sep,
				c.Location, sep,
				c.KubernetesVersion, sep,
				c.ProvisioningState,
			)
			b.WriteString(itemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	// End separator
	b.WriteString(statusStyle.Render(strings.Repeat("─", min(width, 120))))
	b.WriteString("\n")

	// Status bar
	status := fmt.Sprintf(" %d clusters | %d/%d | Page %d/%d",
		len(l.clusters), l.cursor+1, len(l.clusters), l.page+1, l.totalPages())
	b.WriteString(statusStyle.Render(status))
	b.WriteString("\n")

	// Help
	b.WriteString(helpStyle.Render("↑/k: up | ↓/j: down | p/P: page | enter: details | /help: usage | r: refresh | q: quit"))

	return b.String()
}

func lipglossRender(style lipgloss.Style, text string) string {
	return style.Render(text)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "…"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
