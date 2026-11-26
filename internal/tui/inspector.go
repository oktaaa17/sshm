package tui

import (
	"fmt"
	"strings"

	"github.com/oktaaa17/sshm/internal/config"
	"github.com/oktaaa17/sshm/internal/sshclient"
)

func (m model) renderInspector(panelWidth int) string {
	innerW := panelWidth - 4
	if innerW < 24 {
		innerW = 24
	}
	lineSep := m.styles.Divider.Render(strings.Repeat("─", innerW))

	var b strings.Builder
	if len(m.rows) == 0 {
		b.WriteString(m.styles.InspectorTitle.Render("Inspector") + "\n")
		b.WriteString(lineSep + "\n")
		b.WriteString(m.styles.Dim.Render("Select a host or group\nto inspect full details.") + "\n")
		return m.styles.InspectorBox.Width(panelWidth).Render(b.String())
	}

	row := m.rows[m.cursor]
	if row.kind == rowGroup {
		grp := m.mgr.HostsFile.Groups[row.group]
		hosts := m.mgr.HostsInGroup(row.group)
		onlineCount := 0
		for _, h := range hosts {
			if p, ok := m.probes[h.ID]; ok && p.Status == sshclient.ProbeOnline {
				onlineCount++
			}
		}

		b.WriteString(m.styles.InspectorTitle.Render("Group — "+truncateWidth(row.group, innerW-8)) + "\n")
		b.WriteString(lineSep + "\n")
		b.WriteString(m.kv("Hosts", fmt.Sprintf("%d total (%d online)", len(hosts), onlineCount), innerW))
		colorLabel := grp.Color
		if colorLabel == "" {
			colorLabel = "default"
		}
		b.WriteString(m.kv("Badge", groupDot(grp.Color, m.mono)+" "+colorLabel, innerW))
		b.WriteString(m.kv("Def User", fallbackDash(grp.DefaultUser), innerW))
		portStr := "22 (default)"
		if grp.DefaultPort > 0 {
			portStr = fmt.Sprintf("%d", grp.DefaultPort)
		}
		b.WriteString(m.kv("Def Port", portStr, innerW))
		b.WriteString(m.kv("Def Key", fallbackDash(grp.DefaultIdentityFile), innerW))
		b.WriteString(m.kv("Def Jump", fallbackDash(grp.DefaultJump), innerW))
		b.WriteString("\n" + m.styles.Dim.Render("[g] Edit group  [Space] Fold") + "\n")
		return m.styles.InspectorBox.Width(panelWidth).Render(b.String())
	}

	h := row.host
	b.WriteString(m.styles.InspectorTitle.Render("Host — "+truncateWidth(h.Alias, innerW-7)) + "\n")
	b.WriteString(lineSep + "\n")

	endpoint := fmt.Sprintf("%s@%s:%d", h.EffectiveUser, h.Hostname, h.EffectivePort)
	b.WriteString(m.kv("Endpoint", endpoint, innerW))

	grpText := groupDot(h.GroupColor, m.mono) + " " + h.Group
	if h.Pinned {
		grpText += "  " + m.styles.PinStar.Render("★ Pinned")
	}
	b.WriteString(m.kv("Group", grpText, innerW))

	backend := "Secure Store"
	if m.store != nil {
		backend = m.store.BackendName()
	}
	authInfo := h.Auth
	if authInfo == "" {
		authInfo = "password"
	}
	if strings.EqualFold(authInfo, "password") && h.EffectiveSavePassword {
		authInfo += " (" + truncateWidth(backend, 18) + ")"
	} else if strings.EqualFold(authInfo, "key") && h.EffectiveIdentityFile != "" {
		authInfo += " (" + truncateWidth(h.EffectiveIdentityFile, 18) + ")"
	}
	b.WriteString(m.kv("Auth", authInfo, innerW))

	jumpVal := "direct"
	if h.EffectiveJumpHost != "" {
		jumpVal = "via " + h.EffectiveJumpHost
		if h.JumpHost == "" {
			jumpVal += " (inherited)"
		}
	}
	b.WriteString(m.kv("Jump", jumpVal, innerW))

	b.WriteString(m.kv("Status", m.formatProbeDetail(h.ID), innerW))

	if len(h.Tags) > 0 {
		var tags []string
		for _, t := range h.Tags {
			tags = append(tags, m.styles.TagBadge.Render("#"+t))
		}
		b.WriteString(m.kv("Tags", strings.Join(tags, " "), innerW))
	} else {
		b.WriteString(m.kv("Tags", m.styles.Dim.Render("—"), innerW))
	}

	if len(h.Forwards) > 0 {
		b.WriteString(m.kv("Tunnels", fmt.Sprintf("%d configured ([f] edit)", len(h.Forwards)), innerW))
		maxShow := 3
		if len(h.Forwards) < maxShow {
			maxShow = len(h.Forwards)
		}
		for i := 0; i < maxShow; i++ {
			b.WriteString("  " + m.styles.ForwardBadge.Render(truncateWidth(formatForwardShort(h.Forwards[i]), innerW-2)) + "\n")
		}
		if len(h.Forwards) > maxShow {
			b.WriteString("  " + m.styles.Dim.Render(fmt.Sprintf("+%d more...", len(h.Forwards)-maxShow)) + "\n")
		}
	} else {
		b.WriteString(m.kv("Tunnels", m.styles.Dim.Render("none ([f] add)"), innerW))
	}

	if h.Notes != "" {
		b.WriteString(m.kv("Notes", h.Notes, innerW))
	}

	lastSeen := "never"
	if h.LastConnected != "" {
		if rel := formatRelativeTime(h.LastConnected); rel != "" {
			lastSeen = rel
		}
	}
	b.WriteString(m.kv("Last Seen", lastSeen, innerW))

	b.WriteString("\n" + m.styles.Dim.Render("[Enter] Connect  [e] Edit  [f] Tunnels") + "\n")
	return m.styles.InspectorBox.Width(panelWidth).Render(b.String())
}

func (m model) kv(label, val string, maxW int) string {
	valW := maxW - 12
	if valW < 10 {
		valW = 10
	}
	return fmt.Sprintf("%s %s\n",
		m.styles.InspectorKey.Render(padRightWidth(label+":", 10)),
		m.styles.InspectorVal.Render(truncateWidth(val, valW)),
	)
}

func (m model) formatProbeBadge(hostID string, selected bool) string {
	p, ok := m.probes[hostID]
	if !ok || p.Status == sshclient.ProbeUnknown {
		if selected {
			return "·  --"
		}
		return m.styles.Dim.Render("·  --")
	}
	switch p.Status {
	case sshclient.ProbeChecking:
		if selected {
			return "○ ..."
		}
		return m.styles.ProbeChecking.Render("○ ...")
	case sshclient.ProbeOnline:
		ms := p.Latency.Milliseconds()
		if ms < 1 {
			ms = 1
		}
		txt := fmt.Sprintf("● %dms", ms)
		if selected {
			return txt
		}
		return m.styles.ProbeOnline.Render(txt)
	default:
		if selected {
			return "● off"
		}
		return m.styles.ProbeOffline.Render("● off")
	}
}

func (m model) formatProbeDetail(hostID string) string {
	p, ok := m.probes[hostID]
	if !ok || p.Status == sshclient.ProbeUnknown {
		return m.styles.Dim.Render("Not checked ([r] probe)")
	}
	switch p.Status {
	case sshclient.ProbeChecking:
		return m.styles.ProbeChecking.Render("Checking TCP reachability...")
	case sshclient.ProbeOnline:
		ms := p.Latency.Milliseconds()
		if ms < 1 {
			ms = 1
		}
		return m.styles.ProbeOnline.Render(fmt.Sprintf("● Online (%dms TCP)", ms))
	default:
		return m.styles.ProbeOffline.Render("● Offline / unreachable")
	}
}

func formatForwardShort(f config.PortForward) string {
	switch strings.ToLower(f.Type) {
	case "local":
		return fmt.Sprintf("L %s -> %s", f.Bind, f.Target)
	case "remote":
		return fmt.Sprintf("R %s -> %s", f.Bind, f.Target)
	case "dynamic":
		return fmt.Sprintf("D %s (SOCKS5)", f.Bind)
	default:
		return fmt.Sprintf("%s %s", f.Type, f.Bind)
	}
}

func fallbackDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}
