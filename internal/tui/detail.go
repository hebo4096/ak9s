package tui

import (
	"fmt"
	"strings"

	"github.com/hebo4096/ak9s/internal/azure"
)

type detailView struct {
	cluster  azure.Cluster
	scroll   int
	maxLines int
}

func newDetailView(cluster azure.Cluster) detailView {
	return detailView{
		cluster:  cluster,
		maxLines: 30,
		scroll:   0,
	}
}

func (d *detailView) up() {
	if d.scroll > 0 {
		d.scroll--
	}
}

func (d *detailView) down() {
	d.scroll++
}

func (d *detailView) render(width int) string {
	var lines []string
	c := d.cluster

	// General info
	lines = append(lines, sectionStyle.Render("  General"))
	lines = append(lines, renderField("  Name", c.Name))
	lines = append(lines, renderField("  Resource Group", c.ResourceGroup))
	lines = append(lines, renderField("  Resource URI", c.ResourceID))
	lines = append(lines, renderField("  Subscription", fmt.Sprintf("%s (%s)", c.SubscriptionName, c.SubscriptionID)))
	lines = append(lines, renderField("  Location", c.Location))
	lines = append(lines, renderField("  Kubernetes Version", c.KubernetesVersion))

	state := c.ProvisioningState
	if state != "" && state != "Succeeded" {
		lines = append(lines, renderFieldRaw("  Provisioning State", stoppedStyle.Render(state)))
		if c.ProvisioningError != "" {
			errText := c.ProvisioningError
			// Wrap error message to fit within terminal width
			labelWidth := 22 // "  Operation Error: " label width
			maxLen := width - labelWidth
			if maxLen < 20 {
				maxLen = 60
			}
			for i := 0; i < len(errText); i += maxLen {
				end := i + maxLen
				if end > len(errText) {
					end = len(errText)
				}
				chunk := errText[i:end]
				if i == 0 {
					lines = append(lines, renderFieldRaw("  Operation Error", stoppedStyle.Render(chunk)))
				} else {
					lines = append(lines, "                      "+stoppedStyle.Render(chunk))
				}
			}
		} else {
			lines = append(lines, renderFieldRaw("  Error", statusStyle.Render("(loading...)")))
		}
	} else {
		lines = append(lines, renderField("  Provisioning State", state))
	}

	powerLine := c.PowerState
	if c.PowerState == "Running" {
		powerLine = runningStyle.Render(c.PowerState)
	} else if c.PowerState != "" {
		powerLine = stoppedStyle.Render(c.PowerState)
	}
	lines = append(lines, renderFieldRaw("  Power State", powerLine))

	lines = append(lines, renderField("  FQDN", c.FQDN))
	lines = append(lines, renderField("  Total Nodes", fmt.Sprintf("%d", c.NodeCount)))
	lines = append(lines, renderField("  SKU / Tier", fmt.Sprintf("%s / %s", c.SKU, c.Tier)))
	lines = append(lines, "")

	// Network
	lines = append(lines, sectionStyle.Render("  Network"))
	lines = append(lines, renderField("  Network Plugin", c.NetworkPlugin))
	networkPolicy := c.NetworkPolicy
	if networkPolicy == "" {
		networkPolicy = "none"
	}
	lines = append(lines, renderField("  Network Policy", networkPolicy))
	networkDataplane := c.NetworkDataplane
	if networkDataplane == "" {
		networkDataplane = "none"
	}
	lines = append(lines, renderField("  Network Dataplane", networkDataplane))
	lines = append(lines, renderField("  Service CIDR", c.ServiceCIDR))
	lines = append(lines, renderField("  DNS Service IP", c.DNSServiceIP))
	lines = append(lines, renderField("  Pod CIDR", c.PodCIDR))
	lines = append(lines, "")

	// Node pools
	lines = append(lines, sectionStyle.Render("  Node Pools"))
	for _, np := range c.NodePools {
		poolState := np.PowerState
		if np.PowerState == "Running" {
			poolState = runningStyle.Render(np.PowerState)
		} else if np.PowerState != "" {
			poolState = stoppedStyle.Render(np.PowerState)
		}

		lines = append(lines, "")
		lines = append(lines, headerStyle.Render(fmt.Sprintf("    [%s] (%s)", np.Name, np.Mode)))
		lines = append(lines, renderField("    VM Size", np.VMSize))
		lines = append(lines, renderField("    Count", fmt.Sprintf("%d (min: %d, max: %d)", np.Count, np.MinCount, np.MaxCount)))
		lines = append(lines, renderField("    OS Type", np.OSType))
		lines = append(lines, renderField("    OS Disk Size", fmt.Sprintf("%d GB", np.OSDiskSizeGB)))
		lines = append(lines, renderFieldRaw("    Power State", poolState))
		lines = append(lines, renderField("    K8s Version", np.K8sVersion))
		vnetSubnet := np.VnetSubnet
		if vnetSubnet == "" {
			vnetSubnet = "none (auto-generated)"
		}
		lines = append(lines, renderField("    VNet Subnet", vnetSubnet))
		podSubnet := np.PodSubnet
		if podSubnet == "" {
			podSubnet = "none"
		}
		lines = append(lines, renderField("    Pod Subnet", podSubnet))
	}

	// Tags
	if len(c.Tags) > 0 {
		lines = append(lines, "")
		lines = append(lines, sectionStyle.Render("  Tags"))
		for k, v := range c.Tags {
			lines = append(lines, renderField("    "+k, v))
		}
	}

	// Addons
	lines = append(lines, "")
	lines = append(lines, sectionStyle.Render("  Addons"))
	if len(c.Addons) > 0 {
		for _, addon := range c.Addons {
			lines = append(lines, valueStyle.Render("    • "+addon))
		}
	} else {
		lines = append(lines, statusStyle.Render("    No addons enabled"))
	}

	// Extensions
	lines = append(lines, "")
	lines = append(lines, sectionStyle.Render("  Extensions"))
	if len(c.Extensions) > 0 {
		for _, ext := range c.Extensions {
			lines = append(lines, valueStyle.Render("    • "+ext))
		}
	} else {
		lines = append(lines, statusStyle.Render("    No extensions installed"))
	}

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("esc/backspace: back | ↑/k: scroll up | ↓/j: scroll down | q: quit"))

	// Apply scroll - cap so last page always shows maxLines rows
	maxScroll := len(lines) - d.maxLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if d.scroll > maxScroll {
		d.scroll = maxScroll
	}
	if d.scroll < 0 {
		d.scroll = 0
	}

	end := d.scroll + d.maxLines
	if end > len(lines) {
		end = len(lines)
	}

	visible := lines[d.scroll:end]

	// Truncate lines to terminal width to prevent wrapping
	if width > 0 {
		for i, line := range visible {
			visible[i] = truncateVisual(line, width)
		}
	}

	return strings.Join(visible, "\n")
}

func renderField(label, value string) string {
	return labelStyle.Render(label+":") + " " + valueStyle.Render(value)
}

func renderFieldRaw(label, value string) string {
	return labelStyle.Render(label+":") + " " + value
}

func isLetter(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

// truncateVisual truncates a string to maxWidth visible characters,
// preserving ANSI escape codes.
func truncateVisual(s string, maxWidth int) string {
	visibleLen := 0
	result := make([]byte, 0, len(s))
	i := 0
	for i < len(s) {
		// Check for ANSI escape sequence
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !isLetter(s[j]) {
				j++
			}
			if j < len(s) {
				j++ // include the final letter
			}
			result = append(result, s[i:j]...)
			i = j
			continue
		}
		if visibleLen >= maxWidth {
			break
		}
		result = append(result, s[i])
		visibleLen++
		i++
	}
	// Append any remaining ANSI reset codes
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && !isLetter(s[j]) {
				j++
			}
			if j < len(s) {
				j++
			}
			result = append(result, s[i:j]...)
			i = j
		} else {
			i++
		}
	}
	return string(result)
}
