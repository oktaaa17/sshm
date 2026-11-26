//go:build windows

package importer

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/oktaaa17/sshm/internal/config"
	"golang.org/x/sys/windows/registry"
)

const puttySessionsKey = `Software\SimonTatham\PuTTY\Sessions`

// ImportPuTTYSessions imports saved PuTTY SSH sessions from the Windows Registry (HKCU).
func ImportPuTTYSessions(defaultGroup string) ([]config.Host, error) {
	if defaultGroup == "" {
		defaultGroup = "putty"
	}

	root, err := registry.OpenKey(registry.CURRENT_USER, puttySessionsKey, registry.ENUMERATE_SUB_KEYS)
	if err != nil {
		return nil, fmt.Errorf("could not open PuTTY sessions registry key: %w", err)
	}
	defer root.Close()

	names, err := root.ReadSubKeyNames(-1)
	if err != nil {
		return nil, fmt.Errorf("failed to list PuTTY sessions: %w", err)
	}

	var hosts []config.Host
	for _, rawName := range names {
		decoded, err := url.QueryUnescape(rawName)
		if err != nil {
			decoded = rawName
		}
		if strings.EqualFold(decoded, "Default Settings") {
			continue
		}

		sub, err := registry.OpenKey(registry.CURRENT_USER, puttySessionsKey+`\`+rawName, registry.QUERY_VALUE)
		if err != nil {
			continue
		}

		hostName, _, _ := sub.GetStringValue("HostName")
		userName, _, _ := sub.GetStringValue("UserName")
		portVal, _, errPort := sub.GetIntegerValue("PortNumber")
		keyFile, _, _ := sub.GetStringValue("PublicKeyFile")
		proto, _, _ := sub.GetStringValue("Protocol")
		_ = sub.Close()

		if hostName == "" {
			continue
		}
		if proto != "" && !strings.EqualFold(proto, "ssh") {
			continue
		}

		// Handle PuTTY HostName formatted as user@host
		if strings.Contains(hostName, "@") {
			parts := strings.SplitN(hostName, "@", 2)
			if userName == "" {
				userName = parts[0]
			}
			hostName = parts[1]
		}

		port := 22
		if errPort == nil && portVal > 0 && portVal <= 65535 {
			port = int(portVal)
		}

		alias := strings.ReplaceAll(strings.TrimSpace(decoded), " ", "-")
		auth := "password"
		if keyFile != "" {
			auth = "key"
		}

		hosts = append(hosts, config.Host{
			Alias:        alias,
			Group:        defaultGroup,
			Hostname:     hostName,
			Port:         port,
			User:         userName,
			Auth:         auth,
			IdentityFile: keyFile,
		})
	}
	return hosts, nil
}
