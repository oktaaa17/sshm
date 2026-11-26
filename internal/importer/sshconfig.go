package importer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/oktaaa17/sshm/internal/config"
)

// ImportSSHConfig parses an OpenSSH client config file (~/.ssh/config) into sshm Host records.
func ImportSSHConfig(path, defaultGroup string) ([]config.Host, error) {
	p := filepath.Clean(config.ExpandPath(path))
	f, err := os.Open(p) // #nosec G304 -- OpenSSH config path specified by user
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", p, err)
	}
	defer f.Close()

	if defaultGroup == "" {
		defaultGroup = config.DefaultGroupName
	}

	var hosts []config.Host
	var currentAliases []string
	current := map[string]string{}

	flush := func() {
		if len(currentAliases) == 0 {
			return
		}
		for _, alias := range currentAliases {
			if alias == "" || strings.ContainsAny(alias, "*?!") {
				continue
			}
			hostname := current["hostname"]
			if hostname == "" {
				hostname = alias
			}
			port := 0
			if pv := current["port"]; pv != "" {
				if n, err := strconv.Atoi(pv); err == nil && n > 0 {
					port = n
				}
			}
			auth := "auto"
			if current["identityfile"] != "" {
				auth = "key"
			}
			hosts = append(hosts, config.Host{
				Alias:        alias,
				Group:        defaultGroup,
				Hostname:     hostname,
				Port:         port,
				User:         current["user"],
				Auth:         auth,
				IdentityFile: current["identityfile"],
				JumpHost:     current["proxyjump"],
			})
		}
		currentAliases = nil
		current = map[string]string{}
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Strip inline comments outside quotes (simple heuristic).
		if idx := strings.Index(line, " #"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		key, val := splitSSHConfigDirective(line)
		if key == "" {
			continue
		}
		switch strings.ToLower(key) {
		case "host":
			flush()
			currentAliases = strings.Fields(val)
		case "hostname", "user", "port", "identityfile", "proxyjump":
			k := strings.ToLower(key)
			if _, exists := current[k]; !exists {
				current[k] = strings.Trim(val, `"'`)
			}
		}
	}
	flush()

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return hosts, nil
}

func splitSSHConfigDirective(line string) (string, string) {
	if idx := strings.IndexAny(line, " \t="); idx > 0 {
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(strings.TrimLeft(line[idx:], " \t="))
		return key, val
	}
	return "", ""
}
