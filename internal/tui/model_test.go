package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/Oktanetflow/sshm/internal/config"
	"github.com/Oktanetflow/sshm/internal/sshclient"
	tea "github.com/charmbracelet/bubbletea"
)

func TestExtractPrintableRunesFiltersWindowsNulBytes(t *testing.T) {
	// Windows coninput can emit NUL (0x00) for Shift/Alt/Ctrl keys
	nulMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0}}
	if got := extractPrintableRunes(nulMsg); got != "" {
		t.Fatalf("expected empty string for NUL rune, got %q", got)
	}

	mixed := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0, 'A', '\r', '1'}}
	if got := extractPrintableRunes(mixed); got != "A1" {
		t.Fatalf("expected 'A1', got %q", got)
	}

	space := tea.KeyMsg{Type: tea.KeySpace}
	if got := extractPrintableRunes(space); got != " " {
		t.Fatalf("expected single space, got %q", got)
	}
}

func TestAddHostFormIgnoresNulAndSelectsHostOnEnter(t *testing.T) {
	mgr, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	m := newModel(mgr, nil)
	// Open Add form
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = next.(model)

	// Type Alias with injected NUL bytes (simulating Windows Shift key)
	for _, msg := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{0}},
		{Type: tea.KeyRunes, Runes: []rune{'b', 'o', 'x'}},
		{Type: tea.KeyTab}, // Move to Host / IP
		{Type: tea.KeyRunes, Runes: []rune{0}},
		{Type: tea.KeyRunes, Runes: []rune("10.0.0.5")},
		{Type: tea.KeyEnter}, // Save
	} {
		next, _ = m.Update(msg)
		m = next.(model)
	}

	h, _, ok := mgr.FindHost("box")
	if !ok {
		t.Fatalf("expected saved host 'box'")
	}
	if h.Hostname != "10.0.0.5" {
		t.Fatalf("expected clean hostname '10.0.0.5', got %q", h.Hostname)
	}

	// Move down from group header to host row and press Enter -> should request ConnectAlias
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(model)
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)
	if m.result == nil || m.result.ConnectAlias != "box" {
		t.Fatalf("expected ConnectAlias='box', got %+v", m.result)
	}
	if cmd == nil {
		t.Fatalf("expected tea.Quit command on connect")
	}
}

func TestEditHostPreservesTagsNotesPinnedForwardsAndCredentialRef(t *testing.T) {
	mgr, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	orig, err := mgr.UpsertHost(config.Host{
		Alias:         "prod-api",
		Group:         "prod",
		Hostname:      "10.0.1.12",
		Port:          2222,
		User:          "deploy",
		Tags:          []string{"api", "linux"},
		Notes:         "Primary API",
		Pinned:        true,
		LastConnected: time.Now().UTC().Format(time.RFC3339),
		Forwards: []config.PortForward{
			{Type: "local", Bind: "127.0.0.1:8080", Target: "127.0.0.1:80"},
		},
	})
	if err != nil {
		t.Fatalf("UpsertHost: %v", err)
	}

	m := newModel(mgr, nil)
	// Select host row (index 1 after group header)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(model)
	// Open edit form and save immediately without modifying fields
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = next.(model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)

	after, _, ok := mgr.FindHost("prod-api")
	if !ok {
		t.Fatalf("expected host prod-api after edit")
	}
	if after.ID != orig.ID || after.CredentialRef != orig.CredentialRef {
		t.Fatalf("ID/CredentialRef changed on edit: orig=%s/%s after=%s/%s", orig.ID, orig.CredentialRef, after.ID, after.CredentialRef)
	}
	if !after.Pinned {
		t.Errorf("expected Pinned=true preserved")
	}
	if after.Notes != "Primary API" {
		t.Errorf("expected Notes preserved, got %q", after.Notes)
	}
	if len(after.Tags) != 2 || after.Tags[0] != "api" || after.Tags[1] != "linux" {
		t.Errorf("expected Tags preserved, got %v", after.Tags)
	}
	if len(after.Forwards) != 1 || after.Forwards[0].Bind != "127.0.0.1:8080" {
		t.Errorf("expected Forwards preserved, got %+v", after.Forwards)
	}
	if after.LastConnected != orig.LastConnected {
		t.Errorf("expected LastConnected preserved, got %q", after.LastConnected)
	}
}

func TestAgileTUIFeaturesWatermarkInspectorThemeProbesForwards(t *testing.T) {
	mgr, err := config.Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	saved, err := mgr.UpsertHost(config.Host{
		Alias:    "edge-01",
		Group:    "prod",
		Hostname: "10.10.0.1",
		User:     "root",
		Tags:     []string{"edge"},
		Notes:    "Jakarta PoP",
	})
	if err != nil {
		t.Fatalf("UpsertHost: %v", err)
	}

	m := newModel(mgr, nil)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = next.(model)

	// Feed probe result
	next, _ = m.Update(probeBatchMsg{results: []sshclient.ProbeResult{
		{HostID: saved.ID, Alias: saved.Alias, Status: sshclient.ProbeOnline, Latency: 12 * time.Millisecond},
	}})
	m = next.(model)

	// Move to host row
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = next.(model)

	view := m.View()
	for _, want := range []string{
		config.AppCopyright,
		"edge-01",
		"12ms",
		"Jakarta PoP",
		"#edge",
		"1 online",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("expected TUI view to contain %q, got:\n%s", want, view)
		}
	}

	// Cycle theme with 't'
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t'}})
	m = next.(model)
	if m.mgr.Config.UI.Theme != "minimal-light" {
		t.Fatalf("expected theme minimal-light, got %q", m.mgr.Config.UI.Theme)
	}

	// Open Forwards modal ('f'), add a local forward, and persist
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = next.(model)
	if m.mode != modeForwards {
		t.Fatalf("expected modeForwards, got %v", m.mode)
	}
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = next.(model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5432:127.0.0.1:5432")})
	m = next.(model)
	next, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = next.(model)

	updated, _, _ := mgr.FindHost("edge-01")
	if len(updated.Forwards) != 1 || updated.Forwards[0].Bind != "127.0.0.1:5432" {
		t.Fatalf("expected saved forward on edge-01, got %+v", updated.Forwards)
	}
}
