# Spec: sshm Documentation, ADRs, Visual Mockups, Illustrations, Guidelines, Use Cases & Demo Videos

> **Product:** `sshm` (`sshm.exe`) — Agile CLI/TUI SSH Client for Windows  
> **Publisher / Copyright:** `Copyright © 2026 Oktanetflow`  
> **Target Repository:** `https://github.com/oktaaa17/sshm` (branch: `main`)  
> **Standards:** Spec-Driven Development (`/spec-driven-development`) & Documentation and ADRs (`/documentation-and-adrs`)

---

## 1. Objective

Build and publish a comprehensive, production-grade documentation suite for **`sshm` by Oktanetflow** that captures both **how** to operate `sshm` and **why** its architecture was engineered this way.

### Target Audience
- **Windows DevOps / SRE / SysAdmin Engineers** evaluating or adopting `sshm` as their daily CLI/TUI SSH client.
- **Security & Enterprise Reviewers** auditing credential storage, NTFS ACLs, Win32 PE packaging, and Defender/SmartScreen posture.
- **Contributors & AI Agents** onboarding to the codebase who need Architecture Decision Records (ADRs), design rationales, and project rules (`CLAUDE.md`).

### Deliverables
1. **Documentation Hub (`docs/README.md`) & Refreshed Root `README.md`** — Visual showcase with hero banner, embedded animated GIF demos, mockup gallery, ADR index, and quick-start navigation.
2. **Architecture Decision Records (`docs/decisions/ADR-001` through `ADR-006`)** — Capturing key architectural trade-offs and rejected alternatives per `/documentation-and-adrs`.
3. **Illustrated How-To Demo Videos (`docs/VIDEOS.md` + `docs/assets/videos/*.gif` & `*.webm` + reproducible renderer scripts)** — Visual, illustrated step-by-step walkthrough videos showing **how to use `sshm`** (combining illustrated architecture/workflow panels, numbered step callouts, keystroke badges, and crisp terminal/TUI screens) in both inline-playable GitHub Markdown `.gif` and `.webm` video formats:
   - Demo 1 (`demo-tui-agile-workflow`): Illustrated guide to launching `sshm`, navigating groups, inspecting hosts in Split-Pane Inspector (`i`), fuzzy search (`/`), live TCP probing (`r`), and live theme cycling (`t`)
   - Demo 2 (`demo-add-host-wincred-connect`): Illustrated guide to adding a host (`a`), saving passwords securely by default into Windows Credential Manager (`hosts.toml` stores only ID refs), and connecting instantly (`Enter`)
   - Demo 3 (`demo-tunnels-batch-sftp`): Illustrated guide to configuring interactive Port Forwards (`-L`/`-R`/`-D` via `f`), running parallel `@group` batch commands (`sshm exec @prod --parallel`), and copying files via built-in SFTP (`sshm cp`)
4. **High-Fidelity TUI Mockups (`docs/MOCKUPS.md` + `docs/assets/mockups/*.png`)** — Pixel-accurate rendered mockups faithfully reflecting `internal/tui/styles.go`, `model.go`, `inspector.go`, and `forwards.go`:
   - `tui-minimal-dark.png` (Split-pane main dashboard with live probes & Oktanetflow watermark)
   - `tui-minimal-light.png` (Light theme variant)
   - `tui-mono.png` (Monochrome / high-contrast accessible variant)
   - `tui-add-host-modal.png` (Add/Edit Host form with Save Password ON, Tags & Notes)
   - `tui-port-forwards-modal.png` (Interactive Port Forward Manager `-L`, `-R`, `-D`)
   - `tui-help-group-modal.png` (Group Defaults Inspector & Help overlay)
5. **Custom Illustrations (`docs/assets/illustrations/*.png` & `*.svg`)**:
   - `hero-banner.png` — Oktanetflow `sshm` Windows-first SSH client hero visual
   - `architecture-overview.svg` — Clean vector package & data-flow diagram
   - `security-credential-vault.png` — Three-tier Zero-Plaintext credential architecture (WinCred / DPAPI / Argon2id+AES-256-GCM + NTFS DACL)
   - `network-proxyjump-tunnels.png` — Multi-hop `ProxyJump` bastion chain + `-L`/`-R`/`-D` SOCKS5 tunneling topology
   - `windows-publisher-trust.svg` — Build-to-SmartScreen trust pipeline (PE VersionInfo, `asInvoker`, `-trimpath`, Authenticode/Trusted Signing, WDSI)
6. **Operational & Contribution Guidelines (`docs/GUIDELINES.md`)** — Daily operational hygiene, credential backend selection matrix, team onboarding, Defender/SmartScreen verification rules, ADR authoring lifecycle, and code/doc standards.
7. **Real-World Use Cases (`docs/USE_CASES.md`)** — Six end-to-end playbooks with concrete commands, config snippets, expected outputs, and topology diagrams:
   - UC-1: Solo Windows Developer / Full-Stack Daily Workflow
   - UC-2: Multi-Environment Production Ops via Bastion (`ProxyJump` + Group Inheritance)
   - UC-3: Local Database & Internal Web UI Tunneling (`-L`, `-R`, `-D` SOCKS5)
   - UC-4: Fleet Operations & Artifact Deployment (`sshm exec @prod --parallel` + `sshm cp`)
   - UC-5: Zero-Friction Migration from PuTTY & OpenSSH `~/.ssh/config`
   - UC-6: Encrypted Backup, Portable Vault & Air-Gapped Workstation Recovery (`.sshm.enc`)
8. **AI Agent & Contributor Context (`CLAUDE.md`)** — Concise project rules, build/test commands, security invariants, and ADR pointers for future engineers and AI agents.

---

## 2. Tech Stack & Asset Toolchain

| Layer | Tool / Format | Rationale |
| :--- | :--- | :--- |
| **Docs & ADRs** | GitHub-Flavored Markdown (`.md`) + Mermaid | Native rendering on GitHub, diff-friendly, zero runtime build dependency |
| **TUI Mockups (`.png`)** | Headless Chromium (`msedge.exe` / `chrome.exe`) rendering deterministic HTML/CSS templates using exact xterm-256 colors from `internal/tui/styles.go` | Pixel-sharp monospace typography and exact parity with actual `sshm` Bubble Tea output |
| **Demo Videos (`.gif` + `.webm`)** | Node.js + Headless Chromium / Canvas MediaRecorder (`.webm`) + deterministic frame-to-GIF encoder (`docs/scripts/render-media.mjs`) | Zero external `ffmpeg`/`vhs` install required; 100% reproducible from versioned source scripts |
| **Editorial Illustrations (`.png`)** | High-resolution generated visuals (`docs/assets/illustrations/*.png`) | Rich visual clarity for Hero Banner, Credential Vault, and ProxyJump topologies |
| **Technical Vector Diagrams (`.svg`)** | Hand-crafted clean SVG (`docs/assets/illustrations/*.svg`) | Crisp at any zoom level, tiny file size, Git-diffable |

---

## 3. Commands

```powershell
# 1. Render all high-DPI TUI mockups (.png) and demo videos (.gif + .webm)
node docs/scripts/render-media.mjs

# 2. Verify all documentation links, ADR numbering, and binary media headers
powershell -ExecutionPolicy Bypass -File docs/scripts/verify-docs.ps1

# 3. Ensure Go code & tests remain green and untouched
go vet ./...
go test -race ./...

# 4. Stage, commit, and push to origin/main
git status
git add README.md CLAUDE.md docs/
git commit -m "docs: add ADRs, visual mockups, illustrations, guidelines, use cases, and demo videos"
git push origin main
```

---

## 4. Project Structure

```text
cli-based-ssh-client-windows-support/
├── CLAUDE.md                                      # Project rules & context for AI agents / engineers
├── README.md                                      # Updated with hero banner, GIF preview, visual & ADR links
└── docs/
    ├── README.md                                  # Central Documentation Hub & Navigation Index
    ├── SPEC_DOCUMENTATION.md                      # This specification (living doc)
    ├── ARCHITECTURE.md                            # Existing architecture doc (enhanced with SVG + ADR links)
    ├── USAGE.md                                   # Existing CLI/TUI user manual
    ├── WINDOWS_PUBLISHER_GUIDE.md                 # Existing SmartScreen & Trusted Signing guide
    ├── GUIDELINES.md                              # Operational, Security, Team & Documentation Guidelines
    ├── USE_CASES.md                               # 6 end-to-end real-world sshm playbooks
    ├── MOCKUPS.md                                 # High-DPI TUI Screen & Modal Mockup Gallery
    ├── VIDEOS.md                                  # Interactive Demo Videos Guide (GIF + WebM + scripts)
    ├── decisions/
    │   ├── README.md                              # ADR Index & Lifecycle Guide
    │   ├── ADR-001-pure-go-ssh-engine.md
    │   ├── ADR-002-save-password-by-default-and-credstore.md
    │   ├── ADR-003-windows-first-vt100-ntfs-acl-and-pe-packaging.md
    │   ├── ADR-004-bubble-tea-split-pane-tui-and-async-probing.md
    │   ├── ADR-005-toml-config-group-inheritance-and-atomic-writes.md
    │   └── ADR-006-independent-codebase-and-distribution.md
    ├── scripts/
    │   ├── render-media.mjs                       # Reproducible Node + Headless Chrome PNG/GIF/WebM generator
    │   └── verify-docs.ps1                        # Automated Markdown link, ADR & media asset verifier
    └── assets/
        ├── illustrations/
        │   ├── hero-banner.png
        │   ├── security-credential-vault.png
        │   ├── network-proxyjump-tunnels.png
        │   ├── architecture-overview.svg
        │   └── windows-publisher-trust.svg
        ├── mockups/
        │   ├── tui-minimal-dark.png
        │   ├── tui-minimal-light.png
        │   ├── tui-mono.png
        │   ├── tui-add-host-modal.png
        │   ├── tui-port-forwards-modal.png
        │   └── tui-help-group-modal.png
        └── videos/
            ├── demo-tui-agile-workflow.gif
            ├── demo-tui-agile-workflow.webm
            ├── demo-add-host-wincred-connect.gif
            ├── demo-add-host-wincred-connect.webm
            ├── demo-tunnels-batch-sftp.gif
            └── demo-tunnels-batch-sftp.webm
```

---

## 5. Documentation & ADR Style

Every ADR in `docs/decisions/` follows the `/documentation-and-adrs` structure:

```markdown
# ADR-001: Pure-Go Embedded SSHv2 & SFTP Engine (Zero External Binaries)

## Status
Accepted

## Date
2026-11-26

## Context
[Concrete problem, Windows environment constraints, operational goals]

## Decision
[Clear architectural choice grounded in actual packages]

## Alternatives Considered
### Option A: Shelling out to system OpenSSH (`ssh.exe`)
- Pros: ...
- Cons: ...
- Rejected: ...

## Consequences
- Positive & negative trade-offs, maintenance impact, security posture
```

### Content Conventions
- **Grounded in real code:** Every CLI flag, keybinding, file path, config key, and xterm color must match `internal/cli`, `internal/tui`, `internal/config`, `internal/credstore`, and `internal/sshclient`.
- **Consistent branding:** Include `Copyright © 2026 Oktanetflow` in top-level docs and visual mockups; never alter existing source code comments unrelated to this task.
- **Asset weight hygiene:** Keep GIFs crisp and reasonably sized (≤ 3 MB each) and WebM videos lightweight (≤ 5 MB each) so Git cloning stays fast.

---

## 6. Testing & Verification Strategy

1. **Automated Documentation Verification (`docs/scripts/verify-docs.ps1`):**
   - Verifies every local Markdown link and image `![...](...)` path across `README.md`, `CLAUDE.md`, and `docs/**/*.md` resolves to an existing file.
   - Verifies all 6 ADRs contain required sections (`Status`, `Date`, `Context`, `Decision`, `Alternatives Considered`, `Consequences`).
   - Verifies every `.png` (PNG magic bytes), `.gif` (`GIF89a`), `.webm` (`0x1A45DFA3` EBML header), and `.svg` (valid `<svg` root) asset exists, is non-empty, and respects size budgets.
2. **Visual Quality Inspection:**
   - Visually inspect generated PNG mockups and illustrations via `read_file` to confirm typography, contrast, layout alignment, and zero clipping/artifacts.
3. **Repository Integrity:**
   - Run `go vet ./...` and `go test -race ./...` to confirm zero regressions.
   - Verify untracked temporary artifacts (such as root `coverage`) are NOT staged or pushed.

---

## 7. Boundaries

- **Always do:**
  - Ground every command, keybinding, and config setting in the actual Go codebase.
  - Keep `Copyright © 2026 Oktanetflow` consistent across docs and mockups.
  - Run `docs/scripts/verify-docs.ps1` and `go test -race ./...` before committing and pushing.
- **Ask first:**
  - Before making any changes to Go application source files (`cmd/`, `internal/`).
  - Before installing third-party global system software.
- **Never do:**
  - Never commit secrets, certificates, `.sshm.enc` backups, binaries (`sshm.exe`), or untracked `coverage` files.
  - Never fabricate commands or flags that do not exist in `sshm`.
  - Never recommend disabling Windows Defender or SmartScreen in any guideline or use case.

---

## 8. Success Criteria

- [x] All 6 ADRs (`ADR-001`–`ADR-006`) + `docs/decisions/README.md` exist and follow the ADR template.
- [x] All 6 high-DPI TUI mockup PNGs exist, visually inspected, and cataloged in `docs/MOCKUPS.md`.
- [x] All 5 illustrations (3 PNGs + 2 SVGs) exist, visually inspected, and embedded across `README.md` and `docs/`.
- [x] All 3 demo workflows exist in **both** `.gif` (inline GitHub playback) and `.webm` formats, generated by reproducible `docs/scripts/render-media.mjs`, and documented in `docs/VIDEOS.md`.
- [x] `docs/GUIDELINES.md`, `docs/USE_CASES.md`, `docs/README.md`, and `CLAUDE.md` are complete and cross-linked from `README.md`.
- [x] `docs/scripts/verify-docs.ps1` passes with 0 broken links and 0 invalid media files.
- [x] `go vet ./...` and `go test ./...` pass (`-race` runs in CI with CGo enabled).
- [x] Changes are cleanly committed and pushed to `https://github.com/oktaaa17/sshm` (`main` branch).

---

## 9. Implementation Plan & Ordered Tasks

### Phase A — Architecture Decision Records & Core Written Docs (Parallelizable)
- [x] **Task 1:** Create `docs/decisions/README.md` and `ADR-001` through `ADR-006`
  - *Acceptance:* All 6 ADRs document Context, Decision, Alternatives Considered, Consequences grounded in real packages.
  - *Verify:* `docs/scripts/verify-docs.ps1` ADR section check passes.
- [x] **Task 2:** Write `docs/GUIDELINES.md`, `docs/USE_CASES.md`, and `CLAUDE.md`
  - *Acceptance:* Covers operational/security/team/ADR guidelines, 6 concrete use-case playbooks, and agent rules.
  - *Verify:* All commands cross-checked against `internal/cli/root.go`.

### Phase B — Illustrations (PNG + SVG)
- [x] **Task 3:** Generate 3 PNG illustrations (`hero-banner.png`, `security-credential-vault.png`, `network-proxyjump-tunnels.png`) and craft 2 vector SVGs (`architecture-overview.svg`, `windows-publisher-trust.svg`).
  - *Acceptance:* All 5 files exist in `docs/assets/illustrations/` and pass visual inspection.
  - *Verify:* Visual check via `read_file` + magic-byte check.

### Phase C — Deterministic TUI Mockups (PNG) & Demo Videos (GIF + WebM)
- [x] **Task 4:** Build `docs/scripts/render-media.mjs` (Headless Chrome/Edge renderer + pure-JS GIF89a encoder + WebM MediaRecorder) and generate all 6 mockup PNGs + 3 GIFs + 3 WebM video clips.
  - *Acceptance:* All 6 PNGs, 3 GIFs, and 3 WebMs generated cleanly with exact `styles.go` palette and valid headers.
  - *Verify:* Visual inspection of PNGs + header/size checks in `verify-docs.ps1`.
- [x] **Task 5:** Write `docs/MOCKUPS.md`, `docs/VIDEOS.md`, `docs/README.md`, update `docs/ARCHITECTURE.md` and root `README.md`.
  - *Acceptance:* Full visual gallery, embedded inline GIFs, WebM links, ADR index, and clean cross-navigation.
  - *Verify:* `docs/scripts/verify-docs.ps1` passes with zero errors.

### Phase D — Verification, Commit & Push
- [x] **Task 6:** Run `docs/scripts/verify-docs.ps1`, `go vet ./...`, and `go test ./...`; stage only intended docs/assets (excluding untracked `coverage`), commit, and push to `origin/main`.
  - *Acceptance:* Clean `git push origin main` succeeded; remote `https://github.com/oktaaa17/sshm` updated.
  - *Verify:* `git status` and `git log -n 1 --stat`.
