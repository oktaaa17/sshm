# ADR-001: Pure-Go Embedded SSHv2 & SFTP Engine (Zero External Binaries)

## Status
Accepted

## Date
2026-11-26

## Context
Windows workstations frequently lack a consistent external SSH toolchain:
- Older Windows 10 machines or locked-down enterprise laptops may not have the Windows OpenSSH Client optional feature enabled.
- Password-based automation tools like `sshpass` do not exist natively on Windows without Cygwin, Git Bash, or WSL.
- Shelling out to external `ssh.exe` or `plink.exe` breaks portable programmatic features such as native Windows Credential Manager password injection, cycle-checked multi-hop `ProxyJump`, RFC 1928 SOCKS5 dynamic forwarding (`-D`), SFTP file transfers (`sshm cp`), and parallel batch execution (`sshm exec @group --parallel`).
- Requiring CGo complicates reproducible cross-platform builds and introduces runtime DLL dependencies.

## Decision
Build `sshm` as a **100% pure-Go (`CGO_ENABLED=0`), single-binary SSHv2 & SFTP client** using:
- `golang.org/x/crypto/ssh` and `golang.org/x/crypto/ssh/knownhosts` for native SSHv2 transport, authentication, `ProxyJump` tunneling, and TOFU host key verification (`internal/sshclient/`).
- `github.com/pkg/sftp` for built-in SFTP transfers (`sshm cp`).
- `github.com/Microsoft/go-winio` for native Windows OpenSSH agent named pipe support (`\\.\pipe\openssh-ssh-agent`).

## Alternatives Considered

### Wrapping system `ssh.exe` (OpenSSH subprocess wrapper)
- **Pros:** Inherits every OpenSSH `ssh_config` directive automatically; smaller Go codebase.
- **Cons:** Fails when OpenSSH is absent or outdated on Windows; cannot securely inject credentials from Windows Credential Manager without fragile PTY scraping; cannot provide unified parallel `@group` batch execution or integrated SFTP without spawning multiple external processes (`ssh.exe`, `scp.exe`, `sftp.exe`).
- **Rejected:** Violates the core goal of a zero-dependency, self-contained Windows SSH client.

### Binding `libssh2` via CGo
- **Pros:** Mature C SSH library.
- **Cons:** Requires MinGW/MSVC toolchains on Windows; breaks `CGO_ENABLED=0` reproducible builds; introduces memory-safety risks (buffer overflows, C pointer leaks) and harder security auditing (`govulncheck` cannot scan C dependencies).
- **Rejected:** Pure Go delivers memory safety, static binaries, and clean `govulncheck` + `gosec` gates.

### Bundling PuTTY / `plink.exe` alongside a TUI
- **Pros:** Familiar on legacy Windows environments.
- **Cons:** Multi-binary distribution; inconsistent ANSI/VT100 terminal rendering; poor PowerShell/Windows Terminal UX; requires maintaining third-party binary updates.
- **Rejected:** Poor ergonomics and higher maintenance risk compared to a single native Go executable.

## Consequences
- **Positive:** Single ~10 MB static `sshm.exe` runs anywhere on Windows 10/11 (`amd64`/`arm64`) as well as Linux/macOS with zero runtime prerequisites.
- **Positive:** Full programmatic control over authentication fallback (Windows OpenSSH agent, private keys, saved passwords), host key mismatch protection, keepalives, `-L`/`-R`/`-D` port forwards, and parallel group execution.
- **Negative:** Exotic legacy OpenSSH directives (e.g., GSSAPI/Kerberos, `ProxyCommand` shell scripts, PKCS#11 smartcards) are not automatically inherited and must be explicitly implemented if needed in future releases.
- **Mitigation:** Provide `sshm import --ssh-config` and `sshm import --putty` so users can migrate existing inventories seamlessly while keeping `~/.ssh/known_hosts` compatible.
