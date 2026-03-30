package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/hebo4096/ak9s/internal/azure"
)

type listView struct {
	clusters []azure.Cluster
	cursor   int
	offset   int
	height   int
}

func newListView(clusters []azure.Cluster) listView {
	return listView{
		clusters: clusters,
		height:   20,
	}
}

func (l *listView) up() {
	if l.cursor > 0 {
		l.cursor--
		if l.cursor < l.offset {
			l.offset = l.cursor
		}
	}
}

func (l *listView) down() {
	if l.cursor < len(l.clusters)-1 {
		l.cursor++
		if l.cursor >= l.offset+l.visibleRows() {
			l.offset = l.cursor - l.visibleRows() + 1
		}
	}
}

func (l *listView) visibleRows() int {
	return l.height - 4 // header + column headers + border + status
}

func (l *listView) selected() *azure.Cluster {
	if len(l.clusters) == 0 {
		return nil
	}
	return &l.clusters[l.cursor]
}

func (l *listView) render(width int) string {
	return l.renderBody(width)
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
	visible := l.visibleRows()
	end := l.offset + visible
	if end > len(l.clusters) {
		end = len(l.clusters)
	}

	for i := l.offset; i < end; i++ {
		c := l.clusters[i]
		stateIcon := "●"
		if c.PowerState == "Running" {
			stateIcon = runningStyle.Render("●")
		} else if c.PowerState == "Stopped" {
			stateIcon = stoppedStyle.Render("●")
		}

		// Pad powerState manually to handle ANSI color codes
		powerState := padRight(c.PowerState, 15)
		if c.PowerState == "Running" {
			powerState = runningStyle.Render(padRight(c.PowerState, 15))
		} else if c.PowerState == "Stopped" {
			powerState = stoppedStyle.Render(padRight(c.PowerState, 15))
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
	b.WriteString("\n")
	status := fmt.Sprintf(" %d clusters | %d/%d",
		len(l.clusters), l.cursor+1, len(l.clusters))
	b.WriteString(statusStyle.Render(status))
	b.WriteString("\n")

	// Help
	b.WriteString(helpStyle.Render("↑/k: up | ↓/j: down | enter: details | /help: usage | r: refresh | q: quit"))

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
