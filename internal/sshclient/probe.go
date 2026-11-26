package sshclient

import (
	"context"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/Oktanetflow/sshm/internal/config"
)

// ProbeStatus represents the TCP reachability state of an SSH host.
type ProbeStatus int

const (
	ProbeUnknown ProbeStatus = iota
	ProbeChecking
	ProbeOnline
	ProbeOffline
)

// ProbeResult captures the outcome of a single host TCP reachability probe.
type ProbeResult struct {
	HostID    string
	Alias     string
	Status    ProbeStatus
	Latency   time.Duration
	CheckedAt time.Time
	Err       string
}

// ProbeTCP checks whether host:port accepts TCP connections within timeout and measures RTT latency.
func ProbeTCP(ctx context.Context, host string, port int, timeout time.Duration) ProbeResult {
	if timeout <= 0 {
		timeout = 1500 * time.Millisecond
	}
	if port <= 0 {
		port = config.DefaultPort
	}
	addr := net.JoinHostPort(host, strconv.Itoa(port))
	dialer := &net.Dialer{Timeout: timeout}

	pCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	conn, err := dialer.DialContext(pCtx, "tcp", addr)
	elapsed := time.Since(start)
	now := time.Now()
	if err != nil {
		return ProbeResult{
			Status:    ProbeOffline,
			Latency:   elapsed,
			CheckedAt: now,
			Err:       err.Error(),
		}
	}
	_ = conn.Close()
	return ProbeResult{
		Status:    ProbeOnline,
		Latency:   elapsed,
		CheckedAt: now,
	}
}

// ProbeHosts probes multiple hosts concurrently with a bounded worker pool.
func ProbeHosts(ctx context.Context, hosts []config.ResolvedHost, timeout time.Duration, maxConcurrency int) []ProbeResult {
	if len(hosts) == 0 {
		return nil
	}
	if maxConcurrency <= 0 {
		maxConcurrency = 8
	}
	if maxConcurrency > len(hosts) {
		maxConcurrency = len(hosts)
	}

	results := make([]ProbeResult, len(hosts))
	jobs := make(chan int, len(hosts))
	for i := range hosts {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	for w := 0; w < maxConcurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				h := hosts[idx]
				res := ProbeTCP(ctx, h.Hostname, h.EffectivePort, timeout)
				res.HostID = h.ID
				res.Alias = h.Alias
				results[idx] = res
			}
		}()
	}
	wg.Wait()
	return results
}
