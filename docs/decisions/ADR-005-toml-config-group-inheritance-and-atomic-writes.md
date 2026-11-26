# ADR-005: TOML Configuration, Group Default Inheritance & Mutex-Safe Atomic Persistence

## Status
Accepted

## Date
2026-11-26

## Context
Managing 20–200 servers across `prod`, `staging`, and `personal` environments creates heavy repetition (same bastion jump host, same SSH key, same non-standard port, same deploy username). Meanwhile:
- OpenSSH's `~/.ssh/config` lacks first-class structured metadata for collapsible groups, group colors, host pins, tags, notes, `last_connected` timestamps, and opaque `credential_ref` pointers.
- Parallel batch execution (`sshm exec @prod "uptime" --parallel`) updates `LastConnected` concurrently across goroutines — naive read/write file updates cause race conditions and truncated config files on Windows.
- Overwriting a file directly in place (`os.WriteFile`) risks total config loss if a power outage or crash occurs mid-write.

## Decision
1. **Two-File TOML Schema (`%APPDATA%\sshm\`):**
   - `config.toml`: Global behavioral, SSH keepalive/timeout, security, and UI preferences (`AppConfig`), strictly validated on `sshm config set`.
   - `hosts.toml`: Structured `[groups.*]` definitions and `[[hosts]]` records (`HostsFile`), never storing plaintext passwords.
2. **Group Default Inheritance (`Manager.ResolveHost`):** Each group supports `color`, `default_user`, `default_port`, `default_identity_file`, and `default_jump`. Individual hosts inherit group defaults unless explicitly overridden, and `ResolvedHost` surfaces effective values consistently across CLI, TUI list, and Split-Pane Inspector.
3. **Mutex-Safe Atomic File Writes (`internal/config/config.go`):** Protect `config.Manager` state mutations with `sync.Mutex` and persist files via unique temporary files (`os.CreateTemp(dir, ".sshm-tmp-*")`), `Close()`, permission hardening, and Windows-safe atomic replacement (`replaceFile`). Apply the same atomic pattern to `DPAPIStore`, `VaultStore`, and `knownhosts.RemoveHostKey`.

## Alternatives Considered

### Reading/writing `~/.ssh/config` directly as the primary datastore
- **Pros:** Direct interoperability with external OpenSSH tools.
- **Cons:** OpenSSH config grammar has no native schema for group colors, tags, notes, pinned state, `last_connected`, or credential store references; custom comments (`# sshm:...`) are brittle and easily broken by hand edits or other tools.
- **Rejected:** Keep `~/.ssh/config` as a first-class **import source** (`sshm import --ssh-config`) while maintaining clean structured TOML for `sshm`.

### SQLite embedded database
- **Pros:** Transactional ACID writes out of the box.
- **Cons:** Official SQLite bindings require CGo (violating `CGO_ENABLED=0`), and binary `.db` files cannot be inspected, diffed, or hand-edited in a text editor by sysadmins.
- **Rejected:** Human-readable TOML with mutex-guarded atomic writes provides durability, zero CGo, and instant inspectability.

### JSON or YAML config files
- **Pros:** Widely supported.
- **Cons:** JSON disallows comments and has noisy syntax; YAML has whitespace sensitivity and type-coercion pitfalls (`port: 22`, `yes`/`no`, unquoted IP octets).
- **Rejected:** TOML (`github.com/pelletier/go-toml/v2`) is unambiguous, clean, and ideal for host/group tables.

## Consequences
- **Positive:** Zero data races under `go test -race ./...` even during high-concurrency `sshm exec @group --parallel` runs.
- **Positive:** Editing a group's `default_jump` or `default_key` immediately propagates to every host inheriting from that group.
- **Negative:** Users editing `hosts.toml` manually while `sshm` TUI is open must restart `sshm` to reload external edits.
