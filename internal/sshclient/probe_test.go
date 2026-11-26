package sshclient

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/oktaaa17/sshm/internal/config"
)

func TestProbeTCPOnlineAndOffline(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start test listener: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()

	tcpAddr := ln.Addr().(*net.TCPAddr)
	online := ProbeTCP(context.Background(), "127.0.0.1", tcpAddr.Port, time.Second)
	if online.Status != ProbeOnline {
		t.Fatalf("expected ProbeOnline, got %v (err: %s)", online.Status, online.Err)
	}
	if online.Latency < 0 {
		t.Fatalf("expected non-negative latency, got %v", online.Latency)
	}

	// Find a closed port by opening and immediately closing a listener.
	closedLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to reserve closed port: %v", err)
	}
	closedPort := closedLn.Addr().(*net.TCPAddr).Port
	_ = closedLn.Close()

	offline := ProbeTCP(context.Background(), "127.0.0.1", closedPort, 400*time.Millisecond)
	if offline.Status != ProbeOffline {
		t.Fatalf("expected ProbeOffline for closed port, got %v", offline.Status)
	}

	// Verify bounded batch probing.
	batch := ProbeHosts(context.Background(), []config.ResolvedHost{
		{Host: config.Host{ID: "h1", Alias: "up", Hostname: "127.0.0.1"}, EffectivePort: tcpAddr.Port},
		{Host: config.Host{ID: "h2", Alias: "down", Hostname: "127.0.0.1"}, EffectivePort: closedPort},
	}, 500*time.Millisecond, 2)

	if len(batch) != 2 {
		t.Fatalf("expected 2 results, got %d", len(batch))
	}
	if batch[0].Status != ProbeOnline || batch[0].HostID != "h1" {
		t.Errorf("unexpected result[0]: %+v", batch[0])
	}
	if batch[1].Status != ProbeOffline || batch[1].HostID != "h2" {
		t.Errorf("unexpected result[1]: %+v", batch[1])
	}
}
