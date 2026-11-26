package sshclient

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/oktaaa17/sshm/internal/config"
	"golang.org/x/crypto/ssh"
)

// splitForwardSpec splits a forward spec by ':' while keeping bracketed IPv6 literals intact.
func splitForwardSpec(spec string) ([]string, error) {
	var parts []string
	var cur strings.Builder
	inBracket := false
	for _, r := range spec {
		switch r {
		case '[':
			if inBracket {
				return nil, fmt.Errorf("nested '[' in forward spec %q", spec)
			}
			inBracket = true
		case ']':
			if !inBracket {
				return nil, fmt.Errorf("unmatched ']' in forward spec %q", spec)
			}
			inBracket = false
		case ':':
			if !inBracket {
				parts = append(parts, cur.String())
				cur.Reset()
				continue
			}
			cur.WriteRune(r)
		default:
			cur.WriteRune(r)
		}
	}
	if inBracket {
		return nil, fmt.Errorf("unclosed '[' in forward spec %q", spec)
	}
	parts = append(parts, cur.String())
	return parts, nil
}

func validatePortStr(s string) error {
	p, err := strconv.Atoi(s)
	if err != nil || p <= 0 || p > 65535 {
		return fmt.Errorf("invalid port %q (must be 1..65535)", s)
	}
	return nil
}

// ParseLocalOrRemoteSpec parses "[bind_addr:]port:target_host:target_port" (IPv4 and bracketed IPv6).
func ParseLocalOrRemoteSpec(specType, spec string) (config.PortForward, error) {
	parts, err := splitForwardSpec(strings.TrimSpace(spec))
	if err != nil {
		return config.PortForward{}, err
	}
	switch len(parts) {
	case 3:
		if err := validatePortStr(parts[0]); err != nil {
			return config.PortForward{}, fmt.Errorf("invalid %s forward spec %q: %w", specType, spec, err)
		}
		if err := validatePortStr(parts[2]); err != nil {
			return config.PortForward{}, fmt.Errorf("invalid %s forward spec %q: %w", specType, spec, err)
		}
		if parts[1] == "" {
			return config.PortForward{}, fmt.Errorf("invalid %s forward spec %q: empty target host", specType, spec)
		}
		return config.PortForward{
			Type:   specType,
			Bind:   "127.0.0.1:" + parts[0],
			Target: net.JoinHostPort(parts[1], parts[2]),
		}, nil
	case 4:
		bindHost := parts[0]
		if bindHost == "" {
			bindHost = "127.0.0.1"
		}
		if err := validatePortStr(parts[1]); err != nil {
			return config.PortForward{}, fmt.Errorf("invalid %s forward spec %q: %w", specType, spec, err)
		}
		if err := validatePortStr(parts[3]); err != nil {
			return config.PortForward{}, fmt.Errorf("invalid %s forward spec %q: %w", specType, spec, err)
		}
		if parts[2] == "" {
			return config.PortForward{}, fmt.Errorf("invalid %s forward spec %q: empty target host", specType, spec)
		}
		return config.PortForward{
			Type:   specType,
			Bind:   net.JoinHostPort(bindHost, parts[1]),
			Target: net.JoinHostPort(parts[2], parts[3]),
		}, nil
	default:
		return config.PortForward{}, fmt.Errorf("invalid %s forward spec %q (expected [bind:]port:host:port)", specType, spec)
	}
}

// ParseDynamicSpec parses "[bind_addr:]port" for SOCKS5 dynamic forwarding (IPv4 and bracketed IPv6).
func ParseDynamicSpec(spec string) (config.PortForward, error) {
	spec = strings.TrimSpace(spec)
	if h, p, err := net.SplitHostPort(spec); err == nil {
		if err := validatePortStr(p); err != nil {
			return config.PortForward{}, fmt.Errorf("invalid dynamic forward spec %q: %w", spec, err)
		}
		if h == "" {
			h = "127.0.0.1"
		}
		return config.PortForward{Type: "dynamic", Bind: net.JoinHostPort(h, p)}, nil
	}
	if err := validatePortStr(spec); err != nil {
		return config.PortForward{}, fmt.Errorf("invalid dynamic forward spec %q (expected [bind:]port)", spec)
	}
	return config.PortForward{Type: "dynamic", Bind: "127.0.0.1:" + spec}, nil
}

// StartForwards activates all configured local, remote, and dynamic tunnels until ctx is canceled.
func StartForwards(ctx context.Context, client *ssh.Client, forwards []config.PortForward, logOut io.Writer) error {
	for _, f := range forwards {
		switch strings.ToLower(f.Type) {
		case "local":
			if err := startLocalForward(ctx, client, f, logOut); err != nil {
				return err
			}
		case "remote":
			if err := startRemoteForward(ctx, client, f, logOut); err != nil {
				return err
			}
		case "dynamic":
			if err := startDynamicSOCKS5(ctx, client, f, logOut); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown forward type %q", f.Type)
		}
	}
	return nil
}

func startLocalForward(ctx context.Context, client *ssh.Client, f config.PortForward, logOut io.Writer) error {
	ln, err := net.Listen("tcp", f.Bind)
	if err != nil {
		return fmt.Errorf("local forward listen %s failed: %w", f.Bind, err)
	}
	if logOut != nil {
		fmt.Fprintf(logOut, "[forward] local %s -> %s\n", ln.Addr().String(), f.Target)
	}
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	go func() {
		for {
			localConn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				remoteConn, err := client.Dial("tcp", f.Target)
				if err != nil {
					return
				}
				defer remoteConn.Close()
				proxyBidirectional(c, remoteConn)
			}(localConn)
		}
	}()
	return nil
}

func startRemoteForward(ctx context.Context, client *ssh.Client, f config.PortForward, logOut io.Writer) error {
	ln, err := client.Listen("tcp", f.Bind)
	if err != nil {
		return fmt.Errorf("remote forward listen %s failed: %w", f.Bind, err)
	}
	if logOut != nil {
		fmt.Fprintf(logOut, "[forward] remote %s -> %s\n", f.Bind, f.Target)
	}
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	go func() {
		for {
			remoteConn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				localConn, err := net.Dial("tcp", f.Target)
				if err != nil {
					return
				}
				defer localConn.Close()
				proxyBidirectional(c, localConn)
			}(remoteConn)
		}
	}()
	return nil
}

// startDynamicSOCKS5 runs a lightweight RFC 1928 SOCKS5 proxy over the SSH tunnel.
func startDynamicSOCKS5(ctx context.Context, client *ssh.Client, f config.PortForward, logOut io.Writer) error {
	ln, err := net.Listen("tcp", f.Bind)
	if err != nil {
		return fmt.Errorf("dynamic SOCKS5 listen %s failed: %w", f.Bind, err)
	}
	if logOut != nil {
		fmt.Fprintf(logOut, "[forward] dynamic SOCKS5 on %s\n", ln.Addr().String())
		if bindHost, _, err := net.SplitHostPort(f.Bind); err == nil {
			ip := net.ParseIP(bindHost)
			if bindHost == "" || (ip != nil && !ip.IsLoopback()) || (ip == nil && !strings.EqualFold(bindHost, "localhost")) {
				fmt.Fprintf(logOut, "[forward] WARNING: unauthenticated SOCKS5 proxy is listening on non-loopback address %s\n", f.Bind)
			}
		}
	}
	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleSOCKS5Conn(conn, client)
		}
	}()
	return nil
}

func handleSOCKS5Conn(conn net.Conn, client *ssh.Client) {
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	hdr := make([]byte, 2)
	if _, err := io.ReadFull(conn, hdr); err != nil || hdr[0] != 0x05 {
		return
	}
	methods := make([]byte, int(hdr[1]))
	if _, err := io.ReadFull(conn, methods); err != nil {
		return
	}
	// No authentication required (0x00)
	if _, err := conn.Write([]byte{0x05, 0x00}); err != nil {
		return
	}

	req := make([]byte, 4)
	if _, err := io.ReadFull(conn, req); err != nil || req[0] != 0x05 || req[1] != 0x01 {
		_, _ = conn.Write([]byte{0x05, 0x07, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	var host string
	switch req[3] {
	case 0x01: // IPv4
		addr := make([]byte, 4)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return
		}
		host = net.IP(addr).String()
	case 0x03: // Domain
		l := make([]byte, 1)
		if _, err := io.ReadFull(conn, l); err != nil {
			return
		}
		dom := make([]byte, int(l[0]))
		if _, err := io.ReadFull(conn, dom); err != nil {
			return
		}
		host = string(dom)
	case 0x04: // IPv6
		addr := make([]byte, 16)
		if _, err := io.ReadFull(conn, addr); err != nil {
			return
		}
		host = net.IP(addr).String()
	default:
		_, _ = conn.Write([]byte{0x05, 0x08, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}

	portBytes := make([]byte, 2)
	if _, err := io.ReadFull(conn, portBytes); err != nil {
		return
	}
	port := binary.BigEndian.Uint16(portBytes)
	target := net.JoinHostPort(host, strconv.Itoa(int(port)))

	remoteConn, err := client.Dial("tcp", target)
	if err != nil {
		_, _ = conn.Write([]byte{0x05, 0x05, 0x00, 0x01, 0, 0, 0, 0, 0, 0})
		return
	}
	defer remoteConn.Close()

	// Reply success and clear handshake deadline before streaming data.
	if _, err := conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0, 0, 0, 0, 0, 0}); err != nil {
		return
	}
	_ = conn.SetDeadline(time.Time{})
	proxyBidirectional(conn, remoteConn)
}

func proxyBidirectional(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(b, a)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(a, b)
		done <- struct{}{}
	}()
	<-done
}
