package config

import (
	"crypto/rand"
	"encoding/base32"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/pelletier/go-toml/v2"
)

const (
	DefaultGroupName = "default"
	DefaultPort      = 22
	AppPublisher     = "Oktanetflow"
	AppCopyright     = "Copyright © 2026 Oktanetflow"
)

// sanitizeField strips control characters (including NUL bytes) and surrounding whitespace.
func sanitizeField(s string) string {
	cleaned := strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f || !unicode.IsPrint(r) {
			return -1
		}
		return r
	}, s)
	return strings.TrimSpace(cleaned)
}

// AppConfig represents global settings in config.toml.
type AppConfig struct {
	General GeneralConfig `toml:"general"`
	SSH     SSHConfig     `toml:"ssh"`
	UI      UIConfig      `toml:"ui"`
}

type GeneralConfig struct {
	DefaultGroup          string `toml:"default_group"`
	SavePasswordByDefault bool   `toml:"save_password_by_default"`
	CredentialBackend     string `toml:"credential_backend"` // "wincred" | "dpapi" | "vault"
	KnownHostsPath        string `toml:"known_hosts_path"`
	StrictHostKeyChecking string `toml:"strict_host_key_checking"` // "ask" | "yes" | "no"
}

type SSHConfig struct {
	ConnectTimeoutSec    int  `toml:"connect_timeout_sec"`
	KeepaliveIntervalSec int  `toml:"keepalive_interval_sec"`
	KeepaliveCountMax    int  `toml:"keepalive_count_max"`
	UseSSHAgent          bool `toml:"use_ssh_agent"`
}

type UIConfig struct {
	Theme                   string `toml:"theme"` // "minimal-dark" | "minimal-light" | "mono"
	ShowLastConnected       bool   `toml:"show_last_connected"`
	CollapseGroupsByDefault bool   `toml:"collapse_groups_by_default"`
	AutoProbeOnStart        bool   `toml:"auto_probe_on_start"`
}

// Group defines a logical group of hosts with optional inheritable defaults.
type Group struct {
	Color               string `toml:"color,omitempty"`
	DefaultUser         string `toml:"default_user,omitempty"`
	DefaultPort         int    `toml:"default_port,omitempty"`
	DefaultIdentityFile string `toml:"default_identity_file,omitempty"`
	DefaultJump         string `toml:"default_jump,omitempty"`
}

// PortForward defines a local, remote, or dynamic SOCKS5 tunnel.
type PortForward struct {
	Type   string `toml:"type"` // "local" | "remote" | "dynamic"
	Bind   string `toml:"bind"`
	Target string `toml:"target,omitempty"`
}

// Host represents a saved SSH host profile. Never stores plaintext passwords.
type Host struct {
	ID            string        `toml:"id"`
	Alias         string        `toml:"alias"`
	Group         string        `toml:"group"`
	Hostname      string        `toml:"hostname"`
	Port          int           `toml:"port,omitempty"`
	User          string        `toml:"user,omitempty"`
	Auth          string        `toml:"auth,omitempty"` // "password" | "key" | "agent" | "auto"
	CredentialRef string        `toml:"credential_ref,omitempty"`
	SavePassword  *bool         `toml:"save_password,omitempty"`
	IdentityFile  string        `toml:"identity_file,omitempty"`
	JumpHost      string        `toml:"jump_host,omitempty"`
	Tags          []string      `toml:"tags,omitempty"`
	Notes         string        `toml:"notes,omitempty"`
	Pinned        bool          `toml:"pinned,omitempty"`
	LastConnected string        `toml:"last_connected,omitempty"`
	Forwards      []PortForward `toml:"forwards,omitempty"`
}

// HostsFile represents hosts.toml.
type HostsFile struct {
	Groups map[string]Group `toml:"groups"`
	Hosts  []Host           `toml:"hosts"`
}

// ResolvedHost is a Host with group inheritance and global defaults applied.
type ResolvedHost struct {
	Host
	EffectivePort         int
	EffectiveUser         string
	EffectiveIdentityFile string
	EffectiveJumpHost     string
	EffectiveSavePassword bool
	GroupColor            string
}

// Manager handles loading, resolving, and persisting config.toml and hosts.toml.
type Manager struct {
	mu        sync.Mutex
	Dir       string
	Config    AppConfig
	HostsFile HostsFile
}

// DefaultAppConfig returns sensible Windows-first defaults.
func DefaultAppConfig() AppConfig {
	return AppConfig{
		General: GeneralConfig{
			DefaultGroup:          DefaultGroupName,
			SavePasswordByDefault: true,
			CredentialBackend:     defaultBackendForOS(),
			KnownHostsPath:        "~/.ssh/known_hosts",
			StrictHostKeyChecking: "ask",
		},
		SSH: SSHConfig{
			ConnectTimeoutSec:    10,
			KeepaliveIntervalSec: 30,
			KeepaliveCountMax:    3,
			UseSSHAgent:          true,
		},
		UI: UIConfig{
			Theme:                   "minimal-dark",
			ShowLastConnected:       true,
			CollapseGroupsByDefault: false,
			AutoProbeOnStart:        true,
		},
	}
}

func defaultBackendForOS() string {
	if os.PathSeparator == '\\' {
		return "wincred"
	}
	return "vault"
}

// ResolveConfigDir returns the directory for sshm config files.
// Respects SSHM_CONFIG_DIR if set; otherwise uses %APPDATA%\sshm (Windows) or user config dir.
func ResolveConfigDir() (string, error) {
	if env := strings.TrimSpace(os.Getenv("SSHM_CONFIG_DIR")); env != "" {
		return env, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		home, herr := os.UserHomeDir()
		if herr != nil {
			return "", fmt.Errorf("cannot determine config directory: %w", err)
		}
		return filepath.Join(home, ".sshm"), nil
	}
	return filepath.Join(base, "sshm"), nil
}

// ExpandPath expands leading ~ to the current user's home directory.
func ExpandPath(p string) string {
	if p == "" {
		return ""
	}
	if p == "~" || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			if p == "~" {
				return home
			}
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// GenerateID generates a sortable, unique 20-char Crockford-like uppercase ID.
func GenerateID() string {
	var buf [12]byte
	ms := time.Now().UnixMilli()
	if ms < 0 {
		ms = 0
	}
	var ts [8]byte
	binary.BigEndian.PutUint64(ts[:], uint64(ms))
	copy(buf[:6], ts[2:])
	_, _ = rand.Read(buf[6:])
	enc := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf[:])
	return enc
}

// BoolPtr returns a pointer to b.
func BoolPtr(b bool) *bool {
	return &b
}

// Load loads config.toml and hosts.toml from dir (or default dir if empty), creating defaults if absent.
func Load(dir string) (*Manager, error) {
	if dir == "" {
		var err error
		dir, err = ResolveConfigDir()
		if err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("failed to create config dir %s: %w", dir, err)
	}
	_ = RestrictDirToCurrentUser(dir)

	m := &Manager{
		Dir:    dir,
		Config: DefaultAppConfig(),
		HostsFile: HostsFile{
			Groups: map[string]Group{},
			Hosts:  []Host{},
		},
	}

	cfgPath := filepath.Clean(m.ConfigPath())
	if data, err := os.ReadFile(cfgPath); err == nil { // #nosec G304 -- config file inside resolved sshm config directory
		if err := toml.Unmarshal(data, &m.Config); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", cfgPath, err)
		}
		// Fill zero-values with safe defaults.
		if m.Config.General.DefaultGroup == "" {
			m.Config.General.DefaultGroup = DefaultGroupName
		}
		if m.Config.General.CredentialBackend == "" {
			m.Config.General.CredentialBackend = defaultBackendForOS()
		}
		if m.Config.General.KnownHostsPath == "" {
			m.Config.General.KnownHostsPath = "~/.ssh/known_hosts"
		}
		if m.Config.General.StrictHostKeyChecking == "" {
			m.Config.General.StrictHostKeyChecking = "ask"
		}
		if m.Config.SSH.ConnectTimeoutSec <= 0 {
			m.Config.SSH.ConnectTimeoutSec = 10
		}
		if m.Config.SSH.KeepaliveIntervalSec < 0 {
			m.Config.SSH.KeepaliveIntervalSec = 30
		}
		if m.Config.SSH.KeepaliveCountMax <= 0 {
			m.Config.SSH.KeepaliveCountMax = 3
		}
		if m.Config.UI.Theme == "" {
			m.Config.UI.Theme = "minimal-dark"
		}
	} else if errors.Is(err, os.ErrNotExist) {
		if err := m.SaveConfig(); err != nil {
			return nil, err
		}
	} else {
		return nil, err
	}

	hostsPath := filepath.Clean(m.HostsPath())
	if data, err := os.ReadFile(hostsPath); err == nil { // #nosec G304 -- hosts file inside resolved sshm config directory
		if err := toml.Unmarshal(data, &m.HostsFile); err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", hostsPath, err)
		}
		if m.HostsFile.Groups == nil {
			m.HostsFile.Groups = map[string]Group{}
		}
		for i := range m.HostsFile.Hosts {
			m.HostsFile.Hosts[i].Alias = sanitizeField(m.HostsFile.Hosts[i].Alias)
			m.HostsFile.Hosts[i].Hostname = sanitizeField(m.HostsFile.Hosts[i].Hostname)
			m.HostsFile.Hosts[i].User = sanitizeField(m.HostsFile.Hosts[i].User)
			m.HostsFile.Hosts[i].Group = sanitizeField(m.HostsFile.Hosts[i].Group)
			m.HostsFile.Hosts[i].JumpHost = sanitizeField(m.HostsFile.Hosts[i].JumpHost)
			m.HostsFile.Hosts[i].IdentityFile = sanitizeField(m.HostsFile.Hosts[i].IdentityFile)
			m.HostsFile.Hosts[i].Notes = sanitizeField(m.HostsFile.Hosts[i].Notes)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	return m, nil
}

func (m *Manager) ConfigPath() string {
	return filepath.Join(m.Dir, "config.toml")
}

func (m *Manager) HostsPath() string {
	return filepath.Join(m.Dir, "hosts.toml")
}

func (m *Manager) SaveConfig() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveConfigLocked()
}

func (m *Manager) saveConfigLocked() error {
	data, err := toml.Marshal(&m.Config)
	if err != nil {
		return err
	}
	return WriteFileAtomic(m.ConfigPath(), data, 0o600)
}

func (m *Manager) SaveHosts() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveHostsLocked()
}

func (m *Manager) saveHostsLocked() error {
	data, err := toml.Marshal(&m.HostsFile)
	if err != nil {
		return err
	}
	return WriteFileAtomic(m.HostsPath(), data, 0o600)
}

// WriteFileAtomic writes data to a unique temporary file in the same directory and atomically replaces path.
func WriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, ".sshm-tmp-*")
	if err != nil {
		return err
	}
	tmp := tmpFile.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmp)
		}
	}()

	if _, err := tmpFile.Write(data); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Sync(); err != nil {
		_ = tmpFile.Close()
		return err
	}
	if err := tmpFile.Close(); err != nil {
		return err
	}
	_ = os.Chmod(tmp, perm)

	if err := os.Rename(tmp, path); err != nil {
		// On Windows, os.Rename may fail if destination exists on older filesystems; fallback.
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			return err2
		}
	}
	cleanup = false
	return nil
}

// ResolveHost applies group defaults and global settings to a host.
func (m *Manager) ResolveHost(h Host) ResolvedHost {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.resolveHostLocked(h)
}

func (m *Manager) resolveHostLocked(h Host) ResolvedHost {
	groupName := h.Group
	if groupName == "" {
		groupName = m.Config.General.DefaultGroup
	}
	grp := m.HostsFile.Groups[groupName]

	effPort := h.Port
	if effPort <= 0 {
		if grp.DefaultPort > 0 {
			effPort = grp.DefaultPort
		} else {
			effPort = DefaultPort
		}
	}

	effUser := h.User
	if effUser == "" {
		effUser = grp.DefaultUser
	}
	if effUser == "" {
		effUser = currentOSUser()
	}

	effKey := h.IdentityFile
	if effKey == "" {
		effKey = grp.DefaultIdentityFile
	}

	effJump := h.JumpHost
	if effJump == "" {
		effJump = grp.DefaultJump
	}

	effSave := m.Config.General.SavePasswordByDefault
	if h.SavePassword != nil {
		effSave = *h.SavePassword
	}

	resolved := ResolvedHost{
		Host:                  h,
		EffectivePort:         effPort,
		EffectiveUser:         effUser,
		EffectiveIdentityFile: ExpandPath(effKey),
		EffectiveJumpHost:     effJump,
		EffectiveSavePassword: effSave,
		GroupColor:            grp.Color,
	}
	if resolved.Group == "" {
		resolved.Group = groupName
	}
	return resolved
}

func currentOSUser() string {
	for _, k := range []string{"USERNAME", "USER"} {
		if v := os.Getenv(k); v != "" {
			return v
		}
	}
	return "root"
}

// FindHost looks up a host by alias (case-insensitive fallback).
func (m *Manager) FindHost(alias string) (Host, int, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.findHostLocked(alias)
}

func (m *Manager) findHostLocked(alias string) (Host, int, bool) {
	for i, h := range m.HostsFile.Hosts {
		if h.Alias == alias {
			return h, i, true
		}
	}
	for i, h := range m.HostsFile.Hosts {
		if strings.EqualFold(h.Alias, alias) {
			return h, i, true
		}
	}
	return Host{}, -1, false
}

// UpsertHost adds or updates a host by ID or Alias.
func (m *Manager) UpsertHost(h Host) (Host, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	h.Alias = sanitizeField(h.Alias)
	h.Hostname = sanitizeField(h.Hostname)
	h.User = sanitizeField(h.User)
	h.Group = sanitizeField(h.Group)
	h.JumpHost = sanitizeField(h.JumpHost)
	h.IdentityFile = sanitizeField(h.IdentityFile)
	h.Notes = sanitizeField(h.Notes)
	if len(h.Tags) > 0 {
		cleanTags := make([]string, 0, len(h.Tags))
		for _, t := range h.Tags {
			if ct := sanitizeField(t); ct != "" {
				cleanTags = append(cleanTags, ct)
			}
		}
		h.Tags = cleanTags
	}
	if h.Alias == "" {
		return h, errors.New("host alias is required")
	}
	if h.Hostname == "" {
		return h, errors.New("hostname is required")
	}
	if h.Group == "" {
		h.Group = m.Config.General.DefaultGroup
	}

	// Match existing host BEFORE generating a new ID/CredentialRef so existing credentials are never orphaned.
	existingIdx := -1
	for i, existing := range m.HostsFile.Hosts {
		if (h.ID != "" && existing.ID == h.ID) || strings.EqualFold(existing.Alias, h.Alias) {
			existingIdx = i
			break
		}
	}
	if existingIdx >= 0 {
		existing := m.HostsFile.Hosts[existingIdx]
		h.ID = existing.ID
		if h.CredentialRef == "" {
			h.CredentialRef = existing.CredentialRef
		}
		if h.LastConnected == "" {
			h.LastConnected = existing.LastConnected
		}
	}

	if h.ID == "" {
		h.ID = GenerateID()
	}
	if h.CredentialRef == "" {
		h.CredentialRef = "sshm:" + h.ID
	}
	if h.Auth == "" {
		if h.IdentityFile != "" {
			h.Auth = "key"
		} else {
			h.Auth = "password"
		}
	}
	if h.SavePassword == nil {
		h.SavePassword = BoolPtr(m.Config.General.SavePasswordByDefault)
	}

	if _, exists := m.HostsFile.Groups[h.Group]; !exists {
		m.HostsFile.Groups[h.Group] = Group{}
	}

	if existingIdx >= 0 {
		m.HostsFile.Hosts[existingIdx] = h
	} else {
		m.HostsFile.Hosts = append(m.HostsFile.Hosts, h)
	}
	return h, m.saveHostsLocked()
}

// DeleteHost removes a host by alias and returns the deleted Host.
func (m *Manager) DeleteHost(alias string) (Host, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	h, idx, ok := m.findHostLocked(alias)
	if !ok {
		return Host{}, fmt.Errorf("host %q not found", alias)
	}
	m.HostsFile.Hosts = append(m.HostsFile.Hosts[:idx], m.HostsFile.Hosts[idx+1:]...)
	return h, m.saveHostsLocked()
}

// TouchLastConnected updates the host's last_connected timestamp safely across concurrent goroutines.
func (m *Manager) TouchLastConnected(alias string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, idx, ok := m.findHostLocked(alias)
	if !ok {
		return nil
	}
	m.HostsFile.Hosts[idx].LastConnected = time.Now().UTC().Format(time.RFC3339)
	return m.saveHostsLocked()
}

// AllGroupNames returns sorted group names across declared groups and host references.
func (m *Manager) AllGroupNames() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	set := map[string]struct{}{}
	for name := range m.HostsFile.Groups {
		set[name] = struct{}{}
	}
	for _, h := range m.HostsFile.Hosts {
		g := h.Group
		if g == "" {
			g = m.Config.General.DefaultGroup
		}
		set[g] = struct{}{}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// HostsInGroup returns all hosts belonging to groupName (or all hosts if groupName is "").
func (m *Manager) HostsInGroup(groupName string) []ResolvedHost {
	m.mu.Lock()
	defer m.mu.Unlock()

	var out []ResolvedHost
	for _, h := range m.HostsFile.Hosts {
		r := m.resolveHostLocked(h)
		if groupName == "" || strings.EqualFold(r.Group, groupName) {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Pinned != out[j].Pinned {
			return out[i].Pinned
		}
		return strings.ToLower(out[i].Alias) < strings.ToLower(out[j].Alias)
	})
	return out
}

// CreateOrUpdateGroup creates or updates group metadata.
func (m *Manager) CreateOrUpdateGroup(name string, g Group) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("group name is required")
	}
	m.HostsFile.Groups[name] = g
	return m.saveHostsLocked()
}

// RenameGroup renames a group and updates all hosts belonging to it.
func (m *Manager) RenameGroup(oldName, newName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldName = strings.TrimSpace(oldName)
	newName = strings.TrimSpace(newName)
	if oldName == "" || newName == "" {
		return errors.New("both old and new group names are required")
	}
	g, ok := m.HostsFile.Groups[oldName]
	hasHosts := false
	for i := range m.HostsFile.Hosts {
		if m.HostsFile.Hosts[i].Group == oldName {
			m.HostsFile.Hosts[i].Group = newName
			hasHosts = true
		}
	}
	if !ok && !hasHosts {
		return fmt.Errorf("group %q not found", oldName)
	}
	delete(m.HostsFile.Groups, oldName)
	m.HostsFile.Groups[newName] = g
	return m.saveHostsLocked()
}

// DeleteGroup deletes a group. If deleteHosts is true, returns deleted hosts; otherwise moves hosts to default group.
func (m *Manager) DeleteGroup(name string, deleteHosts bool) ([]Host, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("group name is required")
	}
	delete(m.HostsFile.Groups, name)
	var removed []Host
	kept := make([]Host, 0, len(m.HostsFile.Hosts))
	for _, h := range m.HostsFile.Hosts {
		if strings.EqualFold(h.Group, name) {
			if deleteHosts {
				removed = append(removed, h)
				continue
			}
			h.Group = m.Config.General.DefaultGroup
		}
		kept = append(kept, h)
	}
	m.HostsFile.Hosts = kept
	return removed, m.saveHostsLocked()
}
