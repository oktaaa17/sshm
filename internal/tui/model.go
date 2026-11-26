package tui

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Oktanetflow/sshm/internal/config"
	"github.com/Oktanetflow/sshm/internal/credstore"
	"github.com/Oktanetflow/sshm/internal/sshclient"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AppVersion is populated by the CLI package so the TUI header matches --version.
var AppVersion = "1.0.0"

// extractPrintableRunes filters out NUL bytes (\x00) and control characters
// that Windows console input (coninput) emits for modifier keys like Shift/Alt/Ctrl.
func extractPrintableRunes(msg tea.KeyMsg) string {
	if msg.Type != tea.KeyRunes && msg.Type != tea.KeySpace {
		return ""
	}
	if msg.Type == tea.KeySpace {
		return " "
	}
	var b strings.Builder
	for _, r := range msg.Runes {
		if r >= 0x20 && r != 0x7f && unicode.IsPrint(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Result communicates the user's chosen action back to the CLI after TUI exit.
type Result struct {
	ConnectAlias string
}

type probeBatchMsg struct {
	results []sshclient.ProbeResult
}

type viewMode int

const (
	modeList viewMode = iota
	modeAddEdit
	modeDelete
	modeGroup
	modeForwards
	modeHelp
)

type rowKind int

const (
	rowGroup rowKind = iota
	rowHost
)

type listRow struct {
	kind  rowKind
	group string
	count int
	color string
	host  config.ResolvedHost
}

type hostForm struct {
	editingID     string
	credRef       string
	pinned        bool
	lastConnected string
	forwards      []config.PortForward
	isEdit        bool
	focus         int
	alias         string
	hostname      string
	port          string
	user          string
	group         string
	authIdx       int // 0=password, 1=key, 2=agent
	password      string
	identityFile  string
	savePassword  bool
	jumpHost      string
	tags          string
	notes         string
	errMsg        string
}

var authChoices = []string{"password", "key", "agent"}

type groupForm struct {
	focus          int
	name           string
	color          string
	defaultUser    string
	defaultPort    string
	defaultKeyFile string
	defaultJump    string
	errMsg         string
}

type model struct {
	mgr           *config.Manager
	store         credstore.Store
	styles        themeStyles
	mono          bool
	width         int
	height        int
	mode          viewMode
	search        string
	searching     bool
	tabIdx        int // 0 = All, 1..N = group names
	collapsed     map[string]bool
	showInspector bool
	probes        map[string]sshclient.ProbeResult
	probing       bool
	cursor        int
	rows          []listRow
	status        string
	form          hostForm
	gForm         groupForm
	fwdModal      forwardModal
	delTarget     config.ResolvedHost
	result        *Result
}

// Run launches the minimalist interactive TUI and returns the selected action (if any).
func Run(mgr *config.Manager, store credstore.Store, initialStatus ...string) (*Result, error) {
	m := newModel(mgr, store)
	if len(initialStatus) > 0 && initialStatus[0] != "" {
		m.status = initialStatus[0]
	}
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return nil, err
	}
	if fm, ok := finalModel.(model); ok {
		return fm.result, nil
	}
	return nil, nil
}

func newModel(mgr *config.Manager, store credstore.Store) model {
	m := model{
		mgr:           mgr,
		store:         store,
		styles:        newTheme(mgr.Config.UI.Theme),
		mono:          strings.EqualFold(mgr.Config.UI.Theme, "mono"),
		width:         110,
		height:        28,
		mode:          modeList,
		collapsed:     map[string]bool{},
		showInspector: true,
		probes:        map[string]sshclient.ProbeResult{},
	}
	if mgr.Config.UI.CollapseGroupsByDefault {
		for _, g := range mgr.AllGroupNames() {
			m.collapsed[g] = true
		}
	}
	m.rebuildRows()
	return m
}

func (m model) Init() tea.Cmd {
	if m.mgr.Config.UI.AutoProbeOnStart && len(m.mgr.HostsFile.Hosts) > 0 {
		return m.probeAllCmd()
	}
	return nil
}

func (m model) probeAllCmd() tea.Cmd {
	hosts := m.mgr.HostsInGroup("")
	if len(hosts) == 0 {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		results := sshclient.ProbeHosts(ctx, hosts, 1500*time.Millisecond, 8)
		return probeBatchMsg{results: results}
	}
}

func (m *model) markAllChecking() {
	for _, h := range m.mgr.HostsFile.Hosts {
		m.probes[h.ID] = sshclient.ProbeResult{
			HostID: h.ID,
			Alias:  h.Alias,
			Status: sshclient.ProbeChecking,
		}
	}
	m.probing = true
}

func (m *model) tabs() []string {
	t := []string{"All"}
	t = append(t, m.mgr.AllGroupNames()...)
	return t
}

func (m *model) rebuildRows() {
	tabs := m.tabs()
	if m.tabIdx >= len(tabs) {
		m.tabIdx = 0
	}
	activeGroup := ""
	if m.tabIdx > 0 {
		activeGroup = tabs[m.tabIdx]
	}

	query := strings.ToLower(strings.TrimSpace(m.search))
	grouped := map[string][]config.ResolvedHost{}
	var groupOrder []string

	if activeGroup != "" {
		groupOrder = []string{activeGroup}
	} else {
		groupOrder = m.mgr.AllGroupNames()
	}

	for _, g := range groupOrder {
		hosts := m.mgr.HostsInGroup(g)
		for _, h := range hosts {
			if query != "" && !matchesFuzzy(h, query) {
				continue
			}
			grouped[g] = append(grouped[g], h)
		}
	}

	rows := make([]listRow, 0, len(m.mgr.HostsFile.Hosts)+len(groupOrder))
	for _, g := range groupOrder {
		hs := grouped[g]
		if query != "" && len(hs) == 0 {
			continue
		}
		if len(hs) == 0 && activeGroup == "" && len(m.mgr.HostsFile.Hosts) > 0 {
			// Still show explicitly declared empty groups when not searching.
			if _, declared := m.mgr.HostsFile.Groups[g]; !declared {
				continue
			}
		}
		grpMeta := m.mgr.HostsFile.Groups[g]
		rows = append(rows, listRow{
			kind:  rowGroup,
			group: g,
			count: len(hs),
			color: grpMeta.Color,
		})
		if query != "" || !m.collapsed[g] {
			for _, h := range hs {
				rows = append(rows, listRow{
					kind:  rowHost,
					group: g,
					color: h.GroupColor,
					host:  h,
				})
			}
		}
	}

	m.rows = rows
	if len(m.rows) == 0 {
		m.cursor = 0
		return
	}
	if m.cursor >= len(m.rows) {
		m.cursor = len(m.rows) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	// Prefer landing on a host row when search is active so Enter connects immediately.
	if m.rows[m.cursor].kind == rowGroup && m.cursor+1 < len(m.rows) && m.rows[m.cursor+1].kind == rowHost {
		if query != "" {
			m.cursor++
		}
	}
}

func matchesFuzzy(h config.ResolvedHost, query string) bool {
	hay := strings.ToLower(strings.Join([]string{
		h.Alias,
		h.Hostname,
		h.EffectiveUser,
		h.Group,
		h.EffectiveJumpHost,
		strings.Join(h.Tags, " "),
		h.Notes,
	}, " "))
	if strings.Contains(hay, query) {
		return true
	}
	// Subsequence fuzzy check for quick typing (e.g. "pa1" -> "prod-api-01")
	qi := 0
	qRunes := []rune(query)
	for _, r := range hay {
		if qi < len(qRunes) && r == qRunes[qi] {
			qi++
		}
	}
	return qi == len(qRunes)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case probeBatchMsg:
		m.probing = false
		online := 0
		for _, r := range msg.results {
			m.probes[r.HostID] = r
			if r.Status == sshclient.ProbeOnline {
				online++
			}
		}
		if len(msg.results) > 0 {
			m.status = fmt.Sprintf("Probed %d host(s): %d online", len(msg.results), online)
		}
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case modeList:
			return m.updateList(msg)
		case modeAddEdit:
			return m.updateAddEdit(msg)
		case modeDelete:
			return m.updateDelete(msg)
		case modeGroup:
			return m.updateGroup(msg)
		case modeForwards:
			return m.updateForwards(msg)
		case modeHelp:
			if msg.String() == "esc" || msg.String() == "?" || msg.String() == "q" || msg.String() == "enter" {
				m.mode = modeList
			}
			return m, nil
		}
	}
	return m, nil
}

func (m model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := msg.String()

	if m.searching {
		switch k {
		case "esc":
			m.searching = false
			m.search = ""
			m.rebuildRows()
		case "enter":
			m.searching = false
			if len(m.rows) > 0 && m.rows[m.cursor].kind == rowHost {
				m.result = &Result{ConnectAlias: m.rows[m.cursor].host.Alias}
				return m, tea.Quit
			}
		case "backspace":
			if len(m.search) > 0 {
				r := []rune(m.search)
				m.search = string(r[:len(r)-1])
				m.rebuildRows()
			} else {
				m.searching = false
			}
		case "up", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "ctrl+n":
			if m.cursor < len(m.rows)-1 {
				m.cursor++
			}
		case "ctrl+c":
			return m, tea.Quit
		default:
			if txt := extractPrintableRunes(msg); txt != "" {
				m.search += txt
				m.rebuildRows()
			}
		}
		return m, nil
	}

	m.status = ""
	switch k {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "esc":
		if m.search != "" {
			m.search = ""
			m.rebuildRows()
		}
	case "/":
		m.searching = true
	case "?":
		m.mode = modeHelp
	case "i":
		m.showInspector = !m.showInspector
		if m.showInspector {
			m.status = "Host inspector enabled"
		} else {
			m.status = "Host inspector hidden"
		}
	case "t":
		next := nextThemeName(m.mgr.Config.UI.Theme)
		m.mgr.Config.UI.Theme = next
		m.styles = newTheme(next)
		m.mono = strings.EqualFold(next, "mono")
		_ = m.mgr.SaveConfig()
		m.status = fmt.Sprintf("Theme switched to %s", next)
	case "r":
		if len(m.mgr.HostsFile.Hosts) > 0 {
			m.markAllChecking()
			m.status = "Probing host reachability..."
			return m, m.probeAllCmd()
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rows)-1 {
			m.cursor++
		}
	case "tab":
		tabs := m.tabs()
		if len(tabs) > 0 {
			m.tabIdx = (m.tabIdx + 1) % len(tabs)
			m.cursor = 0
			m.rebuildRows()
		}
	case "shift+tab":
		tabs := m.tabs()
		if len(tabs) > 0 {
			m.tabIdx = (m.tabIdx - 1 + len(tabs)) % len(tabs)
			m.cursor = 0
			m.rebuildRows()
		}
	case " ", "left", "right":
		if len(m.rows) > 0 {
			row := m.rows[m.cursor]
			if row.kind == rowGroup {
				if k == "left" {
					m.collapsed[row.group] = true
				} else if k == "right" {
					m.collapsed[row.group] = false
				} else {
					m.collapsed[row.group] = !m.collapsed[row.group]
				}
				m.rebuildRows()
			}
		}
	case "enter":
		if len(m.rows) > 0 {
			row := m.rows[m.cursor]
			if row.kind == rowGroup {
				m.collapsed[row.group] = !m.collapsed[row.group]
				m.rebuildRows()
			} else {
				m.result = &Result{ConnectAlias: row.host.Alias}
				return m, tea.Quit
			}
		}
	case "a":
		m.openAddForm()
	case "e":
		if len(m.rows) > 0 && m.rows[m.cursor].kind == rowHost {
			m.openEditForm(m.rows[m.cursor].host)
		}
	case "f":
		if len(m.rows) > 0 && m.rows[m.cursor].kind == rowHost {
			m.openForwardsModal(m.rows[m.cursor].host)
		}
	case "d":
		if len(m.rows) > 0 && m.rows[m.cursor].kind == rowHost {
			m.delTarget = m.rows[m.cursor].host
			m.mode = modeDelete
		}
	case "p":
		if len(m.rows) > 0 && m.rows[m.cursor].kind == rowHost {
			h, _, ok := m.mgr.FindHost(m.rows[m.cursor].host.Alias)
			if ok {
				h.Pinned = !h.Pinned
				if _, err := m.mgr.UpsertHost(h); err == nil {
					m.rebuildRows()
					if h.Pinned {
						m.status = fmt.Sprintf("Pinned %s", h.Alias)
					} else {
						m.status = fmt.Sprintf("Unpinned %s", h.Alias)
					}
				}
			}
		}
	case "g":
		m.openGroupForm()
	}
	return m, nil
}

func (m *model) openAddForm() {
	grp := m.mgr.Config.General.DefaultGroup
	tabs := m.tabs()
	if m.tabIdx > 0 && m.tabIdx < len(tabs) {
		grp = tabs[m.tabIdx]
	}
	m.form = hostForm{
		isEdit:       false,
		port:         "22",
		group:        grp,
		authIdx:      0, // Password default
		savePassword: m.mgr.Config.General.SavePasswordByDefault,
	}
	m.mode = modeAddEdit
}

func (m *model) openEditForm(h config.ResolvedHost) {
	authIdx := 0
	switch strings.ToLower(h.Auth) {
	case "key":
		authIdx = 1
	case "agent":
		authIdx = 2
	}
	portStr := strconv.Itoa(h.EffectivePort)
	fwdsCopy := make([]config.PortForward, len(h.Forwards))
	copy(fwdsCopy, h.Forwards)

	m.form = hostForm{
		editingID:     h.ID,
		credRef:       h.CredentialRef,
		pinned:        h.Pinned,
		lastConnected: h.LastConnected,
		forwards:      fwdsCopy,
		isEdit:        true,
		alias:         h.Alias,
		hostname:      h.Hostname,
		port:          portStr,
		user:          h.User,
		group:         h.Group,
		authIdx:       authIdx,
		identityFile:  h.IdentityFile,
		savePassword:  h.EffectiveSavePassword,
		jumpHost:      h.JumpHost,
		tags:          strings.Join(h.Tags, ", "),
		notes:         h.Notes,
	}
	m.mode = modeAddEdit
}

func (m model) updateAddEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const totalFields = 11
	k := msg.String()
	switch k {
	case "esc":
		m.mode = modeList
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "tab", "down":
		m.form.focus = (m.form.focus + 1) % totalFields
		return m, nil
	case "shift+tab", "up":
		m.form.focus = (m.form.focus - 1 + totalFields) % totalFields
		return m, nil
	case "enter":
		if m.saveHostForm() {
			m.mode = modeList
			m.rebuildRows()
		}
		return m, nil
	case "ctrl+s":
		if m.saveHostForm() {
			m.result = &Result{ConnectAlias: m.form.alias}
			return m, tea.Quit
		}
		return m, nil
	}

	switch m.form.focus {
	case 5: // Auth type toggle
		if k == " " || k == "right" {
			m.form.authIdx = (m.form.authIdx + 1) % len(authChoices)
		} else if k == "left" {
			m.form.authIdx = (m.form.authIdx - 1 + len(authChoices)) % len(authChoices)
		}
	case 7: // Save password toggle
		if k == " " || k == "left" || k == "right" {
			m.form.savePassword = !m.form.savePassword
		}
	default:
		ptr := m.activeHostFieldPtr()
		if ptr != nil {
			if k == "backspace" {
				if len(*ptr) > 0 {
					r := []rune(*ptr)
					*ptr = string(r[:len(r)-1])
				}
			} else if txt := extractPrintableRunes(msg); txt != "" {
				*ptr += txt
			}
		}
	}
	return m, nil
}

func (m *model) activeHostFieldPtr() *string {
	switch m.form.focus {
	case 0:
		return &m.form.alias
	case 1:
		return &m.form.hostname
	case 2:
		return &m.form.port
	case 3:
		return &m.form.user
	case 4:
		return &m.form.group
	case 6:
		if authChoices[m.form.authIdx] == "key" {
			return &m.form.identityFile
		}
		if authChoices[m.form.authIdx] == "agent" {
			return nil
		}
		return &m.form.password
	case 8:
		return &m.form.jumpHost
	case 9:
		return &m.form.tags
	case 10:
		return &m.form.notes
	default:
		return nil
	}
}

func parseTagsCSV(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func (m *model) saveHostForm() bool {
	alias := strings.TrimSpace(m.form.alias)
	hostname := strings.TrimSpace(m.form.hostname)
	if alias == "" {
		m.form.errMsg = "Alias is required"
		return false
	}
	if hostname == "" {
		m.form.errMsg = "Host / IP is required"
		return false
	}
	port := 22
	if strings.TrimSpace(m.form.port) != "" {
		p, err := strconv.Atoi(strings.TrimSpace(m.form.port))
		if err != nil || p <= 0 || p > 65535 {
			m.form.errMsg = "Port must be between 1 and 65535"
			return false
		}
		port = p
	}

	h := config.Host{
		ID:            m.form.editingID,
		CredentialRef: m.form.credRef,
		Pinned:        m.form.pinned,
		LastConnected: m.form.lastConnected,
		Forwards:      m.form.forwards,
		Alias:         alias,
		Hostname:      hostname,
		Port:          port,
		User:          strings.TrimSpace(m.form.user),
		Group:         strings.TrimSpace(m.form.group),
		Auth:          authChoices[m.form.authIdx],
		IdentityFile:  strings.TrimSpace(m.form.identityFile),
		SavePassword:  config.BoolPtr(m.form.savePassword),
		JumpHost:      strings.TrimSpace(m.form.jumpHost),
		Tags:          parseTagsCSV(m.form.tags),
		Notes:         strings.TrimSpace(m.form.notes),
	}

	saved, err := m.mgr.UpsertHost(h)
	if err != nil {
		m.form.errMsg = err.Error()
		return false
	}

	if m.form.password != "" && m.form.savePassword && m.store != nil {
		pw := []byte(m.form.password)
		if err := m.store.Set(saved.CredentialRef, pw); err != nil {
			credstore.ZeroBytes(pw)
			m.form.errMsg = "Saved host, but failed storing password: " + err.Error()
			return false
		}
		credstore.ZeroBytes(pw)
	}
	m.form.password = ""
	m.form.alias = saved.Alias
	m.status = fmt.Sprintf("Saved host %s", saved.Alias)
	return true
}

func (m model) updateDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch strings.ToLower(msg.String()) {
	case "y", "enter":
		deleted, err := m.mgr.DeleteHost(m.delTarget.Alias)
		if err == nil {
			if m.store != nil && deleted.CredentialRef != "" {
				_ = m.store.Delete(deleted.CredentialRef)
				_ = m.store.Delete(credstore.KeyPassphraseRef(deleted.ID))
			}
			delete(m.probes, deleted.ID)
			m.status = fmt.Sprintf("Deleted host %s", deleted.Alias)
		}
		m.mode = modeList
		m.rebuildRows()
	case "n", "esc":
		m.mode = modeList
	case "ctrl+c":
		return m, tea.Quit
	}
	return m, nil
}

func (m *model) openGroupForm() {
	gName := ""
	if len(m.rows) > 0 {
		gName = m.rows[m.cursor].group
	}
	grp := m.mgr.HostsFile.Groups[gName]
	portStr := ""
	if grp.DefaultPort > 0 {
		portStr = strconv.Itoa(grp.DefaultPort)
	}
	m.gForm = groupForm{
		name:           gName,
		color:          grp.Color,
		defaultUser:    grp.DefaultUser,
		defaultPort:    portStr,
		defaultKeyFile: grp.DefaultIdentityFile,
		defaultJump:    grp.DefaultJump,
	}
	m.mode = modeGroup
}

func (m model) updateGroup(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	const totalFields = 6
	k := msg.String()
	switch k {
	case "esc":
		m.mode = modeList
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "tab", "down":
		m.gForm.focus = (m.gForm.focus + 1) % totalFields
		return m, nil
	case "shift+tab", "up":
		m.gForm.focus = (m.gForm.focus - 1 + totalFields) % totalFields
		return m, nil
	case "enter":
		name := strings.TrimSpace(m.gForm.name)
		if name == "" {
			m.gForm.errMsg = "Group name is required"
			return m, nil
		}
		port := 0
		if strings.TrimSpace(m.gForm.defaultPort) != "" {
			p, err := strconv.Atoi(strings.TrimSpace(m.gForm.defaultPort))
			if err != nil || p <= 0 || p > 65535 {
				m.gForm.errMsg = "Invalid default port"
				return m, nil
			}
			port = p
		}
		err := m.mgr.CreateOrUpdateGroup(name, config.Group{
			Color:               strings.TrimSpace(m.gForm.color),
			DefaultUser:         strings.TrimSpace(m.gForm.defaultUser),
			DefaultPort:         port,
			DefaultIdentityFile: strings.TrimSpace(m.gForm.defaultKeyFile),
			DefaultJump:         strings.TrimSpace(m.gForm.defaultJump),
		})
		if err != nil {
			m.gForm.errMsg = err.Error()
			return m, nil
		}
		m.status = fmt.Sprintf("Saved group %s", name)
		m.mode = modeList
		m.rebuildRows()
		return m, nil
	}

	var ptr *string
	switch m.gForm.focus {
	case 0:
		ptr = &m.gForm.name
	case 1:
		ptr = &m.gForm.color
	case 2:
		ptr = &m.gForm.defaultUser
	case 3:
		ptr = &m.gForm.defaultPort
	case 4:
		ptr = &m.gForm.defaultKeyFile
	case 5:
		ptr = &m.gForm.defaultJump
	}
	if ptr != nil {
		if k == "backspace" {
			if len(*ptr) > 0 {
				r := []rune(*ptr)
				*ptr = string(r[:len(r)-1])
			}
		} else if txt := extractPrintableRunes(msg); txt != "" {
			*ptr += txt
		}
	}
	return m, nil
}

func (m model) View() string {
	switch m.mode {
	case modeAddEdit:
		return m.viewAddEdit()
	case modeDelete:
		return m.viewDelete()
	case modeGroup:
		return m.viewGroup()
	case modeForwards:
		return m.viewForwards()
	case modeHelp:
		return m.viewHelp()
	default:
		return m.viewList()
	}
}

func (m model) countProbeStats() (total, groups, online int) {
	total = len(m.mgr.HostsFile.Hosts)
	groups = len(m.mgr.AllGroupNames())
	for _, h := range m.mgr.HostsFile.Hosts {
		if p, ok := m.probes[h.ID]; ok && p.Status == sshclient.ProbeOnline {
			online++
		}
	}
	return total, groups, online
}

func (m model) viewList() string {
	w := m.width
	if w < 64 {
		w = 64
	}
	divider := m.styles.Divider.Render(" " + strings.Repeat("─", w-2))

	var b strings.Builder

	// Header with Agile Summary Stats
	totalHosts, totalGroups, onlineHosts := m.countProbeStats()
	statsStr := fmt.Sprintf("  %d hosts · %d groups", totalHosts, totalGroups)
	if len(m.probes) > 0 {
		if m.probing {
			statsStr += " · probing..."
		} else {
			statsStr += fmt.Sprintf(" · %d online", onlineHosts)
		}
	}
	left := m.styles.HeaderTitle.Render(" sshm v"+AppVersion) + m.styles.HeaderStats.Render(statsStr)
	right := m.styles.HeaderMeta.Render("[?] Help  [a] Add  [g] Group  [t] Theme  [r] Probe  [q] Quit ")
	pad := w - lipgloss.Width(left) - lipgloss.Width(right)
	if pad < 1 {
		pad = 1
	}
	b.WriteString(left + strings.Repeat(" ", pad) + right + "\n")
	b.WriteString(divider + "\n")

	// Search + Group Tabs
	searchCursor := ""
	if m.searching {
		searchCursor = "_"
	}
	searchPart := m.styles.SearchPrompt.Render(" > search: ") + m.styles.SearchText.Render(m.search+searchCursor)

	var tabParts []string
	for i, name := range m.tabs() {
		if i == m.tabIdx {
			tabParts = append(tabParts, m.styles.TabActive.Render(name))
		} else {
			tabParts = append(tabParts, m.styles.TabInactive.Render(name))
		}
	}
	tabStr := strings.Join(tabParts, " ")
	b.WriteString(searchPart + "    " + tabStr + "\n\n")

	// Split-pane geometry (Inspector auto-shown when terminal width >= 100 and enabled)
	useSplit := m.showInspector && w >= 100
	listWidth := w
	inspectorWidth := 0
	if useSplit {
		inspectorWidth = w * 38 / 100
		if inspectorWidth < 36 {
			inspectorWidth = 36
		}
		if inspectorWidth > 52 {
			inspectorWidth = 52
		}
		listWidth = w - inspectorWidth - 1
	}

	// List body
	maxRows := m.height - 9
	if maxRows < 5 {
		maxRows = 5
	}
	start := 0
	if m.cursor >= maxRows {
		start = m.cursor - maxRows + 1
	}
	end := start + maxRows
	if end > len(m.rows) {
		end = len(m.rows)
	}

	var listBuilder strings.Builder
	if len(m.rows) == 0 {
		if len(m.mgr.HostsFile.Hosts) == 0 {
			listBuilder.WriteString(m.styles.Dim.Render("   No saved hosts yet. Press [a] to add your first SSH server.\n"))
		} else {
			listBuilder.WriteString(m.styles.Dim.Render("   No matching hosts. Press [Esc] to clear search.\n"))
		}
	} else {
		for i := start; i < end; i++ {
			row := m.rows[i]
			selected := i == m.cursor
			if row.kind == rowGroup {
				arrow := "▼"
				if m.collapsed[row.group] && strings.TrimSpace(m.search) == "" {
					arrow = "▶"
				}
				line := fmt.Sprintf(" %s %s (%d)", arrow, row.group, row.count)
				if selected {
					listBuilder.WriteString(m.styles.HostSelected.Render(padRightWidth(line, listWidth-2)) + "\n")
				} else {
					listBuilder.WriteString(m.styles.GroupHeader.Render(line) + "\n")
				}
			} else {
				listBuilder.WriteString(m.renderHostRow(row.host, selected, listWidth) + "\n")
			}
		}
	}

	if useSplit {
		leftPane := lipgloss.NewStyle().Width(listWidth).Render(strings.TrimSuffix(listBuilder.String(), "\n"))
		rightPane := m.renderInspector(inspectorWidth)
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane) + "\n")
	} else {
		b.WriteString(listBuilder.String())
	}

	// Footer & persistent Oktanetflow Watermark
	b.WriteString("\n" + divider + "\n")
	if m.status != "" {
		b.WriteString(m.styles.StatusText.Render(" "+m.status) + "\n")
	}
	footer := " [Enter] Connect  [Tab] Group  [/] Search  [i] Inspector  [e] Edit  [f] Forwards  [d] Delete  [p] Pin"
	b.WriteString(m.styles.Footer.Render(truncateWidth(footer, w-1)) + "\n")

	backendName := "Secure Store"
	if m.store != nil {
		backendName = m.store.BackendName()
	}
	wmLeft := " " + config.AppCopyright
	wmRight := fmt.Sprintf("Theme: %s  ·  Store: %s ", m.mgr.Config.UI.Theme, backendName)
	wmPad := w - lipgloss.Width(wmLeft) - lipgloss.Width(wmRight)
	if wmPad < 2 {
		b.WriteString(m.styles.Watermark.Render(wmLeft) + "\n")
	} else {
		b.WriteString(m.styles.Watermark.Render(wmLeft+strings.Repeat(" ", wmPad)+wmRight) + "\n")
	}

	return b.String()
}

func (m model) renderHostRow(h config.ResolvedHost, selected bool, width int) string {
	dot := groupDot(h.GroupColor, m.mono)
	pin := " "
	if h.Pinned {
		pin = m.styles.PinStar.Render("★")
	}

	endpoint := fmt.Sprintf("%s@%s", h.EffectiveUser, h.Hostname)
	if h.EffectivePort != 22 {
		endpoint += fmt.Sprintf(":%d", h.EffectivePort)
	}

	authTag := "[pw]"
	authStyle := m.styles.BadgePW
	switch strings.ToLower(h.Auth) {
	case "key":
		authTag = "[key]"
		authStyle = m.styles.BadgeKey
	case "agent":
		authTag = "[agt]"
		authStyle = m.styles.BadgeKey
	}

	probeBadge := m.formatProbeBadge(h.ID, selected)

	aliasW := 16
	epW := 24
	if width < 72 {
		aliasW = 13
		epW = 19
	}

	fwdMark := ""
	if len(h.Forwards) > 0 {
		fwdMark = fmt.Sprintf(" ⇄%d", len(h.Forwards))
	}

	extra := ""
	if h.EffectiveJumpHost != "" {
		extra = "via " + h.EffectiveJumpHost
	} else if m.mgr.Config.UI.ShowLastConnected && h.LastConnected != "" {
		extra = formatRelativeTime(h.LastConnected)
	}

	if selected {
		plain := fmt.Sprintf("   ●%s %s %s %-5s %-8s%s %s",
			boolStar(h.Pinned),
			padRightWidth(truncateWidth(h.Alias, aliasW), aliasW),
			padRightWidth(truncateWidth(endpoint, epW), epW),
			authTag,
			probeBadge,
			fwdMark,
			extra,
		)
		return m.styles.HostSelected.Render(padRightWidth(truncateWidth(plain, width-2), width-2))
	}

	fwdStyled := ""
	if fwdMark != "" {
		fwdStyled = m.styles.ForwardBadge.Render(fwdMark)
	}

	return fmt.Sprintf("   %s%s %s %s %s %s%s %s",
		dot,
		pin,
		m.styles.HostNormal.Render(padRightWidth(truncateWidth(h.Alias, aliasW), aliasW)),
		m.styles.HostDetail.Render(padRightWidth(truncateWidth(endpoint, epW), epW)),
		authStyle.Render(padRightWidth(authTag, 5)),
		padRightWidth(probeBadge, 8),
		fwdStyled,
		m.styles.Dim.Render(truncateWidth(extra, 14)),
	)
}

func (m model) viewAddEdit() string {
	title := "Add Host"
	if m.form.isEdit {
		title = "Edit Host — " + m.form.alias
	}

	backendName := "Secure Store"
	if m.store != nil {
		backendName = m.store.BackendName()
	}

	authDisplay := make([]string, len(authChoices))
	for i, c := range authChoices {
		if i == m.form.authIdx {
			authDisplay[i] = "[x] " + c
		} else {
			authDisplay[i] = "[ ] " + c
		}
	}

	saveCheck := "[ ] No"
	if m.form.savePassword {
		saveCheck = "[x] Yes (" + backendName + ")"
	}

	authFieldLabel := "Password"
	authFieldDisplay := strings.Repeat("•", len([]rune(m.form.password)))
	if m.form.isEdit && m.form.password == "" {
		authFieldDisplay = "(unchanged — type to update)"
	}
	if authChoices[m.form.authIdx] == "key" {
		authFieldLabel = "Key Path"
		authFieldDisplay = m.form.identityFile
	} else if authChoices[m.form.authIdx] == "agent" {
		authFieldLabel = "Agent"
		authFieldDisplay = "(uses Windows OpenSSH agent)"
	}

	fields := []struct {
		label string
		val   string
	}{
		{"Alias", m.form.alias},
		{"Host / IP", m.form.hostname},
		{"Port", m.form.port},
		{"User", m.form.user},
		{"Group", m.form.group},
		{"Auth Type", strings.Join(authDisplay, "   ")},
		{authFieldLabel, authFieldDisplay},
		{"Save Pass?", saveCheck},
		{"Jump Host", m.form.jumpHost},
		{"Tags (csv)", m.form.tags},
		{"Notes", m.form.notes},
	}

	var b strings.Builder
	b.WriteString(m.styles.FormTitle.Render(title) + "\n\n")
	for i, f := range fields {
		cursor := "  "
		labelStyle := m.styles.FormLabel
		val := f.val
		if i == m.form.focus {
			cursor = "> "
			labelStyle = m.styles.FormActive
			if i != 5 && i != 7 && !(i == 6 && authChoices[m.form.authIdx] == "agent") {
				if !(i == 6 && m.form.isEdit && m.form.password == "" && authChoices[m.form.authIdx] == "password") {
					val += "_"
				}
			}
		}
		b.WriteString(fmt.Sprintf("%s%s : %s\n", cursor, labelStyle.Render(padRightWidth(f.label, 12)), val))
	}

	if m.form.errMsg != "" {
		b.WriteString("\n" + m.styles.ErrorText.Render("Error: "+m.form.errMsg) + "\n")
	}
	b.WriteString("\n" + m.styles.Footer.Render("[Tab/↑↓] Field  [Space] Toggle  [Enter] Save  [Ctrl+S] Save & Connect  [Esc] Cancel"))
	b.WriteString("\n" + m.styles.Watermark.Render(config.AppCopyright))

	return "\n " + m.styles.FormBox.Render(b.String()) + "\n"
}

func (m model) viewDelete() string {
	var b strings.Builder
	b.WriteString(m.styles.FormTitle.Render("Confirm Delete") + "\n\n")
	b.WriteString(fmt.Sprintf("Delete host %q (%s@%s) and its saved credentials?\n\n", m.delTarget.Alias, m.delTarget.EffectiveUser, m.delTarget.Hostname))
	b.WriteString(m.styles.Footer.Render("[y/Enter] Delete   [n/Esc] Cancel"))
	b.WriteString("\n" + m.styles.Watermark.Render(config.AppCopyright))
	return "\n " + m.styles.FormBox.Render(b.String()) + "\n"
}

func (m model) viewGroup() string {
	fields := []struct {
		label string
		val   string
	}{
		{"Group Name", m.gForm.name},
		{"Color", m.gForm.color},
		{"Default User", m.gForm.defaultUser},
		{"Default Port", m.gForm.defaultPort},
		{"Default Key", m.gForm.defaultKeyFile},
		{"Default Jump", m.gForm.defaultJump},
	}

	var b strings.Builder
	b.WriteString(m.styles.FormTitle.Render("Create / Edit Group") + "\n\n")
	for i, f := range fields {
		cursor := "  "
		labelStyle := m.styles.FormLabel
		val := f.val
		if i == m.gForm.focus {
			cursor = "> "
			labelStyle = m.styles.FormActive
			val += "_"
		}
		hint := ""
		if i == 1 {
			hint = m.styles.Dim.Render("  (red, yellow, green, blue, cyan, magenta)")
		}
		b.WriteString(fmt.Sprintf("%s%s : %s%s\n", cursor, labelStyle.Render(padRightWidth(f.label, 14)), val, hint))
	}
	if m.gForm.errMsg != "" {
		b.WriteString("\n" + m.styles.ErrorText.Render("Error: "+m.gForm.errMsg) + "\n")
	}
	b.WriteString("\n" + m.styles.Footer.Render("[Tab/↑↓] Field  [Enter] Save  [Esc] Cancel"))
	b.WriteString("\n" + m.styles.Watermark.Render(config.AppCopyright))
	return "\n " + m.styles.FormBox.Render(b.String()) + "\n"
}

func (m model) viewHelp() string {
	help := fmt.Sprintf(`sshm v%s — Keybindings

  ↑ / ↓ or j / k   Navigate hosts and groups
  Enter            Connect to host (or toggle group)
  Space / ← / →    Collapse / expand group
  /                Fuzzy search hosts, IPs, users, groups, tags, notes
  Tab / Shift+Tab  Switch group filter tabs
  i                Toggle Split-Pane Host Inspector
  r                Refresh live TCP reachability & latency probes
  t                Switch UI theme (minimal-dark / minimal-light / mono)
  a                Add new host (Save Password enabled by default)
  e                Edit selected host (tags, notes, auth, jump)
  f                Manage saved port forwards (-L / -R / -D) on host
  d                Delete selected host and saved credentials
  p                Pin / unpin selected host
  g                Create or edit a group & inherited defaults
  q / Ctrl+C       Quit

%s
Press [Esc] or [?] to close.`, AppVersion, config.AppCopyright)
	return "\n " + m.styles.FormBox.Render(help) + "\n"
}

func boolStar(b bool) string {
	if b {
		return "★"
	}
	return " "
}

func truncateWidth(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	if maxW == 1 {
		return "…"
	}
	var b strings.Builder
	cur := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if cur+rw > maxW-1 {
			break
		}
		b.WriteRune(r)
		cur += rw
	}
	b.WriteRune('…')
	return b.String()
}

func padRightWidth(s string, width int) string {
	w := lipgloss.Width(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

func formatRelativeTime(iso string) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return ""
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 48*time.Hour:
		return "yesterday"
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}
