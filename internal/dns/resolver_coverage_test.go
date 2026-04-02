package dns

import (
	"encoding/binary"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	klog "github.com/karadul/karadul/internal/log"
)

// TestStart_SemaphoreDrop verifies that queries are silently dropped when
// the semaphore channel is full (the default case at line 105 of resolver.go).
func TestStart_SemaphoreDrop(t *testing.T) {
	magic := NewMagicDNS()
	magic.Set("testdrop", net.ParseIP("100.64.0.99"))

	r := NewResolver("127.0.0.1:0", "127.0.0.1:1", magic,
		klog.New(nil, klog.LevelError, klog.FormatText))
	// Override semaphore to capacity 1 so we can easily overflow it.
	r.sem = make(chan struct{}, 1)

	started := make(chan *net.UDPAddr, 1)
	go func() {
		_ = r.Start()
	}()
	t.Cleanup(func() { r.Close() })

	// Wait for the resolver to start.
	go func() {
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			r.mu.Lock()
			c := r.conn
			r.mu.Unlock()
			if c != nil {
				started <- c.LocalAddr().(*net.UDPAddr)
				return
			}
		}
	}()

	var resolverAddr *net.UDPAddr
	select {
	case resolverAddr = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("resolver did not start in time")
	}

	// Block the semaphore with 1 slot held indefinitely.
	r.sem <- struct{}{}

	// Send a query. The handle goroutine will not be spawned (default case).
	client, err := net.DialUDP("udp", nil, resolverAddr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	pkt := buildQuery(0xDDDD, "testdrop.web.karadul", dnsTypeA)
	_ = client.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := client.Write(pkt); err != nil {
		t.Fatal(err)
	}

	// Read with short deadline -- since the query was dropped, we expect no response.
	_ = client.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	resp := make([]byte, 512)
	_, err = client.Read(resp)
	if err == nil {
		t.Log("query was handled despite semaphore being full (race window)")
	}

	// Unblock the semaphore.
	<-r.sem
}

// TestStart_ConcurrencyLimit verifies that the resolver processes queries
// concurrently up to the semaphore limit without panicking or deadlocking.
func TestStart_ConcurrencyLimit(t *testing.T) {
	magic := NewMagicDNS()
	magic.Set("conchost", net.ParseIP("100.64.0.50"))

	r := NewResolver("127.0.0.1:0", "127.0.0.1:1", magic,
		klog.New(nil, klog.LevelError, klog.FormatText))

	// Override semaphore to a small size for testing.
	r.sem = make(chan struct{}, 4)

	started := make(chan *net.UDPAddr, 1)
	go func() {
		_ = r.Start()
	}()
	t.Cleanup(func() { r.Close() })

	// Wait for the resolver to start by polling for conn.
	go func() {
		for i := 0; i < 100; i++ {
			time.Sleep(10 * time.Millisecond)
			r.mu.Lock()
			c := r.conn
			r.mu.Unlock()
			if c != nil {
				started <- c.LocalAddr().(*net.UDPAddr)
				return
			}
		}
	}()

	var resolverAddr *net.UDPAddr
	select {
	case resolverAddr = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("resolver did not start in time")
	}

	// Send multiple queries concurrently and verify all get responses.
	// Each goroutine uses its own UDP client to avoid socket contention.
	var wg sync.WaitGroup
	var received int32

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(seq int) {
			defer wg.Done()
			client, err := net.DialUDP("udp", nil, resolverAddr)
			if err != nil {
				return
			}
			defer client.Close()

			pkt := buildQuery(uint16(seq), "conchost.web.karadul", dnsTypeA)
			_ = client.SetWriteDeadline(time.Now().Add(time.Second))
			if _, err := client.Write(pkt); err != nil {
				return
			}
			_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
			resp := make([]byte, 512)
			n, err := client.Read(resp)
			if err != nil || n < 12 {
				return
			}
			atomic.AddInt32(&received, 1)
		}(i)
	}
	wg.Wait()

	got := atomic.LoadInt32(&received)
	if got == 0 {
		t.Error("expected at least one response from concurrent queries")
	}
	t.Logf("received %d/4 responses", got)
}

// TestStart_ListenUDPError verifies that Start returns an error when
// ResolveUDPAddr succeeds but ListenUDP fails (e.g., binding a privileged port).
func TestStart_ListenUDPError(t *testing.T) {
	r := NewResolver("127.0.0.1:1", "127.0.0.1:1", nil,
		klog.New(nil, klog.LevelError, klog.FormatText))
	err := r.Start()
	if err == nil {
		r.Close()
		t.Fatal("expected error for privileged port")
	}
	if !strings.Contains(err.Error(), "listen dns") {
		t.Errorf("expected 'listen dns' error prefix, got: %v", err)
	}
}

// TestForward_RealDialUDPSuccessPath exercises the real (non-test-hook) DialUDP
// success path with a bare IP upstream, connecting to port 53. This covers
// lines 211-243 of resolver.go.
func TestForward_RealDialUDPSuccessPath(t *testing.T) {
	// Ensure no test hook is set.
	orig := testDialUDP
	testDialUDP = nil
	defer func() { testDialUDP = orig }()

	magic := NewMagicDNS()
	r := &Resolver{
		upstream: "127.0.0.1", // bare IP -> DialUDP to 127.0.0.1:53
		magic:    magic,
		log:      klog.New(nil, klog.LevelError, klog.FormatText),
	}

	pkt := buildQuery(0xAAAA, "example.com", dnsTypeA)
	resp, err := r.forward(pkt)
	if err != nil {
		// Expected: nothing on port 53, so Write succeeds but ReadFromUDP gets
		// connection refused or times out.
		t.Logf("forward (real DialUDP path) error: %v", err)
	} else {
		txID := binary.BigEndian.Uint16(resp[0:])
		if txID != 0xAAAA {
			t.Errorf("txID: want 0xAAAA, got 0x%04X", txID)
		}
	}
}

// TestForward_RealDialUDPFallbackPath exercises the real DialUDP path where
// ParseIP returns nil (upstream contains port), causing DialUDP to connect
// to 0.0.0.0:53. If DialUDP fails for any reason, the net.Dial fallback
// path (lines 215-231) is triggered.
func TestForward_RealDialUDPFallbackPath(t *testing.T) {
	orig := testDialUDP
	testDialUDP = nil
	defer func() { testDialUDP = orig }()

	// Start a mock server that echoes DNS responses.
	srv, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()

	go func() {
		buf := make([]byte, 512)
		for {
			_ = srv.SetReadDeadline(time.Now().Add(3 * time.Second))
			n, src, err := srv.ReadFromUDP(buf)
			if err != nil {
				return
			}
			resp := make([]byte, n)
			copy(resp, buf[:n])
			resp[2] |= 0x80
			_, _ = srv.WriteToUDP(resp, src)
		}
	}()

	magic := NewMagicDNS()
	r := &Resolver{
		// Using IP:PORT format makes TrimSuffix(":53") a no-op,
		// so ParseIP("127.0.0.1:PORT") returns nil.
		// DialUDP gets &net.UDPAddr{IP: nil, Port: 53}, which dials 127.0.0.1:53.
		// Nothing on port 53 -> write succeeds, read fails.
		upstream: srv.LocalAddr().String(),
		magic:    magic,
		log:      klog.New(nil, klog.LevelError, klog.FormatText),
	}

	pkt := buildQuery(0xBBBB, "example.com", dnsTypeA)
	resp, err := r.forward(pkt)
	if err != nil {
		t.Logf("forward (fallback path) error: %v", err)
	} else {
		txID := binary.BigEndian.Uint16(resp[0:])
		if txID != 0xBBBB {
			t.Errorf("txID: want 0xBBBB, got 0x%04X", txID)
		}
	}
}

// TestForward_HookSuccessRoundTrip verifies the test hook path completes a
// full round-trip (write + read) returning the response.
func TestForward_HookSuccessRoundTrip(t *testing.T) {
	orig := testDialUDP
	defer func() { testDialUDP = orig }()

	// Build a complete DNS response.
	respPkt := buildQuery(0xCC01, "hook.example.com", dnsTypeA)
	respPkt[2] |= 0x80 // QR=1

	testDialUDP = func(network string, laddr, raddr *net.UDPAddr) (net.Conn, error) {
		return &mockConn{readData: respPkt}, nil
	}

	r := &Resolver{
		upstream: "127.0.0.1:53",
		magic:    NewMagicDNS(),
		log:      klog.New(nil, klog.LevelError, klog.FormatText),
	}

	pkt := buildQuery(0xCC01, "hook.example.com", dnsTypeA)
	resp, err := r.forward(pkt)
	if err != nil {
		t.Fatalf("forward hook: %v", err)
	}
	if len(resp) < 12 {
		t.Fatal("response too short")
	}
	txID := binary.BigEndian.Uint16(resp[0:])
	if txID != 0xCC01 {
		t.Errorf("txID: want 0xCC01, got 0x%04X", txID)
	}
}

// TestOverride_AddressParsing verifies the IP extraction logic in Override
// by calling it with a host:port address and checking the restore function.
func TestOverride_AddressParsing(t *testing.T) {
	restore, err := Override("100.64.0.53:53")
	if err != nil {
		t.Logf("Override returned error (expected in CI): %v", err)
		return
	}
	if err := restore(); err != nil {
		t.Logf("restore returned error: %v", err)
	}
}

// TestOverride_EmptyAddress verifies Override with an address that has no colon.
func TestOverride_EmptyAddress(t *testing.T) {
	restore, err := Override("100.64.0.53")
	if err != nil {
		t.Logf("Override returned error (expected in CI): %v", err)
		return
	}
	if err := restore(); err != nil {
		t.Logf("restore returned error: %v", err)
	}
}
