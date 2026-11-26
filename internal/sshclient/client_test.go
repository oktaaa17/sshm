package sshclient

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Oktanetflow/sshm/internal/config"
	"github.com/Oktanetflow/sshm/internal/credstore"
	"golang.org/x/crypto/ssh"
)

type mockSSHServer struct {
	addr     string
	host     string
	port     int
	listener net.Listener
	hostKey  ssh.Signer
}

func startMockSSHServer(t *testing.T, expectedUser, expectedPass string) *mockSSHServer {
	t.Helper()

	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("new signer: %v", err)
	}

	serverCfg := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == expectedUser && string(pass) == expectedPass {
				return &ssh.Permissions{}, nil
			}
			return nil, fmt.Errorf("rejected auth for %s", c.User())
		},
	}
	serverCfg.AddHostKey(signer)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	host, portStr, _ := net.SplitHostPort(ln.Addr().String())
	port, _ := strconv.Atoi(portStr)

	go func() {
		for {
			nConn, err := ln.Accept()
			if err != nil {
				return
			}
			go handleMockSSHConn(nConn, serverCfg)
		}
	}()

	t.Cleanup(func() { _ = ln.Close() })
	return &mockSSHServer{
		addr:     ln.Addr().String(),
		host:     host,
		port:     port,
		listener: ln,
		hostKey:  signer,
	}
}

func handleMockSSHConn(nConn net.Conn, cfg *ssh.ServerConfig) {
	defer nConn.Close()
	_, chans, reqs, err := ssh.NewServerConn(nConn, cfg)
	if err != nil {
		return
	}
	go ssh.DiscardRequests(reqs)

	for newChan := range chans {
		switch newChan.ChannelType() {
		case "session":
			ch, requests, err := newChan.Accept()
			if err != nil {
				continue
			}
			go func(c ssh.Channel, in <-chan *ssh.Request) {
				defer c.Close()
				for req := range in {
					switch req.Type {
					case "exec":
						if len(req.Payload) >= 4 {
							cmdLen := binary.BigEndian.Uint32(req.Payload[:4])
							cmd := string(req.Payload[4 : 4+cmdLen])
							_ = req.Reply(true, nil)
							_, _ = fmt.Fprintf(c, "echo:%s\n", cmd)
							_, _ = c.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
							return
						}
					default:
						if req.WantReply {
							_ = req.Reply(true, nil)
						}
					}
				}
			}(ch, requests)

		case "direct-tcpip":
			// Support ProxyJump & local port forwarding in mock server!
			var payload struct {
				DestAddr   string
				DestPort   uint32
				OriginAddr string
				OriginPort uint32
			}
			if err := ssh.Unmarshal(newChan.ExtraData(), &payload); err != nil {
				_ = newChan.Reject(ssh.Prohibited, "bad payload")
				continue
			}
			target := net.JoinHostPort(payload.DestAddr, strconv.Itoa(int(payload.DestPort)))
			remote, err := net.Dial("tcp", target)
			if err != nil {
				_ = newChan.Reject(ssh.ConnectionFailed, err.Error())
				continue
			}
			ch, reqs, err := newChan.Accept()
			if err != nil {
				_ = remote.Close()
				continue
			}
			go ssh.DiscardRequests(reqs)
			go func() {
				defer ch.Close()
				defer remote.Close()
				done := make(chan struct{}, 2)
				go func() { _, _ = io.Copy(remote, ch); done <- struct{}{} }()
				go func() { _, _ = io.Copy(ch, remote); done <- struct{}{} }()
				<-done
			}()

		default:
			_ = newChan.Reject(ssh.UnknownChannelType, "unsupported")
		}
	}
}

func TestConnectDefaultSavePasswordAndExec(t *testing.T) {
	srv := startMockSSHServer(t, "alice", "hunter2")

	dir := t.TempDir()
	mgr, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	mgr.Config.General.KnownHostsPath = filepath.Join(dir, "known_hosts")
	mgr.Config.General.StrictHostKeyChecking = "ask"

	store := credstore.NewVaultStore(dir, func() ([]byte, error) { return []byte("test-vault"), nil })

	saved, err := mgr.UpsertHost(config.Host{
		Alias:    "mock-box",
		Group:    "dev",
		Hostname: srv.host,
		Port:     srv.port,
		User:     "alice",
		Auth:     "password",
	})
	if err != nil {
		t.Fatalf("UpsertHost: %v", err)
	}

	// First connect: no saved password yet -> prompt supplies password -> TOFU accepts host key -> password saved automatically!
	promptCalls := 0
	var errOut bytes.Buffer
	conn1, err := Connect(context.Background(), ConnectOptions{
		Manager:   mgr,
		CredStore: store,
		Host:      mgr.ResolveHost(saved),
		In:        strings.NewReader("yes\n"),
		ErrOut:    &errOut,
		PromptSecret: func(prompt string) ([]byte, error) {
			promptCalls++
			return []byte("hunter2"), nil
		},
	})
	if err != nil {
		t.Fatalf("first Connect failed: %v", err)
	}

	var stdout, stderr bytes.Buffer
	if err := Exec(context.Background(), conn1.Client, "hello-world", &stdout, &stderr); err != nil {
		t.Fatalf("Exec failed: %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "echo:hello-world" {
		t.Fatalf("unexpected Exec output: %q", stdout.String())
	}
	_ = conn1.Close()

	if promptCalls != 1 {
		t.Fatalf("expected 1 prompt call on first connect, got %d", promptCalls)
	}

	// Second connect: should reuse saved password + known_hosts WITHOUT prompting!
	conn2, err := Connect(context.Background(), ConnectOptions{
		Manager:   mgr,
		CredStore: store,
		Host:      mgr.ResolveHost(saved),
		ErrOut:    &errOut,
		PromptSecret: func(prompt string) ([]byte, error) {
			t.Fatalf("unexpected password prompt on second connect: %s", prompt)
			return nil, nil
		},
	})
	if err != nil {
		t.Fatalf("second Connect with saved password failed: %v", err)
	}
	_ = conn2.Close()
}

func TestProxyJumpAndKnownHostsMismatch(t *testing.T) {
	bastion := startMockSSHServer(t, "jumpuser", "jumppass")
	target := startMockSSHServer(t, "appuser", "apppass")

	dir := t.TempDir()
	mgr, err := config.Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	mgr.Config.General.KnownHostsPath = filepath.Join(dir, "known_hosts")
	mgr.Config.General.StrictHostKeyChecking = "no"

	store := credstore.NewVaultStore(dir, func() ([]byte, error) { return []byte("vpass"), nil })

	bHost, err := mgr.UpsertHost(config.Host{
		Alias:    "bastion",
		Group:    "prod",
		Hostname: bastion.host,
		Port:     bastion.port,
		User:     "jumpuser",
	})
	if err != nil {
		t.Fatalf("upsert bastion: %v", err)
	}
	_ = store.Set(bHost.CredentialRef, []byte("jumppass"))

	tHost, err := mgr.UpsertHost(config.Host{
		Alias:    "internal-app",
		Group:    "prod",
		Hostname: target.host,
		Port:     target.port,
		User:     "appuser",
		JumpHost: "bastion",
	})
	if err != nil {
		t.Fatalf("upsert target: %v", err)
	}
	_ = store.Set(tHost.CredentialRef, []byte("apppass"))

	conn, err := Connect(context.Background(), ConnectOptions{
		Manager:   mgr,
		CredStore: store,
		Host:      mgr.ResolveHost(tHost),
	})
	if err != nil {
		t.Fatalf("ProxyJump Connect failed: %v", err)
	}
	defer conn.Close()

	var stdout, stderr bytes.Buffer
	if err := Exec(context.Background(), conn.Client, "via-bastion", &stdout, &stderr); err != nil {
		t.Fatalf("Exec over ProxyJump failed: %v", err)
	}
	if strings.TrimSpace(stdout.String()) != "echo:via-bastion" {
		t.Fatalf("unexpected ProxyJump exec output: %q", stdout.String())
	}

	// Verify known_hosts mismatch detection
	khPath := filepath.Join(dir, "strict_known_hosts")
	// Seed known_hosts with bastion's key under target's address to simulate MITM key change.
	if err := appendKnownHost(khPath, target.addr, bastion.hostKey.PublicKey()); err != nil {
		t.Fatalf("appendKnownHost: %v", err)
	}
	cb, err := BuildHostKeyCallback(khPath, "ask", "internal-app", strings.NewReader("yes\n"), &bytes.Buffer{})
	if err != nil {
		t.Fatalf("BuildHostKeyCallback: %v", err)
	}
	remoteAddr := &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: target.port}
	if err := cb(target.addr, remoteAddr, target.hostKey.PublicKey()); err == nil || !strings.Contains(err.Error(), "HAS CHANGED") {
		t.Fatalf("expected MITM key change warning, got: %v", err)
	}
}
