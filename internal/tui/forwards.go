package tui

import (
	"fmt"
	"strings"

	"github.com/Oktanetflow/sshm/internal/config"
	"github.com/Oktanetflow/sshm/internal/sshclient"
	tea "github.com/charmbracelet/bubbletea"
)

var forwardTypes = []string{"local", "remote", "dynamic"}

type forwardModal struct {
	hostAlias string
	forwards  []config.PortForward
	selected  int
	adding    bool
	typeIdx   int // 0=local, 1=remote, 2=dynamic
	specInput string
	errMsg    string
}

func (m *model) openForwardsModal(h config.ResolvedHost) {
	fwds := make([]config.PortForward, len(h.Forwards))
	copy(fwds, h.Forwards)
	m.fwdModal = forwardModal{
		hostAlias: h.Alias,
		forwards:  fwds,
	}
	m.mode = modeForwards
}

func (m *model) persistForwards() error {
	h, _, ok := m.mgr.FindHost(m.fwdModal.hostAlias)
	if !ok {
		return fmt.Errorf("host %q not found", m.fwdModal.hostAlias)
	}
	h.Forwards = make([]config.PortForward, len(m.fwdModal.forwards))
	copy(h.Forwards, m.fwdModal.forwards)
	_, err := m.mgr.UpsertHost(h)
	return err
}

func (m model) updateForwards(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()
	if m.fwdModal.adding {
		switch k {
		case "esc":
			m.fwdModal.adding = false
			m.fwdModal.specInput = ""
			m.fwdModal.errMsg = ""
		case "ctrl+c":
			return m, tea.Quit
		case "tab", "right":
			m.fwdModal.typeIdx = (m.fwdModal.typeIdx + 1) % len(forwardTypes)
		case "shift+tab", "left":
			m.fwdModal.typeIdx = (m.fwdModal.typeIdx - 1 + len(forwardTypes)) % len(forwardTypes)
		case "backspace":
			if len(m.fwdModal.specInput) > 0 {
				r := []rune(m.fwdModal.specInput)
				m.fwdModal.specInput = string(r[:len(r)-1])
			}
		case "enter":
			spec := strings.TrimSpace(m.fwdModal.specInput)
			if spec == "" {
				m.fwdModal.errMsg = "Forward spec is required"
				return m, nil
			}
			var f config.PortForward
			var err error
			ft := forwardTypes[m.fwdModal.typeIdx]
			if ft == "dynamic" {
				f, err = sshclient.ParseDynamicSpec(spec)
			} else {
				f, err = sshclient.ParseLocalOrRemoteSpec(ft, spec)
			}
			if err != nil {
				m.fwdModal.errMsg = err.Error()
				return m, nil
			}
			m.fwdModal.forwards = append(m.fwdModal.forwards, f)
			if err := m.persistForwards(); err != nil {
				m.fwdModal.errMsg = err.Error()
				return m, nil
			}
			m.fwdModal.selected = len(m.fwdModal.forwards) - 1
			m.fwdModal.adding = false
			m.fwdModal.specInput = ""
			m.fwdModal.errMsg = ""
			m.rebuildRows()
			m.status = fmt.Sprintf("Added %s forward on %s", ft, m.fwdModal.hostAlias)
		default:
			if txt := extractPrintableRunes(msg); txt != "" {
				m.fwdModal.specInput += txt
			}
		}
		return m, nil
	}

	switch k {
	case "esc", "q":
		m.mode = modeList
	case "ctrl+c":
		return m, tea.Quit
	case "up", "k":
		if m.fwdModal.selected > 0 {
			m.fwdModal.selected--
		}
	case "down", "j":
		if m.fwdModal.selected < len(m.fwdModal.forwards)-1 {
			m.fwdModal.selected++
		}
	case "a", "n":
		m.fwdModal.adding = true
		m.fwdModal.specInput = ""
		m.fwdModal.errMsg = ""
	case "d", "x", "delete":
		if len(m.fwdModal.forwards) > 0 && m.fwdModal.selected < len(m.fwdModal.forwards) {
			idx := m.fwdModal.selected
			m.fwdModal.forwards = append(m.fwdModal.forwards[:idx], m.fwdModal.forwards[idx+1:]...)
			if m.fwdModal.selected >= len(m.fwdModal.forwards) && m.fwdModal.selected > 0 {
				m.fwdModal.selected--
			}
			if err := m.persistForwards(); err != nil {
				m.fwdModal.errMsg = err.Error()
				return m, nil
			}
			m.rebuildRows()
			m.status = fmt.Sprintf("Removed port forward on %s", m.fwdModal.hostAlias)
		}
	}
	return m, nil
}

func (m model) viewForwards() string {
	var b strings.Builder
	b.WriteString(m.styles.FormTitle.Render("Port Forwards — "+m.fwdModal.hostAlias) + "\n\n")

	if len(m.fwdModal.forwards) == 0 {
		b.WriteString(m.styles.Dim.Render("  No saved port forwards for this host. Press [a] to add one.") + "\n")
	} else {
		for i, f := range m.fwdModal.forwards {
			line := "  " + formatForwardShort(f)
			if i == m.fwdModal.selected && !m.fwdModal.adding {
				b.WriteString(m.styles.HostSelected.Render(padRightWidth("> "+formatForwardShort(f), 54)) + "\n")
			} else {
				b.WriteString(m.styles.HostNormal.Render(line) + "\n")
			}
		}
	}

	if m.fwdModal.adding {
		var types []string
		for i, t := range forwardTypes {
			if i == m.fwdModal.typeIdx {
				types = append(types, "[x] "+t)
			} else {
				types = append(types, "[ ] "+t)
			}
		}
		hint := "e.g. 5432:127.0.0.1:5432"
		if forwardTypes[m.fwdModal.typeIdx] == "dynamic" {
			hint = "e.g. 1080 or 127.0.0.1:1080"
		}
		b.WriteString("\n" + m.styles.FormLabel.Render("Type : ") + strings.Join(types, "  ") + "\n")
		b.WriteString(m.styles.FormActive.Render("Spec : ") + m.fwdModal.specInput + "_  " + m.styles.Dim.Render("("+hint+")") + "\n")
	}

	if m.fwdModal.errMsg != "" {
		b.WriteString("\n" + m.styles.ErrorText.Render("Error: "+m.fwdModal.errMsg) + "\n")
	}

	if m.fwdModal.adding {
		b.WriteString("\n" + m.styles.Footer.Render("[Tab/←→] Type  [Enter] Save Tunnel  [Esc] Cancel"))
	} else {
		b.WriteString("\n" + m.styles.Footer.Render("[a] Add Tunnel  [d] Remove  [↑↓] Select  [Esc] Back"))
	}
	b.WriteString("\n" + m.styles.Watermark.Render(config.AppCopyright))

	return "\n " + m.styles.FormBox.Render(b.String()) + "\n"
}
