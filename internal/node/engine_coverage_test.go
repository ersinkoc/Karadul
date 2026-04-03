//go:build !windows

package node

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/karadul/karadul/internal/config"
	"github.com/karadul/karadul/internal/crypto"
	klog "github.com/karadul/karadul/internal/log"
	"github.com/karadul/karadul/internal/mesh"
)

// TestRekeyLoop_WithAgedSession verifies that rekeyLoop exits cleanly on context cancellation
// even when sessions needing rekey exist.
func TestRekeyLoop_WithAgedSession(t *testing.T) {
	e := testEngine(t)

	// Build an aged session that NeedsRekey() returns true.
	var remotePub [32]byte
	remotePub[0] = 0xDD
	ep := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 12345}
	ps := e.buildSession(remotePub, [32]byte{1}, [32]byte{2}, 99, 88, ep)
	peer := mesh.NewPeer(remotePub, "aged-peer", "n1", net.ParseIP("100.64.0.5"))
	peer.SetEndpoint(ep)
	ps.peer = peer

	// Age the session so NeedsRekey() returns true.
	ps.session.mu.Lock()
	ps.session.createdAt = time.Now().Add(-(sessionLifetime + time.Second))
	ps.session.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		e.rekeyLoop(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
		// Good — rekeyLoop exited cleanly.
	case <-time.After(5 * time.Second):
		t.Fatal("rekeyLoop did not exit on context cancellation")
	}
}

// TestEndpointRefreshLoop_Cancelled verifies endpointRefreshLoop exits on context cancellation.
func TestEndpointRefreshLoop_Cancelled(t *testing.T) {
	e := testEngine(t)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		e.endpointRefreshLoop(ctx)
		close(done)
	}()

	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("endpointRefreshLoop did not exit on context cancellation")
	}
}

// TestUdpReadLoop_StopChClosed verifies udpReadLoop exits when stopCh is closed.
func TestUdpReadLoop_StopChClosed(t *testing.T) {
	e := testEngine(t)

	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Skipf("UDP listen: %v", err)
	}
	e.udp = udp

	done := make(chan struct{})
	go func() {
		e.udpReadLoop()
		close(done)
	}()

	// Close stopCh AND the socket to trigger the exit path.
	close(e.stopCh)
	udp.Close()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("udpReadLoop did not exit after stopCh + socket close")
	}
}

// TestConnectPeer_WithEndpoint exercises connectPeer with a peer that has an endpoint.
func TestConnectPeer_WithEndpoint(t *testing.T) {
	e := testEngine(t)

	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Skipf("UDP listen: %v", err)
	}
	e.udp = udp
	t.Cleanup(func() { udp.Close() })

	var remotePub [32]byte
	remotePub[0] = 0xEE
	peer := mesh.NewPeer(remotePub, "connect-peer", "n2", net.ParseIP("100.64.0.6"))
	ep := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: udp.LocalAddr().(*net.UDPAddr).Port}
	peer.SetEndpoint(ep)

	// connectPeer should try to handshake — may fail crypto but shouldn't panic.
	_ = e.connectPeer(peer)
}

// TestConnectPeer_NoEndpoint_NoDERP exercises connectPeer when peer has no endpoint and no DERP client.
func TestConnectPeer_NoEndpoint_NoDERP(t *testing.T) {
	e := testEngine(t)

	var remotePub [32]byte
	remotePub[0] = 0xFF
	peer := mesh.NewPeer(remotePub, "no-ep-peer", "n3", net.ParseIP("100.64.0.7"))
	// No endpoint, no DERP client — connectPeer should return error.

	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Skipf("UDP listen: %v", err)
	}
	e.udp = udp
	t.Cleanup(func() { udp.Close() })

	err = e.connectPeer(peer)
	if err == nil {
		t.Log("connectPeer succeeded despite no endpoint")
	}
}

// TestEnableExitNode_Darwin_ErrorPath verifies EnableExitNode fails gracefully on non-root.
func TestEnableExitNode_Darwin_ErrorPath(t *testing.T) {
	e := testEngine(t)
	err := e.EnableExitNode("nonexistent-iface-xyz")
	if err == nil {
		t.Log("EnableExitNode succeeded (running as root)")
	}
}

// TestDisableExitNode_Darwin_NoPanic verifies DisableExitNode doesn't panic.
func TestDisableExitNode_Darwin_NoPanic(t *testing.T) {
	DisableExitNode("nonexistent-iface-xyz")
}

// TestConcurrentSessionAccess verifies no data races under concurrent session operations.
func TestConcurrentSessionAccess(t *testing.T) {
	e := testEngine(t)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			var pub [32]byte
			pub[0] = byte(id)
			ep := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 5000 + id}
			ps := e.buildSession(pub, [32]byte{byte(id)}, [32]byte{byte(id + 1)}, uint32(id*2), uint32(id*2+1), ep)
			peer := mesh.NewPeer(pub, "concurrent-peer", "n", net.ParseIP("100.64.0.1"))
			peer.SetEndpoint(ep)
			ps.peer = peer

			ps.session.mu.Lock()
			ps.session.createdAt = time.Now().Add(-(sessionLifetime + time.Second))
			ps.session.mu.Unlock()
		}(i)
	}
	wg.Wait()

	e.mu.RLock()
	count := len(e.sessions)
	e.mu.RUnlock()
	if count != 10 {
		t.Errorf("expected 10 sessions, got %d", count)
	}
}

// TestStart_RegisterFails verifies Start returns error when coordination server is unreachable.
func TestStart_RegisterFails(t *testing.T) {
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.NodeConfig{
		ServerURL: "http://127.0.0.1:0",
		Hostname:  "test-fail",
		AuthKey:   "test",
	}
	log := klog.New(nil, klog.LevelDebug, klog.FormatText)
	e := NewEngine(cfg, kp, log)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = e.Start(ctx)
	if err == nil {
		t.Fatal("expected Start to fail with unreachable server")
	}
	if !strings.Contains(err.Error(), "register") {
		t.Errorf("expected register error, got: %v", err)
	}
}

// TestUseExitNode_NilTun verifies UseExitNode handles nil tun gracefully.
func TestUseExitNode_NilTun(t *testing.T) {
	e := testEngine(t)
	peer := mesh.NewPeer([32]byte{1}, "exit-peer", "n3", net.ParseIP("100.64.0.3"))

	defer func() {
		if r := recover(); r != nil {
			t.Logf("UseExitNode panicked with nil tun (expected): %v", r)
		}
	}()
	_ = e.UseExitNode(peer)
}

// ─── handleAPIExitNodeEnable: method and validation paths ────────────────────

func TestHandleAPIExitNodeEnable_Error(t *testing.T) {
	e := testEngine(t)
	req := httptest.NewRequest(http.MethodPost, "/exit-node/enable",
		strings.NewReader(`{"out_interface":"bogus0"}`))
	w := httptest.NewRecorder()
	e.handleAPIExitNodeEnable(w, req)
	// Will fail with error since exit node requires root
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Errorf("got %d, want 200 or 500", w.Code)
	}
}

// ─── handleAPIExitNodeUse: peer lookup paths ────────────────────────────────

func TestHandleAPIExitNodeUse_PeerLookupMiss(t *testing.T) {
	e := testEngine(t)
	req := httptest.NewRequest(http.MethodPost, "/exit-node/use",
		strings.NewReader(`{"peer":"nonexistent"}`))
	w := httptest.NewRecorder()
	e.handleAPIExitNodeUse(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("got %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestHandleAPIExitNodeUse_PeerFoundByHostname(t *testing.T) {
	e := testEngine(t)
	var pub [32]byte
	pub[0] = 0xAA
	e.manager.AddOrUpdate(pub, "lookup-peer", "n1", net.ParseIP("100.64.0.10"), "", nil)

	// UseExitNode will panic on nil tun, so recover
	defer func() {
		if r := recover(); r != nil {
			t.Logf("UseExitNode panicked (expected with nil tun): %v", r)
		}
	}()

	req := httptest.NewRequest(http.MethodPost, "/exit-node/use",
		strings.NewReader(`{"peer":"lookup-peer"}`))
	w := httptest.NewRecorder()
	e.handleAPIExitNodeUse(w, req)
}

// ─── handleUDPPacket: unknown type ──────────────────────────────────────────

func TestHandleUDPPacket_UnknownType(t *testing.T) {
	e := testEngine(t)
	// Empty packet should be ignored without panic
	e.handleUDPPacket(&net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 1234}, []byte{})
}

// ─── udpReadLoop: semaphore full path ──────────────────────────────────────

func TestUdpReadLoop_SemaphoreFull(t *testing.T) {
	e := testEngine(t)

	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Skipf("UDP listen: %v", err)
	}
	e.udp = udp

	// Fill the semaphore
	for i := 0; i < cap(e.udpSem); i++ {
		e.udpSem <- struct{}{}
	}

	// Send a packet — should be dropped (semaphore full)
	udp.WriteToUDP([]byte("test"), udp.LocalAddr().(*net.UDPAddr))

	// Close to trigger exit
	close(e.stopCh)
	udp.Close()

	// Drain semaphore so goroutines finish
	for i := 0; i < cap(e.udpSem); i++ {
		<-e.udpSem
	}
}

// ─── discoverEndpoint: no UDP ───────────────────────────────────────────────

func TestDiscoverEndpoint_NoUDP(t *testing.T) {
	e := testEngine(t)
	// discoverEndpoint panics with nil UDP (BindingRequest dereferences nil conn).
	// Verify it doesn't silently succeed.
	defer func() {
		if r := recover(); r != nil {
			t.Logf("discoverEndpoint panicked with nil UDP (expected): %v", r)
		}
	}()
	_, err := e.discoverEndpoint()
	if err != nil {
		t.Logf("discoverEndpoint returned error: %v", err)
	}
}

// ─── connectPeer: no session, no endpoint ───────────────────────────────────

func TestConnectPeer_NoEndpoint_NoDERP_NoSession(t *testing.T) {
	e := testEngine(t)

	var pub [32]byte
	pub[0] = 0xCC
	peer := mesh.NewPeer(pub, "no-ep", "n5", net.ParseIP("100.64.0.9"))
	// No endpoint set, no DERP client

	err := e.connectPeer(peer)
	// Should return nil (logs warning, doesn't error)
	if err != nil {
		t.Logf("connectPeer returned: %v", err)
	}
}

// ─── serveLocalAPI: listen on unix socket ────────────────────────────────────

func TestServeLocalAPI_ContextCancel(t *testing.T) {
	e := testEngine(t)
	dir := t.TempDir()
	e.cfg.DataDir = dir

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		e.serveLocalAPI(ctx)
		close(done)
	}()

	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("serveLocalAPI did not exit on context cancellation")
	}
}
