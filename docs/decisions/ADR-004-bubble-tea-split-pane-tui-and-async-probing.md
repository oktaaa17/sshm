# ADR-004: Bubble Tea Split-Pane Agile TUI & Bounded Async TCP Probing

## Status
Accepted

## Date
2026-11-26

## Context
SSH host managers typically force a painful trade-off:
- Minimalist plain lists hide essential operational context (effective user/port inherited from a group, jump host, live host reachability, active tunnels, tags, and notes), forcing operators to run separate commands or guess before connecting.
- Heavy GUI clients consume high RAM, break keyboard-driven workflows, and feel disconnected from Windows Terminal and PowerShell.
- Synchronously pinging or probing every host on startup stalls launch time whenever a single host is offline or behind an inactive VPN.

## Decision
Build the interactive TUI (`internal/tui/`) on **Charmbracelet Bubble Tea (`tea.Model`) + Lip Gloss** with:
1. **Responsive Split-Pane Host & Group Inspector (`internal/tui/inspector.go`):** Automatically displays side-by-side when terminal width ≥ 100 columns (toggleable anytime via `i`) and falls back gracefully to a single-column layout on narrow windows.
2. **Bounded Async TCP Reachability Prober (`internal/sshclient/probe.go`):** Launches non-blocking Bubble Tea `tea.Cmd` workers with bounded concurrency and a short TCP timeout (`1500ms`) on startup (configurable via `ui.auto_probe_on_start`) and on manual `[r]` refresh, rendering live latency badges (`● 14ms`, `○ ...`, `● off`) and header online counters.
3. **Inline Operational Modals:**
   - Add/Edit Host form (`a` / `e`) with Save Password enabled by default, inline Tags & Notes, and strict preservation of unedited fields (`Pinned`, `LastConnected`, `Forwards`, `CredentialRef`).
   - Interactive Port Forward Manager (`f`, `internal/tui/forwards.go`) for local (`-L`), remote (`-R`), and dynamic SOCKS5 (`-D`) tunnels.
   - Live Theme Switcher (`t`) cycling `minimal-dark`, `minimal-light`, and `mono` with instant persistence to `config.toml`.
4. **Accurate Unicode Width Rendering:** Compute all padding and truncation with `lipgloss.Width` / `runewidth` so icons (`●`, `★`, `⇄`) and CJK/Unicode host notes never corrupt border alignment.

## Alternatives Considered

### Pure CLI (`cobra` only, no TUI)
- **Pros:** Minimal code and zero terminal-rendering edge cases.
- **Cons:** Slow daily host discovery across dozens of grouped servers; harder to inspect inherited group defaults, tunnels, and live reachability at a glance.
- **Rejected:** `sshm` is designed as a dual CLI + TUI tool — fast CLI aliases for scripting/power users, agile TUI for interactive navigation.

### Native Windows GUI (Electron / Wails / WinUI)
- **Pros:** Rich graphical widgets.
- **Cons:** High memory footprint (100–300 MB), large installer, breaks terminal-first workflows, and requires embedding a custom terminal emulator.
- **Rejected:** Violates the single lightweight pure-Go binary design.

### Synchronous reachability checks before rendering TUI
- **Pros:** Simpler state machine.
- **Cons:** Introduces 1.5–5 second startup lag whenever any host is unreachable.
- **Rejected:** Instant `< 50ms` TUI launch is non-negotiable; Elm-architecture async `tea.Cmd` messages keep the UI 100% responsive.

## Consequences
- **Positive:** Instant startup, full keyboard ergonomics (`j`/`k`, `/`, `Tab`, `i`, `f`, `t`, `r`, `p`), and high testability via pure Bubble Tea state-machine unit tests (`internal/tui/model_test.go`).
- **Positive:** Three built-in themes (`minimal-dark`, `minimal-light`, `mono`) accommodate dark terminals, light terminals, and high-contrast/no-color accessibility needs.
- **Negative:** TCP probing only tests port-level TCP reachability (not full SSH handshake), which can trigger firewall/IDS logs in strict networks — mitigated by exposing `sshm config set auto_probe_on_start false`.
