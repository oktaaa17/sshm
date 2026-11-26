# Contributing to sshm (Oktanetflow)

> **Copyright © 2026 Oktanetflow**

We welcome bug fixes, documentation improvements, and focused features following our **Spec-Driven Development** workflow.

## Development Workflow

1. Review [SPEC.md](SPEC.md) before proposing architectural changes.
2. Create a feature branch from `main` (e.g. `feat/tui-filter`, `fix/ipv6-forward`).
3. Write unit/integration tests alongside your changes (`go test ./...`).
4. Run the full security & quality gate before opening a PR:
   ```powershell
   powershell -ExecutionPolicy Bypass -File scripts/security-scan.ps1
   ```
5. Ensure `gofmt` formatting is clean and no secrets or compiled binaries are staged.

## Code Boundaries

- **Pure Go (`CGO_ENABLED=0`)**: Do not introduce CGo dependencies.
- **No UPX / Packers**: Never add executable packers to build scripts.
- **No Plaintext Secrets**: Credentials must always flow through `internal/credstore.Store`.
- **Windows First**: Verify changes handle Windows paths, VT100 console modes, and NUL modifier runes cleanly.
