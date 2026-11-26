package sshclient

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Oktanetflow/sshm/internal/config"
	"github.com/Oktanetflow/sshm/internal/credstore"
	"github.com/Oktanetflow/sshm/internal/termwin"
	"golang.org/x/crypto/ssh"
)

// PromptFunc prompts the user for a secret (password or key passphrase).
type PromptFunc func(prompt string) ([]byte, error)

// ConnectOptions configures how an SSH connection is established.
type ConnectOptions struct {
	Manager         *config.Manager
	CredStore       credstore.Store
	Host            config.ResolvedHost
	PromptSecret    PromptFunc
	In              io.Reader
	Out             io.Writer
	ErrOut          io.Writer
	ExtraForwards   []config.PortForward
	HostKeyCallback ssh.HostKeyCallback // Optional override (useful for tests)
}

// Connection wraps an established SSH client along with any underlying jump clients.
type Connection struct {
	Client   *ssh.Client
	closers  []io.Closer
	cancelKA context.CancelFunc
}

func (c *Connection) Close() error {
	if c.cancelKA != nil {
		c.cancelKA()
	}
	var firstErr error
	if c.Client != nil {
		if err := c.Client.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	for i := len(c.closers) - 1; i >= 0; i-- {
		if err := c.closers[i].Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Connect establishes an SSH connection (handling ProxyJump, key/agent/saved-password auth, and default password saving).
func Connect(ctx context.Context, opts ConnectOptions) (*Connection, error) {
	return connectWithVisited(ctx, opts, map[string]bool{})
}

func connectWithVisited(ctx context.Context, opts ConnectOptions, visited map[string]bool) (*Connection, error) {
	if opts.In == nil {
		opts.In = os.Stdin
	}
	if opts.Out == nil {
		opts.Out = os.Stdout
	}
	if opts.ErrOut == nil {
		opts.ErrOut = os.Stderr
	}

	aliasKey := strings.ToLower(opts.Host.Alias)
	if aliasKey != "" {
		if visited[aliasKey] {
			return nil, fmt.Errorf("jump host cycle detected at %q", opts.Host.Alias)
		}
		visited[aliasKey] = true
	}

	var closers []io.Closer
	var dialFn func(network, addr string) (net.Conn, error)

	timeout := time.Duration(opts.Manager.Config.SSH.ConnectTimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	if opts.Host.EffectiveJumpHost != "" {
		jumpHostRaw, _, ok := opts.Manager.FindHost(opts.Host.EffectiveJumpHost)
		if !ok {
			return nil, fmt.Errorf("jump host %q not found in saved hosts", opts.Host.EffectiveJumpHost)
		}
		jumpOpts := opts
		jumpOpts.Host = opts.Manager.ResolveHost(jumpHostRaw)
		jumpOpts.ExtraForwards = nil
		jumpConn, err := connectWithVisited(ctx, jumpOpts, visited)
		if err != nil {
			return nil, fmt.Errorf("failed connecting to jump host %q: %w", opts.Host.EffectiveJumpHost, err)
		}
		closers = append(closers, jumpConn)
		dialFn = jumpConn.Client.Dial
	} else {
		dialer := &net.Dialer{Timeout: timeout}
		dialFn = func(network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		}
	}

	hkCallback := opts.HostKeyCallback
	if hkCallback == nil {
		var err error
		hkCallback, err = BuildHostKeyCallback(
			opts.Manager.Config.General.KnownHostsPath,
			opts.Manager.Config.General.StrictHostKeyChecking,
			opts.Host.Alias,
			opts.In,
			opts.ErrOut,
		)
		if err != nil {
			closeAll(closers)
			return nil, err
		}
	}

	authMethods, authClosers, usedSavedPW, cleanupSecrets, err := buildAuthMethods(opts)
	if err != nil {
		closeAll(closers)
		return nil, err
	}
	closers = append(closers, authClosers...)
	defer cleanupSecrets()

	addr := net.JoinHostPort(opts.Host.Hostname, strconv.Itoa(opts.Host.EffectivePort))

	// Attempt 1: configured auth methods (key, agent, saved password).
	if len(authMethods) > 0 {
		client, dialErr := dialSSH(dialFn, addr, opts.Host.EffectiveUser, authMethods, hkCallback, timeout)
		if dialErr == nil {
			return finishConnection(ctx, client, closers, opts)
		}
		// If error is not auth-related (e.g., network refused, host key mismatch), return immediately.
		if !isAuthError(dialErr) {
			closeAll(closers)
			return nil, dialErr
		}
		if usedSavedPW && opts.ErrOut != nil {
			fmt.Fprintf(opts.ErrOut, "Saved password authentication failed for %s@%s; prompting for password.\n", opts.Host.EffectiveUser, opts.Host.Hostname)
		}
	}

	// Attempt 2: interactive password prompt fallback (saves on success if SavePassword is true!).
	if opts.PromptSecret == nil {
		closeAll(closers)
		return nil, fmt.Errorf("authentication failed for %s@%s and no interactive prompt available", opts.Host.EffectiveUser, opts.Host.Hostname)
	}

	pw, err := opts.PromptSecret(fmt.Sprintf("%s@%s's password: ", opts.Host.EffectiveUser, opts.Host.Hostname))
	if err != nil {
		closeAll(closers)
		return nil, fmt.Errorf("password prompt cancelled: %w", err)
	}
	defer credstore.ZeroBytes(pw)

	pwStr := string(pw)
	pwMethods := []ssh.AuthMethod{
		ssh.Password(pwStr),
		keyboardInteractivePassword(pwStr),
	}

	client, err := dialSSH(dialFn, addr, opts.Host.EffectiveUser, pwMethods, hkCallback, timeout)
	if err != nil {
		closeAll(closers)
		return nil, err
	}

	// Default save password: save password to secure store upon successful authentication!
	if opts.Host.EffectiveSavePassword && opts.CredStore != nil && opts.Host.CredentialRef != "" {
		if serr := opts.CredStore.Set(opts.Host.CredentialRef, pw); serr != nil && opts.ErrOut != nil {
			fmt.Fprintf(opts.ErrOut, "Warning: connected, but failed to save password: %v\n", serr)
		}
	}

	return finishConnection(ctx, client, closers, opts)
}

func finishConnection(ctx context.Context, client *ssh.Client, closers []io.Closer, opts ConnectOptions) (*Connection, error) {
	kaCtx, cancelKA := context.WithCancel(ctx)
	conn := &Connection{
		Client:   client,
		closers:  closers,
		cancelKA: cancelKA,
	}

	// Start keep-alive loop if configured.
	intervalSec := opts.Manager.Config.SSH.KeepaliveIntervalSec
	maxMissed := opts.Manager.Config.SSH.KeepaliveCountMax
	if intervalSec > 0 {
		go runKeepAlive(kaCtx, client, time.Duration(intervalSec)*time.Second, maxMissed)
	}

	// Activate profile + CLI port forwards.
	allForwards := append([]config.PortForward{}, opts.Host.Forwards...)
	allForwards = append(allForwards, opts.ExtraForwards...)
	if len(allForwards) > 0 {
		if err := StartForwards(kaCtx, client, allForwards, opts.ErrOut); err != nil {
			_ = conn.Close()
			return nil, err
		}
	}

	if opts.Host.Alias != "" {
		_ = opts.Manager.TouchLastConnected(opts.Host.Alias)
	}

	return conn, nil
}

func dialSSH(
	dialFn func(network, addr string) (net.Conn, error),
	addr, user string,
	methods []ssh.AuthMethod,
	hkCallback ssh.HostKeyCallback,
	timeout time.Duration,
) (*ssh.Client, error) {
	rawConn, err := dialFn("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            methods,
		HostKeyCallback: hkCallback,
		Timeout:         timeout,
	}
	c, chans, reqs, err := ssh.NewClientConn(rawConn, addr, cfg)
	if err != nil {
		_ = rawConn.Close()
		return nil, err
	}
	return ssh.NewClient(c, chans, reqs), nil
}

func buildAuthMethods(opts ConnectOptions) ([]ssh.AuthMethod, []io.Closer, bool, func(), error) {
	var methods []ssh.AuthMethod
	var closers []io.Closer
	var secretsToZero [][]byte
	usedSavedPW := false

	cleanup := func() {
		for _, s := range secretsToZero {
			credstore.ZeroBytes(s)
		}
	}

	authPref := strings.ToLower(strings.TrimSpace(opts.Host.Auth))

	// 1. Explicit identity file
	if opts.Host.EffectiveIdentityFile != "" && authPref != "password" {
		signer, err := loadPrivateKey(opts.Host.EffectiveIdentityFile, opts)
		if err != nil {
			if authPref == "key" {
				return nil, closers, false, cleanup, err
			}
			// In auto mode, continue to other methods.
		} else if signer != nil {
			methods = append(methods, ssh.PublicKeys(signer))
		}
	}

	// 2. Windows / POSIX ssh-agent
	if opts.Manager.Config.SSH.UseSSHAgent && authPref != "password" {
		if ag, closer, err := dialSSHAgent(); err == nil {
			closers = append(closers, closer)
			methods = append(methods, ssh.PublicKeysCallback(ag.Signers))
		}
	}

	// 3. Saved password in credential store
	if opts.CredStore != nil && opts.Host.CredentialRef != "" && authPref != "key" && authPref != "agent" {
		pw, err := opts.CredStore.Get(opts.Host.CredentialRef)
		if err == nil && len(pw) > 0 {
			secretsToZero = append(secretsToZero, pw)
			pwStr := string(pw)
			methods = append(methods, ssh.Password(pwStr), keyboardInteractivePassword(pwStr))
			usedSavedPW = true
		}
	}

	return methods, closers, usedSavedPW, cleanup, nil
}

func loadPrivateKey(keyPath string, opts ConnectOptions) (ssh.Signer, error) {
	keyBytes, err := os.ReadFile(filepath.Clean(keyPath)) // #nosec G304 -- SSH private key path configured by user
	if err != nil {
		return nil, fmt.Errorf("failed to read identity file %s: %w", keyPath, err)
	}
	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err == nil {
		return signer, nil
	}

	var passMissing *ssh.PassphraseMissingError
	if !errors.As(err, &passMissing) {
		return nil, fmt.Errorf("failed to parse private key %s: %w", keyPath, err)
	}

	passRef := credstore.KeyPassphraseRef(opts.Host.ID)
	if opts.CredStore != nil && opts.Host.ID != "" {
		if saved, gerr := opts.CredStore.Get(passRef); gerr == nil && len(saved) > 0 {
			defer credstore.ZeroBytes(saved)
			if s, perr := ssh.ParsePrivateKeyWithPassphrase(keyBytes, saved); perr == nil {
				return s, nil
			}
		}
	}

	if opts.PromptSecret == nil {
		return nil, fmt.Errorf("private key %s requires a passphrase", keyPath)
	}
	pass, perr := opts.PromptSecret(fmt.Sprintf("Passphrase for key %s: ", keyPath))
	if perr != nil {
		return nil, perr
	}
	defer credstore.ZeroBytes(pass)

	signer, err = ssh.ParsePrivateKeyWithPassphrase(keyBytes, pass)
	if err != nil {
		return nil, fmt.Errorf("incorrect passphrase for key %s: %w", keyPath, err)
	}
	if opts.Host.EffectiveSavePassword && opts.CredStore != nil && opts.Host.ID != "" {
		_ = opts.CredStore.Set(passRef, pass)
	}
	return signer, nil
}

func keyboardInteractivePassword(password string) ssh.AuthMethod {
	return ssh.KeyboardInteractive(func(user, instruction string, questions []string, echos []bool) ([]string, error) {
		answers := make([]string, len(questions))
		for i := range questions {
			answers[i] = password
		}
		return answers, nil
	})
}

func isAuthError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "unable to authenticate") ||
		strings.Contains(msg, "no supported methods remain") ||
		strings.Contains(msg, "permission denied")
}

func runKeepAlive(ctx context.Context, client *ssh.Client, interval time.Duration, maxMissed int) {
	if maxMissed <= 0 {
		maxMissed = 3
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	missed := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _, err := client.SendRequest("keepalive@openssh.com", true, nil)
			if err != nil {
				missed++
				if missed >= maxMissed {
					_ = client.Close()
					return
				}
			} else {
				missed = 0
			}
		}
	}
}

func closeAll(closers []io.Closer) {
	for i := len(closers) - 1; i >= 0; i-- {
		_ = closers[i].Close()
	}
}

// StartShell opens a full interactive PTY shell with Windows VT100 mode and live resize tracking.
func StartShell(ctx context.Context, client *ssh.Client) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	state, err := termwin.EnterRawVT()
	if err != nil {
		return fmt.Errorf("failed to initialize terminal mode: %w", err)
	}
	defer state.Restore()

	w, h := termwin.GetSize()
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	termType := os.Getenv("TERM")
	if termType == "" || termType == "dumb" {
		termType = "xterm-256color"
	}

	if err := session.RequestPty(termType, h, w, modes); err != nil {
		return fmt.Errorf("failed to request PTY: %w", err)
	}

	session.Stdin = os.Stdin
	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start remote shell: %w", err)
	}

	resizeCtx, cancelResize := context.WithCancel(ctx)
	defer cancelResize()
	go termwin.WatchResize(resizeCtx, func(newW, newH int) {
		_ = session.WindowChange(newH, newW)
	})

	err = session.Wait()
	if err != nil {
		var exitErr *ssh.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitStatus() == 0 {
			return nil
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
	}
	return err
}

// Exec runs a single command on the remote host and writes output to stdout/stderr.
func Exec(ctx context.Context, client *ssh.Client, command string, stdout, stderr io.Writer) error {
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	session.Stdout = stdout
	session.Stderr = stderr

	done := make(chan error, 1)
	go func() {
		done <- session.Run(command)
	}()

	select {
	case <-ctx.Done():
		_ = session.Signal(ssh.SIGTERM)
		_ = session.Close()
		return ctx.Err()
	case err := <-done:
		return err
	}
}
