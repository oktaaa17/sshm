package sshclient

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/Oktanetflow/sshm/internal/config"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// BuildHostKeyCallback constructs an OpenSSH-compatible known_hosts verifier with TOFU support.
func BuildHostKeyCallback(knownHostsPath, mode, alias string, in io.Reader, out io.Writer) (ssh.HostKeyCallback, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "no" {
		return ssh.InsecureIgnoreHostKey(), nil // #nosec G106 -- explicit user opt-in via strict_host_key_checking=no (default is "ask")
	}

	path := config.ExpandPath(knownHostsPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("failed to create known_hosts directory: %w", err)
	}
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
			return nil, fmt.Errorf("failed to initialize known_hosts file: %w", err)
		}
	}

	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load known_hosts (%s): %w", path, err)
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := cb(hostname, remote, key)
		if err == nil {
			return nil
		}

		var keyErr *knownhosts.KeyError
		if !errors.As(err, &keyErr) {
			return err
		}

		fp := ssh.FingerprintSHA256(key)

		// Host key CHANGED -> potential MITM attack!
		if len(keyErr.Want) > 0 {
			hint := alias
			if hint == "" {
				hint = hostname
			}
			return fmt.Errorf(
				"WARNING: REMOTE HOST IDENTIFICATION HAS CHANGED for %s!\n"+
					"  Presented %s key fingerprint: %s\n"+
					"  This could indicate a man-in-the-middle attack or a reinstalled server.\n"+
					"  To remove the old key, run: sshm hostkey remove %s",
				hostname, key.Type(), fp, hint,
			)
		}

		// Host not found in known_hosts -> strict rejects, ask prompts TOFU.
		if mode == "yes" {
			return fmt.Errorf("host key for %s (%s) not found in %s (strict_host_key_checking=yes)", hostname, fp, path)
		}

		if in == nil || out == nil {
			return fmt.Errorf("unknown host key for %s (%s) and non-interactive session", hostname, fp)
		}

		fmt.Fprintf(out, "The authenticity of host '%s' (%s) can't be established.\n", hostname, remote.String())
		fmt.Fprintf(out, "%s key fingerprint is %s.\n", key.Type(), fp)
		fmt.Fprintf(out, "Are you sure you want to continue connecting (yes/no)? ")

		reader := bufio.NewReader(in)
		line, rerr := reader.ReadString('\n')
		if rerr != nil && !errors.Is(rerr, io.EOF) {
			return fmt.Errorf("failed reading host key confirmation: %w", rerr)
		}
		ans := strings.ToLower(strings.TrimSpace(line))
		if ans != "yes" && ans != "y" {
			return errors.New("host key verification rejected by user")
		}

		if err := appendKnownHost(path, hostname, key); err != nil {
			return fmt.Errorf("failed to persist host key: %w", err)
		}
		fmt.Fprintf(out, "Permanently added '%s' (%s) to %s.\n", hostname, key.Type(), path)
		return nil
	}, nil
}

func appendKnownHost(path, hostname string, key ssh.PublicKey) error {
	norm := knownhosts.Normalize(hostname)
	line := knownhosts.Line([]string{norm}, key) + "\n"
	f, err := os.OpenFile(filepath.Clean(path), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600) // #nosec G304 -- user-configured known_hosts path
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.WriteString(line)
	return err
}

// RemoveHostKey removes entries matching hostname (with or without port) from known_hosts.
func RemoveHostKey(knownHostsPath, hostname string, port int) (int, error) {
	path := filepath.Clean(config.ExpandPath(knownHostsPath))
	raw, err := os.ReadFile(path) // #nosec G304 -- user-configured known_hosts path
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 0, err
	}

	targets := map[string]struct{}{
		strings.ToLower(hostname): {},
	}
	if port > 0 && port != 22 {
		targets[strings.ToLower(fmt.Sprintf("[%s]:%d", hostname, port))] = struct{}{}
	} else {
		targets[strings.ToLower(fmt.Sprintf("[%s]:22", hostname))] = struct{}{}
	}

	lines := strings.Split(string(raw), "\n")
	kept := make([]string, 0, len(lines))
	removed := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			kept = append(kept, line)
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) < 3 {
			kept = append(kept, line)
			continue
		}
		hostList := strings.Split(fields[0], ",")
		matched := false
		for _, h := range hostList {
			if _, ok := targets[strings.ToLower(strings.TrimSpace(h))]; ok {
				matched = true
				break
			}
		}
		if matched {
			removed++
			continue
		}
		kept = append(kept, line)
	}

	if removed > 0 {
		out := strings.Join(kept, "\n")
		if err := config.WriteFileAtomic(path, []byte(out), 0o600); err != nil {
			return 0, err
		}
	}
	return removed, nil
}
