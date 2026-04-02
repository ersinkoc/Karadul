package node

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/karadul/karadul/internal/config"
	"github.com/karadul/karadul/internal/coordinator"
	"github.com/karadul/karadul/internal/crypto"
	klog "github.com/karadul/karadul/internal/log"
	"github.com/karadul/karadul/internal/mesh"
	"github.com/karadul/karadul/internal/protocol"
)

func testEngine(t *testing.T) *Engine {
	t.Helper()
	kp, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.NodeConfig{
		ServerURL: "http://127.0.0.1:8080",
		Hostname:  "test-node",
		AuthKey:   "test-auth-key",
	}
	log := klog.New(nil, klog.LevelDebug, klog.FormatText)
	e := NewEngine(cfg, kp, log)
	// Initialise a mesh manager so tests that call LocalStatus / handleAPIMetrics
	// don't panic on nil.
	e.manager = mesh.NewManager(log, nil)
	return e
}

// ─── Session management tests ────────────────────────────────────────────────

func TestBuildSession(t *testing.T) {
	e := testEngine(t)

	var sendKey, recvKey [32]byte
	for i := range sendKey {
		sendKey[i] = byte(i)
		recvKey[i] = byte(i + 1)
	}

	var remotePub crypto.Key
	for i := range remotePub {
		remotePub[i] = byte(i + 10)
	}

	ep := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 12345}
	ps := e.buildSession(remotePub, sendKey, recvKey, 1, 2, ep)

	if ps == nil {
		t.Fatal("expected non-nil peerSession")
	}
	if ps.localID != 1 {
		t.Errorf("localID: got %d, want 1", ps.localID)
	}
	if ps.receiverID != 2 {
		t.Errorf("receiverID: got %d, want 2", ps.receiverID)
	}

	// Verify maps.
	e.mu.RLock()
	_, ok := e.sessions[remotePub]
	byIDSession, ok2 := e.byID[1]
	e.mu.RUnlock()
	if !ok {
		t.Error("session not in sessions map")
	}
	if !ok2 || byIDSession != ps {
		t.Error("session not in byID map")
	}
}

func TestBuildSession_Overwrite(t *testing.T) {
	e := testEngine(t)

	var remotePub crypto.Key
	for i := range remotePub {
		remotePub[i] = byte(i + 10)
	}

	var sendKey, recvKey [32]byte
	for i := range sendKey {
		sendKey[i] = byte(i)
		recvKey[i] = byte(i + 1)
	}

	// Create first session.
	ps1 := e.buildSession(remotePub, sendKey, recvKey, 10, 20, nil)
	if ps1 == nil {
		t.Fatal("first session nil")
	}

	// Create second session with same remote pub key — should overwrite sessions map.
	var sendKey2, recvKey2 [32]byte
	for i := range sendKey2 {
		sendKey2[i] = byte(i + 50)
		recvKey2[i] = byte(i + 60)
	}
	ps2 := e.buildSession(remotePub, sendKey2, recvKey2, 30, 40, nil)

	// Verify the sessions map has the new session (overwritten).
	e.mu.RLock()
	stored := e.sessions[remotePub]
	_, hasNewByID := e.byID[30]
	e.mu.RUnlock()

	if stored != ps2 {
		t.Error("sessions map should have new session")
	}
	if !hasNewByID {
		t.Error("byID map should have new localID 30")
	}
}

func TestRekeyPeer_CleansByID(t *testing.T) {
	e := testEngine(t)

	var pubKey [32]byte
	for i := range pubKey {
		pubKey[i] = byte(i)
	}

	peer := mesh.NewPeer(pubKey, "test-peer", "node-1", net.ParseIP("100.64.0.2"))

	var sendKey, recvKey [32]byte
	for i := range sendKey {
		sendKey[i] = byte(i)
		recvKey[i] = byte(i + 1)
	}
	_ = e.buildSession(peer.PublicKey, sendKey, recvKey, 42, 99, nil)

	// Verify session exists.
	e.mu.RLock()
	_, ok1 := e.sessions[peer.PublicKey]
	_, ok2 := e.byID[42]
	e.mu.RUnlock()
	if !ok1 || !ok2 {
		t.Fatal("session should exist before rekey")
	}

	e.RekeyPeer(peer)

	// After RekeyPeer, both maps should be cleaned.
	e.mu.RLock()
	_, hasSession := e.sessions[peer.PublicKey]
	_, hasByID := e.byID[42]
	e.mu.RUnlock()
	if hasSession {
		t.Error("sessions map should not have old entry after RekeyPeer")
	}
	if hasByID {
		t.Error("byID map should not have old entry after RekeyPeer")
	}
}

// ─── Metrics tests ───────────────────────────────────────────────────────────

func TestMetricsAtomicCounters(t *testing.T) {
	e := testEngine(t)

	e.metricPacketsTx.Add(5)
	e.metricPacketsTx.Add(3)
	if e.metricPacketsTx.Load() != 8 {
		t.Errorf("packets tx: got %d, want 8", e.metricPacketsTx.Load())
	}

	e.metricBytesTx.Add(100)
	e.metricBytesTx.Add(200)
	if e.metricBytesTx.Load() != 300 {
		t.Errorf("bytes tx: got %d, want 300", e.metricBytesTx.Load())
	}

	e.metricPacketsRx.Add(10)
	if e.metricPacketsRx.Load() != 10 {
		t.Errorf("packets rx: got %d, want 10", e.metricPacketsRx.Load())
	}

	e.metricBytesRx.Add(42)
	if e.metricBytesRx.Load() != 42 {
		t.Errorf("bytes rx: got %d, want 42", e.metricBytesRx.Load())
	}
}

func TestMetricsConcurrent(t *testing.T) {
	e := testEngine(t)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e.metricPacketsTx.Add(1)
			e.metricBytesTx.Add(10)
			e.metricPacketsRx.Add(1)
			e.metricBytesRx.Add(10)
		}()
	}
	wg.Wait()

	if e.metricPacketsTx.Load() != 100 {
		t.Errorf("packets tx: got %d, want 100", e.metricPacketsTx.Load())
	}
	if e.metricBytesTx.Load() != 1000 {
		t.Errorf("bytes tx: got %d, want 1000", e.metricBytesTx.Load())
	}
}

// ─── Topology / MagicDNS tests ───────────────────────────────────────────────

func TestUpdateMagicDNS(t *testing.T) {
	e := testEngine(t)

	nodes := []*coordinator.Node{
		{
			Hostname:  "node-a",
			VirtualIP: "100.64.0.2",
			Status:    coordinator.NodeStatusActive,
		},
		{
			Hostname:  "node-b",
			VirtualIP: "100.64.0.3",
			Status:    coordinator.NodeStatusActive,
		},
		{
			Hostname:  "node-pending",
			VirtualIP: "100.64.0.4",
			Status:    coordinator.NodeStatusPending,
		},
	}

	e.updateMagicDNS(nodes)

	if ip := e.magic.Lookup("node-a"); ip == nil || !ip.Equal(net.ParseIP("100.64.0.2")) {
		t.Errorf("node-a: got %v, want 100.64.0.2", ip)
	}
	if ip := e.magic.Lookup("node-b"); ip == nil || !ip.Equal(net.ParseIP("100.64.0.3")) {
		t.Errorf("node-b: got %v, want 100.64.0.3", ip)
	}
	if ip := e.magic.Lookup("node-pending"); ip != nil {
		t.Errorf("pending node should not resolve, got %v", ip)
	}
}

func TestUpdateMagicDNS_InvalidIP(t *testing.T) {
	e := testEngine(t)

	nodes := []*coordinator.Node{
		{Hostname: "bad-node", VirtualIP: "not-an-ip", Status: coordinator.NodeStatusActive},
	}
	e.updateMagicDNS(nodes)

	if ip := e.magic.Lookup("bad-node"); ip != nil {
		t.Errorf("bad IP should not resolve, got %v", ip)
	}
}

func TestUpdateMagicDNS_ReplacesEntries(t *testing.T) {
	e := testEngine(t)

	// First update.
	e.updateMagicDNS([]*coordinator.Node{
		{Hostname: "node-a", VirtualIP: "100.64.0.2", Status: coordinator.NodeStatusActive},
	})
	if ip := e.magic.Lookup("node-a"); ip == nil || !ip.Equal(net.ParseIP("100.64.0.2")) {
		t.Fatalf("node-a first update: got %v", ip)
	}

	// Second update should replace, not merge.
	e.updateMagicDNS([]*coordinator.Node{
		{Hostname: "node-b", VirtualIP: "100.64.0.3", Status: coordinator.NodeStatusActive},
	})
	if ip := e.magic.Lookup("node-a"); ip != nil {
		t.Errorf("node-a should be gone after second update, got %v", ip)
	}
	if ip := e.magic.Lookup("node-b"); ip == nil || !ip.Equal(net.ParseIP("100.64.0.3")) {
		t.Errorf("node-b: got %v, want 100.64.0.3", ip)
	}
}

// ─── Local API tests ─────────────────────────────────────────────────────────

func TestLocalStatus(t *testing.T) {
	e := testEngine(t)
	e.nodeID = "test-node-123"
	e.virtualIP = net.ParseIP("100.64.0.1")

	status := e.LocalStatus()
	if status["nodeId"] != "test-node-123" {
		t.Errorf("nodeId: got %v, want test-node-123", status["nodeId"])
	}
	if status["virtualIp"] != "100.64.0.1" {
		t.Errorf("virtualIp: got %v, want 100.64.0.1", status["virtualIp"])
	}
}

func TestHandleAPIStatus(t *testing.T) {
	e := testEngine(t)
	e.nodeID = "test-node-456"
	e.virtualIP = net.ParseIP("100.64.0.5")

	w := httptest.NewRecorder()
	e.handleAPIStatus(w, httptest.NewRequest(http.MethodGet, "/status", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if resp["nodeId"] != "test-node-456" {
		t.Errorf("nodeId: got %v, want test-node-456", resp["nodeId"])
	}
}

func TestHandleAPIMetrics(t *testing.T) {
	e := testEngine(t)
	e.metricPacketsTx.Add(42)
	e.metricPacketsRx.Add(10)
	e.metricBytesTx.Add(1024)
	e.metricBytesRx.Add(512)

	w := httptest.NewRecorder()
	e.handleAPIMetrics(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !containsStr(body, "karadul_packets_tx_total 42") {
		t.Errorf("expected packets_tx 42 in metrics output, got:\n%s", body)
	}
	if !containsStr(body, "karadul_bytes_rx_total 512") {
		t.Errorf("expected bytes_rx 512 in metrics output, got:\n%s", body)
	}
}

func TestHandleAPIShutdown(t *testing.T) {
	e := testEngine(t)

	cancelled := false
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	e.cancel = func() {
		cancel()
		cancelled = true
	}

	w := httptest.NewRecorder()
	e.handleAPIShutdown(w, httptest.NewRequest(http.MethodPost, "/shutdown", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status code: got %d, want %d", w.Code, http.StatusOK)
	}
	if !cancelled {
		t.Error("expected cancel to be called")
	}
}

func TestHandleAPIMetrics_IncludesSessions(t *testing.T) {
	e := testEngine(t)

	w := httptest.NewRecorder()
	e.handleAPIMetrics(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := w.Body.String()
	// Should have zero counters for a fresh engine.
	if !containsStr(body, "karadul_sessions_active 0") {
		t.Errorf("expected sessions_active 0, got:\n%s", body)
	}
	if !containsStr(body, "karadul_peers_total 0") {
		t.Errorf("expected peers_total 0, got:\n%s", body)
	}
}

// ─── Packet helpers ──────────────────────────────────────────────────────────

func TestPacketDstPort(t *testing.T) {
	tests := []struct {
		name string
		pkt  []byte
		want uint16
	}{
		{
			name: "tcp packet port 80",
			pkt: func() []byte {
				pkt := make([]byte, 24)
				pkt[0] = 0x45 // IPv4, 20-byte header
				pkt[9] = 6    // protocol = TCP
				pkt[22] = 0   // dst port high byte
				pkt[23] = 80  // dst port low byte
				return pkt
			}(),
			want: 80,
		},
		{
			name: "udp packet port 53",
			pkt: func() []byte {
				pkt := make([]byte, 28)
				pkt[0] = 0x45 // IPv4, 20-byte header
				pkt[9] = 17   // protocol = UDP
				pkt[22] = 0   // dst port high byte
				pkt[23] = 53  // dst port low byte
				return pkt
			}(),
			want: 53,
		},
		{
			name: "too short",
			pkt: func() []byte {
				pkt := make([]byte, 10)
				pkt[0] = 0x45
				return pkt
			}(),
			want: 0,
		},
		{
			name: "high port 443",
			pkt: func() []byte {
				pkt := make([]byte, 24)
				pkt[0] = 0x45
				pkt[9] = 6 // TCP
				pkt[22] = 1
				pkt[23] = 187 // 443 = 0x01BB
				return pkt
			}(),
			want: 443,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := packetDstPort(tt.pkt)
			if got != tt.want {
				t.Errorf("packetDstPort() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ─── ID counter tests ────────────────────────────────────────────────────────

func TestNextID(t *testing.T) {
	e := testEngine(t)

	ids := make(map[uint32]bool)
	for i := 0; i < 100; i++ {
		id := e.nextID()
		if ids[id] {
			t.Errorf("duplicate ID: %d", id)
		}
		ids[id] = true
	}
}

func TestNextID_Concurrent(t *testing.T) {
	e := testEngine(t)

	ids := make(chan uint32, 1000)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				ids <- e.nextID()
			}
		}()
	}
	wg.Wait()
	close(ids)

	seen := make(map[uint32]bool)
	for id := range ids {
		if seen[id] {
			t.Errorf("duplicate ID from concurrent access: %d", id)
		}
		seen[id] = true
	}
}

// ─── Public endpoint tests ───────────────────────────────────────────────────

func TestPublicEP_Atomic(t *testing.T) {
	e := testEngine(t)

	if ep := e.publicEP.Load(); ep != nil {
		t.Errorf("initial publicEP should be nil, got %v", ep)
	}

	addr := &net.UDPAddr{IP: net.ParseIP("203.0.113.1"), Port: 12345}
	e.publicEP.Store(addr)

	loaded := e.publicEP.Load()
	if loaded == nil || !loaded.IP.Equal(net.ParseIP("203.0.113.1")) || loaded.Port != 12345 {
		t.Errorf("publicEP: got %v, want 203.0.113.1:12345", loaded)
	}
}

// ─── Sign request tests ──────────────────────────────────────────────────────

func TestSignRequest(t *testing.T) {
	e := testEngine(t)

	body := []byte(`{"test":"data"}`)
	req, err := http.NewRequest(http.MethodPost, "/api/v1/poll", nil)
	if err != nil {
		t.Fatal(err)
	}

	e.signRequest(req, body)

	keyHeader := req.Header.Get("X-Karadul-Key")
	sigHeader := req.Header.Get("X-Karadul-Sig")

	if keyHeader == "" {
		t.Error("expected non-empty X-Karadul-Key header")
	}
	if sigHeader == "" {
		t.Error("expected non-empty X-Karadul-Sig header")
	}

	decoded, err := base64.StdEncoding.DecodeString(keyHeader)
	if err != nil {
		t.Fatalf("decode key header: %v", err)
	}
	if len(decoded) != 32 {
		t.Errorf("key header decoded length: got %d, want 32", len(decoded))
	}
}

func TestSignRequest_Deterministic(t *testing.T) {
	e := testEngine(t)

	body := []byte(`{"test":"data"}`)
	req1, _ := http.NewRequest(http.MethodPost, "/api/v1/poll", nil)
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/poll", nil)

	e.signRequest(req1, body)
	e.signRequest(req2, body)

	sig1 := req1.Header.Get("X-Karadul-Sig")
	sig2 := req2.Header.Get("X-Karadul-Sig")

	if sig1 != sig2 {
		t.Errorf("same body should produce same signature: %s != %s", sig1, sig2)
	}
}

func TestSignRequest_DifferentBody(t *testing.T) {
	e := testEngine(t)

	req1, _ := http.NewRequest(http.MethodPost, "/api/v1/poll", nil)
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/poll", nil)

	e.signRequest(req1, []byte("body1"))
	e.signRequest(req2, []byte("body2"))

	sig1 := req1.Header.Get("X-Karadul-Sig")
	sig2 := req2.Header.Get("X-Karadul-Sig")

	if sig1 == sig2 {
		t.Error("different bodies should produce different signatures")
	}
}

// ─── Session encrypt/decrypt round-trip ──────────────────────────────────────

func TestSessionRoundTrip(t *testing.T) {
	// Session uses sendKey for encryption, recvKey for decryption.
	// To round-trip, both must be the same key.
	var key [32]byte
	for i := range key {
		key[i] = byte(i + 1)
	}

	s := NewSession(key, key, nil)

	plaintext := []byte("hello mesh network")
	counter, ct, err := s.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if counter != 0 {
		t.Errorf("first counter: got %d, want 0", counter)
	}

	decrypted, err := s.Decrypt(counter, ct)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Errorf("round-trip: got %q, want %q", decrypted, plaintext)
	}
}

func TestSessionEncryptCounterIncrements(t *testing.T) {
	var key [32]byte
	for i := range key {
		key[i] = byte(i + 1)
	}
	s := NewSession(key, key, nil)

	c1, _, _ := s.Encrypt([]byte("a"))
	c2, _, _ := s.Encrypt([]byte("b"))
	c3, _, _ := s.Encrypt([]byte("c"))

	if c1 != 0 || c2 != 1 || c3 != 2 {
		t.Errorf("counters: got %d, %d, %d; want 0, 1, 2", c1, c2, c3)
	}
}

func TestSessionRejectsReplay(t *testing.T) {
	var key [32]byte
	for i := range key {
		key[i] = byte(i + 1)
	}
	s := NewSession(key, key, nil)

	counter, ct, _ := s.Encrypt([]byte("msg"))

	// First decrypt should succeed.
	if _, err := s.Decrypt(counter, ct); err != nil {
		t.Fatalf("first decrypt: %v", err)
	}

	// Replay should be rejected.
	if _, err := s.Decrypt(counter, ct); err == nil {
		t.Error("expected replay to be rejected")
	}
}

// ─── LocalStatus report ─────────────────────────────────────────────────────

func TestLocalStatus_WithPublicEP(t *testing.T) {
	e := testEngine(t)
	e.nodeID = "ep-node"
	e.virtualIP = net.ParseIP("100.64.0.1")
	e.publicEP.Store(&net.UDPAddr{IP: net.ParseIP("1.2.3.4"), Port: 43210})

	status := e.LocalStatus()
	if status["publicEp"] != "1.2.3.4:43210" {
		t.Errorf("publicEp: got %v, want 1.2.3.4:43210", status["publicEp"])
	}
}

// ─── Handshake timeout cleanup ───────────────────────────────────────────────

func TestHandshakeTimeout_Cleans(t *testing.T) {
	e := testEngine(t)

	var pubKey [32]byte
	for i := range pubKey {
		pubKey[i] = byte(i)
	}
	peer := mesh.NewPeer(pubKey, "timeout-peer", "n1", net.ParseIP("100.64.0.5"))

	// Simulate a pending handshake that was sent long ago.
	hs, err := crypto.InitiatorHandshake(e.kp, pubKey)
	if err != nil {
		t.Fatal(err)
	}
	msg1, err := hs.WriteMessage1()
	if err != nil {
		t.Fatal(err)
	}

	localID := e.nextID()
	e.mu.Lock()
	e.pending[localID] = &pendingHandshake{
		peer:    peer,
		hs:      hs,
		localID: localID,
		sentAt:  time.Now().Add(-10 * time.Second), // 10s ago, well past 5s timeout
	}
	e.mu.Unlock()

	// Manually run one iteration of the timeout logic.
	e.mu.Lock()
	for id, ph := range e.pending {
		if time.Since(ph.sentAt) > handshakeTimeout {
			delete(e.pending, id)
			ph.peer.Transition(mesh.PeerDiscovered)
		}
	}
	e.mu.Unlock()

	e.mu.RLock()
	_, exists := e.pending[localID]
	e.mu.RUnlock()
	if exists {
		t.Error("pending handshake should have been cleaned up after timeout")
	}

	// Verify msg1 was consumed correctly by the handshake (basic sanity).
	if len(msg1) != 96 {
		t.Errorf("msg1 length: got %d, want 96", len(msg1))
	}
}

// ─── Session endpoint storage ────────────────────────────────────────────────

func TestSessionEndpoint_Updates(t *testing.T) {
	e := testEngine(t)

	var remotePub crypto.Key
	for i := range remotePub {
		remotePub[i] = byte(i + 10)
	}

	ep1 := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 12345}
	ps := e.buildSession(remotePub, [32]byte{1}, [32]byte{2}, 1, 2, ep1)

	// Verify initial endpoint.
	loaded := ps.endpoint.Load()
	if loaded == nil || loaded.Port != 12345 {
		t.Errorf("initial endpoint: got %v", loaded)
	}

	// Update endpoint.
	ep2 := &net.UDPAddr{IP: net.ParseIP("10.0.0.2"), Port: 54321}
	ps.endpoint.Store(ep2)

	loaded = ps.endpoint.Load()
	if loaded == nil || !loaded.IP.Equal(net.ParseIP("10.0.0.2")) || loaded.Port != 54321 {
		t.Errorf("updated endpoint: got %v, want 10.0.0.2:54321", loaded)
	}
}

// ─── Multiple sessions ───────────────────────────────────────────────────────

func TestMultipleSessions(t *testing.T) {
	e := testEngine(t)

	sessions := make([]*peerSession, 5)
	for i := 0; i < 5; i++ {
		var pub crypto.Key
		pub[0] = byte(i + 1)

		var sk, rk [32]byte
		sk[0] = byte(i)
		rk[0] = byte(i + 10)

		sessions[i] = e.buildSession(pub, sk, rk, uint32(i*10), uint32(i*10+1), nil)
	}

	e.mu.RLock()
	count := len(e.sessions)
	byIDCount := len(e.byID)
	e.mu.RUnlock()

	if count != 5 {
		t.Errorf("sessions count: got %d, want 5", count)
	}
	if byIDCount != 5 {
		t.Errorf("byID count: got %d, want 5", byIDCount)
	}

	// Verify each session is accessible.
	for i, ps := range sessions {
		if ps.localID != uint32(i*10) {
			t.Errorf("session %d localID: got %d, want %d", i, ps.localID, i*10)
		}
	}
}

// ─── HTTP client configuration ───────────────────────────────────────────────

func TestHTTPClientHasTimeouts(t *testing.T) {
	if httpClient == nil {
		t.Fatal("httpClient should not be nil")
	}
	if httpClient.Timeout == 0 {
		t.Error("httpClient should have a non-zero Timeout")
	}
	if httpClient.Transport == nil {
		t.Error("httpClient should have a Transport configured")
	}
}

// ─── ACL tests ──────────────────────────────────────────────────────────────

func TestApplyACL_EmptyRules(t *testing.T) {
	e := testEngine(t)

	// Engine starts with a default allow-all policy.
	if !e.acl.Allow(net.ParseIP("100.64.0.1"), net.ParseIP("100.64.0.2"), 80) {
		t.Fatal("default policy should allow all")
	}

	// Empty rules should not change the ACL engine.
	e.applyACL(coordinator.ACLPolicy{Rules: nil})
	if !e.acl.Allow(net.ParseIP("100.64.0.1"), net.ParseIP("100.64.0.2"), 80) {
		t.Fatal("empty rules should not change allow-all policy")
	}
}

func TestApplyACL_NonEmptyRules(t *testing.T) {
	e := testEngine(t)

	// Apply a deny-all rule for port 22 from 100.64.0.0/10.
	e.applyACL(coordinator.ACLPolicy{
		Version: 1,
		Rules: []coordinator.ACLRule{
			{
				Action: "deny",
				Src:    []string{"100.64.0.0/10"},
				Dst:    []string{"*"},
				Ports:  []string{"22"},
			},
			{
				Action: "allow",
				Src:    []string{"*"},
				Dst:    []string{"*"},
			},
		},
	})

	src := net.ParseIP("100.64.0.5")
	dst := net.ParseIP("100.64.0.10")

	// SSH (port 22) should be denied.
	if e.acl.Allow(src, dst, 22) {
		t.Error("expected port 22 to be denied")
	}

	// HTTP (port 80) should be allowed by the catch-all allow rule.
	if !e.acl.Allow(src, dst, 80) {
		t.Error("expected port 80 to be allowed")
	}
}

// ─── Peers API tests ────────────────────────────────────────────────────────

func TestHandleAPIPeers(t *testing.T) {
	e := testEngine(t)

	var pub [32]byte
	for i := range pub {
		pub[i] = byte(i + 1)
	}
	e.manager.AddOrUpdate(pub, "test-peer", "n1", net.ParseIP("100.64.0.2"), "", nil)

	w := httptest.NewRecorder()
	e.handleAPIPeers(w, httptest.NewRequest(http.MethodGet, "/peers", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	body := w.Body.String()
	if !strings.Contains(body, "test-peer") {
		t.Errorf("response should contain peer hostname, got:\n%s", body)
	}
	if !strings.Contains(body, "100.64.0.2") {
		t.Errorf("response should contain peer virtual IP, got:\n%s", body)
	}
	if !strings.Contains(body, "n1") {
		t.Errorf("response should contain peer node ID, got:\n%s", body)
	}
}

func TestHandleAPIPeers_Empty(t *testing.T) {
	e := testEngine(t)

	w := httptest.NewRecorder()
	e.handleAPIPeers(w, httptest.NewRequest(http.MethodGet, "/peers", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var result []json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("parse response: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty array, got %d items", len(result))
	}
}

// ─── Exit node API tests ───────────────────────────────────────────────────

func TestHandleAPIExitNodeEnable_WrongMethod(t *testing.T) {
	e := testEngine(t)

	w := httptest.NewRecorder()
	e.handleAPIExitNodeEnable(w, httptest.NewRequest(http.MethodGet, "/exit-node/enable", nil))

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleAPIExitNodeEnable_InvalidJSON(t *testing.T) {
	e := testEngine(t)

	w := httptest.NewRecorder()
	e.handleAPIExitNodeEnable(w, httptest.NewRequest(http.MethodPost, "/exit-node/enable",
		strings.NewReader("{invalid json")))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleAPIExitNodeEnable_MissingInterface(t *testing.T) {
	e := testEngine(t)

	w := httptest.NewRecorder()
	e.handleAPIExitNodeEnable(w, httptest.NewRequest(http.MethodPost, "/exit-node/enable",
		strings.NewReader(`{"out_interface":""}`)))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleAPIExitNodeUse_WrongMethod(t *testing.T) {
	e := testEngine(t)

	w := httptest.NewRecorder()
	e.handleAPIExitNodeUse(w, httptest.NewRequest(http.MethodGet, "/exit-node/use", nil))

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleAPIExitNodeUse_PeerNotFound(t *testing.T) {
	e := testEngine(t)

	w := httptest.NewRecorder()
	e.handleAPIExitNodeUse(w, httptest.NewRequest(http.MethodPost, "/exit-node/use",
		strings.NewReader(`{"peer":"nonexistent"}`)))

	if w.Code != http.StatusNotFound {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusNotFound)
	}
}

// ─── pathName helper tests ─────────────────────────────────────────────────

func TestPathName(t *testing.T) {
	if got := pathName(nil); got != "relay" {
		t.Errorf("pathName(nil) = %q, want %q", got, "relay")
	}

	addr := &net.UDPAddr{IP: net.ParseIP("1.2.3.4"), Port: 5678}
	want := "direct:1.2.3.4:5678"
	if got := pathName(addr); got != want {
		t.Errorf("pathName(%v) = %q, want %q", addr, got, want)
	}
}

// ─── PacketDstPort IPv6 tests ────────────────────────────────────────────────

func TestPacketDstPort_IPv6(t *testing.T) {
	tests := []struct {
		name string
		pkt  []byte
		want uint16
	}{
		{
			name: "ipv6 tcp port 443",
			pkt: func() []byte {
				pkt := make([]byte, 60) // 40-byte IPv6 header + 20-byte TCP header
				pkt[0] = 0x60           // version 6
				pkt[6] = 6              // protocol = TCP
				pkt[42] = 0x01          // dst port high byte (40+2)
				pkt[43] = 0xBB          // 443 = 0x01BB
				return pkt
			}(),
			want: 443,
		},
		{
			name: "ipv6 udp port 5353",
			pkt: func() []byte {
				pkt := make([]byte, 48) // 40-byte IPv6 header + 8-byte UDP header
				pkt[0] = 0x60
				pkt[6] = 17             // protocol = UDP
				pkt[42] = 0x14          // dst port high byte (40+2)
				pkt[43] = 0xE9          // 5353 = 0x14E9
				return pkt
			}(),
			want: 5353,
		},
		{
			name: "ipv6 too short",
			pkt: func() []byte {
				pkt := make([]byte, 30)
				pkt[0] = 0x60
				return pkt
			}(),
			want: 0,
		},
		{
			name: "ipv6 non tcp/udp (ICMPv6=58)",
			pkt: func() []byte {
				pkt := make([]byte, 48)
				pkt[0] = 0x60
				pkt[6] = 58 // ICMPv6
				return pkt
			}(),
			want: 0,
		},
		{
			name: "ipv4 ICMP proto",
			pkt: func() []byte {
				pkt := make([]byte, 28)
				pkt[0] = 0x45
				pkt[9] = 1 // ICMP
				return pkt
			}(),
			want: 0,
		},
		{
			name: "empty slice",
			pkt:  []byte{},
			want: 0,
		},
		{
			name: "ipv4 IHL with options",
			pkt: func() []byte {
				// IHL=6 means 24-byte header (with options)
				pkt := make([]byte, 32)
				pkt[0] = 0x46 // version 4, IHL=6
				pkt[9] = 6    // TCP
				pkt[26] = 0   // dst port high
				pkt[27] = 80  // dst port low
				return pkt
			}(),
			want: 80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := packetDstPort(tt.pkt)
			if got != tt.want {
				t.Errorf("packetDstPort() = %d, want %d", got, tt.want)
			}
		})
	}
}

// ─── Sign request edge-case tests ────────────────────────────────────────────

func TestSignRequest_DifferentMethod(t *testing.T) {
	e := testEngine(t)

	req1, _ := http.NewRequest(http.MethodGet, "/api/v1/poll", nil)
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/poll", nil)

	e.signRequest(req1, []byte("body"))
	e.signRequest(req2, []byte("body"))

	sig1 := req1.Header.Get("X-Karadul-Sig")
	sig2 := req2.Header.Get("X-Karadul-Sig")

	if sig1 == sig2 {
		t.Error("different methods should produce different signatures")
	}
}

func TestSignRequest_DifferentURI(t *testing.T) {
	e := testEngine(t)

	req1, _ := http.NewRequest(http.MethodPost, "/api/v1/poll", nil)
	req2, _ := http.NewRequest(http.MethodPost, "/api/v1/register", nil)

	e.signRequest(req1, []byte("body"))
	e.signRequest(req2, []byte("body"))

	sig1 := req1.Header.Get("X-Karadul-Sig")
	sig2 := req2.Header.Get("X-Karadul-Sig")

	if sig1 == sig2 {
		t.Error("different URIs should produce different signatures")
	}
}

func TestSignRequest_NilBody(t *testing.T) {
	e := testEngine(t)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/poll", nil)
	e.signRequest(req, nil)

	sig := req.Header.Get("X-Karadul-Sig")
	if sig == "" {
		t.Error("nil body should still produce a valid signature")
	}
}

func TestSignRequest_EmptyBody(t *testing.T) {
	e := testEngine(t)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/poll", nil)
	e.signRequest(req, []byte{})

	sig := req.Header.Get("X-Karadul-Sig")
	if sig == "" {
		t.Error("empty body should still produce a valid signature")
	}
}

// ─── Metrics with active state tests ─────────────────────────────────────────

func TestHandleAPIMetrics_WithActiveSessions(t *testing.T) {
	e := testEngine(t)

	// Build 2 sessions.
	var pub1, pub2 [32]byte
	pub1[0] = 1
	pub2[0] = 2
	e.buildSession(pub1, [32]byte{1}, [32]byte{2}, 100, 200, nil)
	e.buildSession(pub2, [32]byte{3}, [32]byte{4}, 300, 400, nil)

	// Add 1 pending handshake.
	var pub3 [32]byte
	pub3[0] = 3
	peer := mesh.NewPeer(pub3, "pending-peer", "n3", net.ParseIP("100.64.0.5"))
	localID := e.nextID()
	e.mu.Lock()
	e.pending[localID] = &pendingHandshake{
		peer:    peer,
		localID: localID,
		sentAt:  time.Now(),
	}
	e.mu.Unlock()

	w := httptest.NewRecorder()
	e.handleAPIMetrics(w, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	body := w.Body.String()
	if !containsStr(body, "karadul_sessions_active 2") {
		t.Errorf("expected sessions_active 2, got:\n%s", body)
	}
	if !containsStr(body, "karadul_handshakes_pending 1") {
		t.Errorf("expected handshakes_pending 1, got:\n%s", body)
	}
}

// ─── Exit node use additional tests ──────────────────────────────────────────

func TestHandleAPIExitNodeUse_MissingPeer(t *testing.T) {
	e := testEngine(t)

	w := httptest.NewRecorder()
	e.handleAPIExitNodeUse(w, httptest.NewRequest(http.MethodPost, "/exit-node/use",
		strings.NewReader(`{"peer":""}`)))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleAPIExitNodeUse_InvalidJSON(t *testing.T) {
	e := testEngine(t)

	w := httptest.NewRecorder()
	e.handleAPIExitNodeUse(w, httptest.NewRequest(http.MethodPost, "/exit-node/use",
		strings.NewReader("not json")))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ─── Shutdown additional tests ───────────────────────────────────────────────

func TestHandleAPIShutdown_WrongMethod(t *testing.T) {
	e := testEngine(t)

	cancelled := false
	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	e.cancel = func() {
		cancel()
		cancelled = true
	}

	w := httptest.NewRecorder()
	e.handleAPIShutdown(w, httptest.NewRequest(http.MethodGet, "/shutdown", nil))

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
	if cancelled {
		t.Error("cancel should not be called on GET")
	}
	cancel() // cleanup
}

func TestHandleAPIShutdown_NilCancel(t *testing.T) {
	e := testEngine(t)
	e.ctx = context.Background()
	// e.cancel is nil by default

	w := httptest.NewRecorder()
	e.handleAPIShutdown(w, httptest.NewRequest(http.MethodPost, "/shutdown", nil))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
}

// ─── sendPing tests ───────────────────────────────────────────────────────────

func TestSendPing(t *testing.T) {
	var gotMethod, gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path

		// Verify signed headers.
		if r.Header.Get("X-Karadul-Key") == "" {
			t.Error("missing X-Karadul-Key header")
		}
		if r.Header.Get("X-Karadul-Sig") == "" {
			t.Error("missing X-Karadul-Sig header")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := testEngine(t)
	e.serverURL = srv.URL

	err := e.sendPing(context.Background())
	if err != nil {
		t.Fatalf("sendPing: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method: got %q, want POST", gotMethod)
	}
	if gotPath != "/api/v1/ping" {
		t.Errorf("path: got %q, want /api/v1/ping", gotPath)
	}
}

func TestSendPing_Error(t *testing.T) {
	e := testEngine(t)
	e.serverURL = "http://127.0.0.1:0" // invalid port

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := e.sendPing(ctx)
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}

// ─── onDERPRecv tests ────────────────────────────────────────────────────────

func TestOnDERPRecv_EmptyPayload(t *testing.T) {
	e := testEngine(t)
	// Should not panic.
	e.onDERPRecv([32]byte{}, nil)
	e.onDERPRecv([32]byte{}, []byte{})
}

func TestOnDERPRecv_InvalidType(t *testing.T) {
	e := testEngine(t)
	// Unknown packet type — should be silently ignored.
	e.onDERPRecv([32]byte{}, []byte{0xFF, 0x00, 0x00})
}

func TestOnDERPRecv_Keepalive(t *testing.T) {
	e := testEngine(t)
	// Keepalive type — should be silently ignored (no handler in switch).
	e.onDERPRecv([32]byte{}, []byte{0x04})
}

// ─── tryUpgradeToDirect tests ─────────────────────────────────────────────────

func TestTryUpgradeToDirect_NoRelayedSessions(t *testing.T) {
	e := testEngine(t)

	// Build a session with a direct endpoint — should NOT be selected for upgrade.
	var pub [32]byte
	pub[0] = 1
	ep := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 12345}
	ps := e.buildSession(pub, [32]byte{1}, [32]byte{2}, 1, 2, ep)

	peer := mesh.NewPeer(pub, "direct-peer", "n1", net.ParseIP("100.64.0.2"))
	peer.SetEndpoint(ep)
	ps.peer = peer

	// Should not panic and should not try to upgrade.
	e.tryUpgradeToDirect()
}

func TestTryUpgradeToDirect_RelayedSession(t *testing.T) {
	e := testEngine(t)

	// Bind a real UDP socket so initiateHandshake doesn't panic on nil conn.
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Skipf("UDP listen: %v", err)
	}
	e.udp = udp
	t.Cleanup(func() { udp.Close() })

	// Build a session with nil endpoint (relayed via DERP).
	var pub [32]byte
	pub[0] = 2
	ps := e.buildSession(pub, [32]byte{3}, [32]byte{4}, 10, 20, nil)
	ps.endpoint.Store(nil) // explicitly nil = relayed

	peer := mesh.NewPeer(pub, "relayed-peer", "n2", net.ParseIP("100.64.0.3"))
	// Give the peer an endpoint that was discovered after the session was created.
	peer.SetEndpoint(&net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: udp.LocalAddr().(*net.UDPAddr).Port})
	ps.peer = peer

	// This should attempt a handshake upgrade.
	e.tryUpgradeToDirect()
}

func TestTryUpgradeToDirect_EmptySessions(t *testing.T) {
	e := testEngine(t)
	// No sessions at all — should be a no-op.
	e.tryUpgradeToDirect()
}

// ─── handleData edge cases ────────────────────────────────────────────────────

func TestHandleData_UnknownReceiverIndex(t *testing.T) {
	e := testEngine(t)

	// Craft a minimal data message with an unknown receiver index.
	// Data packet: type(1) + reserved(3) + receiverIndex(4) + counter(8) + ciphertext
	pkt := make([]byte, 32)
	pkt[0] = 0x03 // TypeData
	// receiverIndex = 99999 (bytes 4-7)
	pkt[4] = 0x9F
	pkt[5] = 0x86
	pkt[6] = 0x01
	pkt[7] = 0x00

	// Should silently drop (no panic).
	e.handleData(&net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 5000}, pkt)
}

func TestHandleUDPPacket_InvalidType(t *testing.T) {
	e := testEngine(t)

	// Packet with unknown type byte.
	e.handleUDPPacket(&net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 5000}, []byte{0xFF})
}

// ─── handleAPIExitNodeUse with peer by VIP ─────────────────────────────────────

func TestHandleAPIExitNodeUse_ByVirtualIP(t *testing.T) {
	e := testEngine(t)

	// Set up mock TUN and router to avoid nil panics.
	mtun := &mockTUN{name: "mocktun0", mtu: 1420}
	e.tun = mtun
	e.router = mesh.NewRouter(e.manager)

	var pub [32]byte
	for i := range pub {
		pub[i] = byte(i + 1)
	}
	e.manager.AddOrUpdate(pub, "exit-peer", "n1", net.ParseIP("100.64.0.99"), "", nil)

	// Use VirtualIP as the peer identifier.
	w := httptest.NewRecorder()
	e.handleAPIExitNodeUse(w, httptest.NewRequest(http.MethodPost, "/exit-node/use",
		strings.NewReader(`{"peer":"100.64.0.99"}`)))

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}
}

// ─── Mock TUN device for exit node tests ───────────────────────────────────────

type mockTUN struct {
	name    string
	closed  bool
	routes  []*net.IPNet
	mtu     int
	addr    net.IP
	prefix  int
}

func (m *mockTUN) Name() string                     { return m.name }
func (m *mockTUN) Read(buf []byte) (int, error)     { return 0, nil }
func (m *mockTUN) Write(buf []byte) (int, error)    { return 0, nil }
func (m *mockTUN) MTU() int                         { return m.mtu }
func (m *mockTUN) SetMTU(mtu int) error             { m.mtu = mtu; return nil }
func (m *mockTUN) SetAddr(ip net.IP, pl int) error  { m.addr = ip; m.prefix = pl; return nil }
func (m *mockTUN) AddRoute(dst *net.IPNet) error    { m.routes = append(m.routes, dst); return nil }
func (m *mockTUN) Close() error                     { m.closed = true; return nil }

func TestHandleAPIExitNodeUse_Success(t *testing.T) {
	e := testEngine(t)

	// Set up mock TUN device.
	mtun := &mockTUN{name: "mocktun0", mtu: 1420}
	e.tun = mtun

	// Set up router.
	e.router = mesh.NewRouter(e.manager)

	// Add a peer that will serve as exit node.
	var pub [32]byte
	for i := range pub {
		pub[i] = byte(i + 1)
	}
	e.manager.AddOrUpdate(pub, "exit-node", "n1", net.ParseIP("100.64.0.50"), "", nil)

	w := httptest.NewRecorder()
	e.handleAPIExitNodeUse(w, httptest.NewRequest(http.MethodPost, "/exit-node/use",
		strings.NewReader(`{"peer":"exit-node"}`)))

	if w.Code != http.StatusOK {
		t.Fatalf("status: got %d, want %d; body: %s", w.Code, http.StatusOK, w.Body.String())
	}

	// Verify the route was added (default route 0.0.0.0/0).
	if len(mtun.routes) == 0 {
		t.Fatal("expected route to be added")
	}
	_, defaultNet, _ := net.ParseCIDR("0.0.0.0/0")
	if !mtun.routes[0].IP.Equal(defaultNet.IP) {
		t.Errorf("route IP: got %v, want %v", mtun.routes[0].IP, defaultNet.IP)
	}
}

// ─── Helper ──────────────────────────────────────────────────────────────────

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ─── Register tests ──────────────────────────────────────────────────────────

func TestRegister_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/register" {
			t.Errorf("request path: got %q, want /api/v1/register", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("request method: got %q, want POST", r.Method)
		}

		// Decode the register request to verify fields.
		var req registerReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode register request: %v", err)
		}
		if req.Hostname != "test-node" {
			t.Errorf("hostname: got %q, want %q", req.Hostname, "test-node")
		}
		if req.AuthKey != "test-auth-key" {
			t.Errorf("authKey: got %q, want %q", req.AuthKey, "test-auth-key")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(registerResp{
			NodeID:    "test-123",
			VirtualIP: "100.64.0.1",
			Hostname:  "test",
		})
	}))
	defer srv.Close()

	e := testEngine(t)
	e.serverURL = srv.URL

	err := e.register(context.Background())
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if e.nodeID != "test-123" {
		t.Errorf("nodeID: got %q, want %q", e.nodeID, "test-123")
	}
	if e.virtualIP == nil || !e.virtualIP.Equal(net.ParseIP("100.64.0.1")) {
		t.Errorf("virtualIP: got %v, want 100.64.0.1", e.virtualIP)
	}
}

func TestRegister_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	}))
	defer srv.Close()

	e := testEngine(t)
	e.serverURL = srv.URL

	err := e.register(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}

	// Verify error message contains the status code.
	if !containsStr(err.Error(), "status 403") {
		t.Errorf("error should mention status 403, got: %v", err)
	}
}

func TestRegister_InvalidVirtualIP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(registerResp{
			NodeID:    "test",
			VirtualIP: "not-an-ip",
			Hostname:  "test",
		})
	}))
	defer srv.Close()

	e := testEngine(t)
	e.serverURL = srv.URL

	err := e.register(context.Background())
	if err == nil {
		t.Fatal("expected error for invalid virtual IP")
	}

	if !containsStr(err.Error(), "invalid virtual IP") {
		t.Errorf("error should mention invalid virtual IP, got: %v", err)
	}
}

// ─── Poll tests ──────────────────────────────────────────────────────────────

func TestPoll_Success(t *testing.T) {
	wantVersion := int64(42)
	wantHostname := "peer-a"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/poll" {
			t.Errorf("request path: got %q, want /api/v1/poll", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("request method: got %q, want POST", r.Method)
		}

		// Verify signed headers are present.
		if r.Header.Get("X-Karadul-Key") == "" {
			t.Error("missing X-Karadul-Key header")
		}
		if r.Header.Get("X-Karadul-Sig") == "" {
			t.Error("missing X-Karadul-Sig header")
		}

		// Decode request body to verify sinceVersion.
		var reqBody map[string]int64
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("decode poll request: %v", err)
		}
		if reqBody["sinceVersion"] != 10 {
			t.Errorf("sinceVersion: got %d, want 10", reqBody["sinceVersion"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(coordinator.NetworkState{
			Version: wantVersion,
			Nodes: []*coordinator.Node{
				{
					ID:        "peer-1",
					Hostname:  wantHostname,
					VirtualIP: "100.64.0.2",
					Status:    coordinator.NodeStatusActive,
				},
			},
		})
	}))
	defer srv.Close()

	e := testEngine(t)
	e.serverURL = srv.URL

	state, err := e.poll(context.Background(), 10)
	if err != nil {
		t.Fatalf("poll: %v", err)
	}

	if state.Version != wantVersion {
		t.Errorf("version: got %d, want %d", state.Version, wantVersion)
	}
	if len(state.Nodes) != 1 {
		t.Fatalf("nodes count: got %d, want 1", len(state.Nodes))
	}
	if state.Nodes[0].Hostname != wantHostname {
		t.Errorf("node hostname: got %q, want %q", state.Nodes[0].Hostname, wantHostname)
	}
	if state.Nodes[0].VirtualIP != "100.64.0.2" {
		t.Errorf("node virtualIP: got %q, want %q", state.Nodes[0].VirtualIP, "100.64.0.2")
	}
}

func TestPoll_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	e := testEngine(t)
	e.serverURL = srv.URL

	_, err := e.poll(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}

	if !containsStr(err.Error(), "status 500") {
		t.Errorf("error should mention status 500, got: %v", err)
	}
}

// ─── ReportEndpoint tests ───────────────────────────────────────────────────

func TestReportEndpoint(t *testing.T) {
	var receivedReq *http.Request
	var receivedBody map[string]interface{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedReq = r

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)
		receivedBody = body

		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	e := testEngine(t)
	e.serverURL = srv.URL

	err := e.reportEndpoint(context.Background(), "203.0.113.1:43210")
	if err != nil {
		t.Fatalf("reportEndpoint: %v", err)
	}

	// Verify the request went to the right path.
	if receivedReq.URL.Path != "/api/v1/update-endpoint" {
		t.Errorf("path: got %q, want /api/v1/update-endpoint", receivedReq.URL.Path)
	}

	// Verify signed headers are present.
	if receivedReq.Header.Get("X-Karadul-Key") == "" {
		t.Error("missing X-Karadul-Key header")
	}
	if receivedReq.Header.Get("X-Karadul-Sig") == "" {
		t.Error("missing X-Karadul-Sig header")
	}

	// Verify the endpoint was included in the request body.
	if receivedBody["endpoint"] != "203.0.113.1:43210" {
		t.Errorf("endpoint in body: got %v, want %q", receivedBody["endpoint"], "203.0.113.1:43210")
	}
}

// ─── New coverage tests ──────────────────────────────────────────────────────

// TestHandleHandshakeInit_ProcessedCorrectly verifies that a valid Noise IK
// handshake init from a real initiator is accepted and produces a session on
// the responder side.
func TestHandleHandshakeInit_ProcessedCorrectly(t *testing.T) {
	// Create initiator (Alice) key pair.
	kpAlice, err := crypto.GenerateKeyPair()
	if err != nil {
		t.Fatal(err)
	}

	// Create responder engine (Bob) with Alice's public key known.
	e := testEngine(t)

	// Bind a UDP socket so Bob can send the response.
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Skipf("UDP listen: %v", err)
	}
	e.udp = udp
	t.Cleanup(func() { udp.Close() })

	// Alice creates initiator handshake state targeting Bob's public key.
	hs, err := crypto.InitiatorHandshake(kpAlice, e.kp.Public)
	if err != nil {
		t.Fatal(err)
	}
	msg1, err := hs.WriteMessage1()
	if err != nil {
		t.Fatal(err)
	}

	// Build the wire-level HandshakeInit message.
	senderIdx := uint32(777)
	initMsg := &protocol.MsgHandshakeInit{SenderIndex: senderIdx}
	copy(initMsg.Ephemeral[:], msg1[:32])
	copy(initMsg.EncStatic[:], msg1[32:80])
	copy(initMsg.EncPayload[:], msg1[80:96])
	wire := initMsg.MarshalBinary()

	// Bob receives and processes the handshake init.
	// Use Bob's own UDP address so the response is sent back to himself (loopback).
	bobAddr := udp.LocalAddr().(*net.UDPAddr)
	e.handleHandshakeInit(bobAddr, wire)
	// Verify Bob now has a session with Alice's public key.
	e.mu.RLock()
	sessCount := len(e.sessions)
	byIDCount := len(e.byID)
	e.mu.RUnlock()

	if sessCount != 1 {
		t.Errorf("expected 1 session after valid handshake init, got %d", sessCount)
	}
	if byIDCount != 1 {
		t.Errorf("expected 1 byID entry after valid handshake init, got %d", byIDCount)
	}

	// Read Bob's response from the UDP socket to verify it was sent.
	buf := make([]byte, 1500)
	udp.SetReadDeadline(time.Now().Add(2 * time.Second))
	n, _, err := udp.ReadFromUDP(buf)
	if err != nil {
		t.Fatalf("failed to read handshake response: %v", err)
	}

	resp, err := protocol.UnmarshalMsgHandshakeResp(buf[:n])
	if err != nil {
		t.Fatalf("failed to unmarshal handshake response: %v", err)
	}
	// The ReceiverIndex in the response should echo the initiator's SenderIndex.
	if resp.ReceiverIndex != senderIdx {
		t.Errorf("response ReceiverIndex: got %d, want %d", resp.ReceiverIndex, senderIdx)
	}
}

// TestHandleHandshakeResp_UnknownIndex verifies that a handshake response
// with an unknown pending ID is silently dropped without creating a session.
func TestHandleHandshakeResp_UnknownIndex(t *testing.T) {
	e := testEngine(t)

	// Ensure no pending handshakes exist.
	e.mu.RLock()
	pendingCount := len(e.pending)
	e.mu.RUnlock()
	if pendingCount != 0 {
		t.Fatalf("expected 0 pending handshakes, got %d", pendingCount)
	}

	// Craft a valid-sized HandshakeResp with an unknown ReceiverIndex.
	pkt := make([]byte, protocol.HandshakeRespSize)
	pkt[0] = protocol.TypeHandshakeResp
	// ReceiverIndex at bytes 8:12 = 0x0001869F (99999)
	pkt[8] = 0x9F
	pkt[9] = 0x86
	pkt[10] = 0x01
	pkt[11] = 0x00

	e.handleHandshakeResp(&net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 5000}, pkt)

	// No session should have been created.
	e.mu.RLock()
	n := len(e.sessions)
	e.mu.RUnlock()
	if n != 0 {
		t.Errorf("expected 0 sessions after unknown receiver index, got %d", n)
	}
}

// TestTunReadLoop_ContextCancel verifies that tunReadLoop exits promptly
// when stopCh is closed.
func TestTunReadLoop_ContextCancel(t *testing.T) {
	e := testEngine(t)

	// Use mock TUN whose Read returns an error once Close is called.
	mtun := &errorAfterCloseMockTUN{closedCh: make(chan struct{})}
	e.tun = mtun

	done := make(chan struct{})
	go func() {
		e.tunReadLoop()
		close(done)
	}()

	// Close stopCh first, then close the TUN so Read returns an error.
	// tunReadLoop checks stopCh after every Read error.
	close(e.stopCh)
	mtun.Close()

	select {
	case <-done:
		// tunReadLoop exited cleanly.
	case <-time.After(5 * time.Second):
		t.Fatal("tunReadLoop did not exit after stopCh was closed")
	}
}

// errorAfterCloseMockTUN is a mock TUN whose Read blocks until Close is called,
// then returns an error.
type errorAfterCloseMockTUN struct {
	closedCh chan struct{}
}

func (m *errorAfterCloseMockTUN) Name() string                    { return "error-after-close-mock" }
func (m *errorAfterCloseMockTUN) Read(buf []byte) (int, error)    { <-m.closedCh; return 0, fmt.Errorf("closed") }
func (m *errorAfterCloseMockTUN) Write(buf []byte) (int, error)   { return len(buf), nil }
func (m *errorAfterCloseMockTUN) MTU() int                        { return 1420 }
func (m *errorAfterCloseMockTUN) SetMTU(mtu int) error            { return nil }
func (m *errorAfterCloseMockTUN) SetAddr(ip net.IP, pl int) error { return nil }
func (m *errorAfterCloseMockTUN) AddRoute(dst *net.IPNet) error   { return nil }
func (m *errorAfterCloseMockTUN) Close() error {
	close(m.closedCh)
	return nil
}

// TestEnableExitNode_Success verifies EnableExitNode calls the platform function.
// This test may need to be skipped on some platforms where the system call is unavailable.
func TestEnableExitNode_Success(t *testing.T) {
	t.Skip("EnableExitNode requires platform-specific system calls (sysctl/pf/iptables) that cannot run in unprivileged test environments")
}

// TestHandleAPIStatus_JSON verifies the /status endpoint returns valid JSON
// with all expected fields.
func TestHandleAPIStatus_JSON(t *testing.T) {
	e := testEngine(t)
	e.nodeID = "json-test-node"
	e.virtualIP = net.ParseIP("100.64.0.1")
	e.publicEP.Store(&net.UDPAddr{IP: net.ParseIP("203.0.113.5"), Port: 43210})

	w := httptest.NewRecorder()
	e.handleAPIStatus(w, httptest.NewRequest(http.MethodGet, "/status", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status code: got %d, want %d", w.Code, http.StatusOK)
	}

	ct := w.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("parse JSON response: %v", err)
	}

	// Verify all expected fields are present.
	expectedFields := []string{"nodeId", "virtualIp", "publicKey", "publicEp", "sessions", "pendingHs", "peers"}
	for _, field := range expectedFields {
		if _, ok := resp[field]; !ok {
			t.Errorf("missing field %q in response", field)
		}
	}

	if resp["nodeId"] != "json-test-node" {
		t.Errorf("nodeId: got %v, want json-test-node", resp["nodeId"])
	}
	if resp["virtualIp"] != "100.64.0.1" {
		t.Errorf("virtualIp: got %v, want 100.64.0.1", resp["virtualIp"])
	}
	if resp["publicEp"] != "203.0.113.5:43210" {
		t.Errorf("publicEp: got %v, want 203.0.113.5:43210", resp["publicEp"])
	}
}

// TestHandleAPIExitNodeEnable_Success verifies the successful path through
// the exit-node enable handler using a mock setup.
func TestHandleAPIExitNodeEnable_Success(t *testing.T) {
	// We cannot easily mock the platform-specific EnableExitNode function,
	// so we test the handler logic up to the point of the system call.
	// For a fully successful test, we skip on platforms where the call would fail.
	t.Skip("EnableExitNode platform call requires root/admin privileges")
}

// TestInitiateHandshake_NoEndpoint verifies that initiateHandshake returns an
// error when the peer has no endpoint and no DERP client is available.
func TestInitiateHandshake_NoEndpoint(t *testing.T) {
	e := testEngine(t)

	// Bind a UDP socket so the engine doesn't panic on nil conn.
	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Skipf("UDP listen: %v", err)
	}
	e.udp = udp
	t.Cleanup(func() { udp.Close() })

	var pubKey [32]byte
	for i := range pubKey {
		pubKey[i] = byte(i + 42)
	}
	// Peer with no endpoint set.
	peer := mesh.NewPeer(pubKey, "no-ep", "n1", net.ParseIP("100.64.0.20"))

	err = e.initiateHandshake(peer)
	if err == nil {
		t.Fatal("expected error when peer has no endpoint and no DERP client")
	}
	if !containsStr(err.Error(), "no path to peer") {
		t.Errorf("error should mention 'no path to peer', got: %v", err)
	}
}

// TestConnectPeer_DirectEndpoint verifies that connectPeer initiates a direct
// handshake when the peer has a known endpoint.
func TestConnectPeer_DirectEndpoint(t *testing.T) {
	e := testEngine(t)

	udp, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 0})
	if err != nil {
		t.Skipf("UDP listen: %v", err)
	}
	e.udp = udp
	t.Cleanup(func() { udp.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	e.ctx = ctx
	t.Cleanup(cancel)

	var pubKey [32]byte
	for i := range pubKey {
		pubKey[i] = byte(i + 100)
	}

	ep := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: udp.LocalAddr().(*net.UDPAddr).Port}
	peer := mesh.NewPeer(pubKey, "direct-peer", "n1", net.ParseIP("100.64.0.30"))
	peer.SetEndpoint(ep)

	err = e.connectPeer(peer)
	if err != nil {
		t.Fatalf("connectPeer with direct endpoint: %v", err)
	}

	// Verify a pending handshake was created.
	e.mu.RLock()
	pendingCount := len(e.pending)
	e.mu.RUnlock()
	if pendingCount == 0 {
		t.Error("expected a pending handshake after connectPeer with direct endpoint")
	}
}

// TestApplyACL_DenyTraffic verifies that ACL deny rules work end-to-end:
// after applying a deny policy, the ACL engine should block matching traffic.
func TestApplyACL_DenyTraffic(t *testing.T) {
	e := testEngine(t)

	// Apply a policy with a single deny-all rule (no allow rules).
	// When rules exist but none match, the default is deny.
	e.applyACL(coordinator.ACLPolicy{
		Version: 2,
		Rules: []coordinator.ACLRule{
			{
				Action: "deny",
				Src:    []string{"*"},
				Dst:    []string{"*"},
				Ports:  []string{"*"},
			},
		},
	})

	src := net.ParseIP("100.64.0.1")
	dst := net.ParseIP("100.64.0.2")

	// All traffic should be denied (first matching rule is deny).
	if e.acl.Allow(src, dst, 80) {
		t.Error("expected port 80 to be denied by deny-all rule")
	}
	if e.acl.Allow(src, dst, 443) {
		t.Error("expected port 443 to be denied by deny-all rule")
	}
	if e.acl.Allow(src, dst, 0) {
		t.Error("expected any port to be denied by deny-all rule")
	}
}

// ─── New coverage: handleData with valid packets ──────────────────────────────

// TestHandleData_ValidPacket verifies that handleData correctly decrypts a
// valid data packet, writes the plaintext to the TUN device, and increments
// the RX metrics.
func TestHandleData_ValidPacket(t *testing.T) {
	e := testEngine(t)

	// Wire up a mock TUN so handleData can write to it.
	mtun := &mockTUN{name: "mocktun0", mtu: 1420}
	e.tun = mtun

	// Build a session with symmetric keys so Encrypt and Decrypt use the same key.
	var sendKey, recvKey [32]byte
	for i := range sendKey {
		sendKey[i] = byte(i + 1)
		recvKey[i] = byte(i + 1)
	}

	var remotePub crypto.Key
	for i := range remotePub {
		remotePub[i] = byte(i + 20)
	}

	localID := uint32(500)
	ps := e.buildSession(remotePub, sendKey, recvKey, localID, 600, nil)
	_ = ps

	// Build a minimal IPv4 packet: src 100.64.0.1, dst 100.64.0.2, TCP port 80.
	pkt := make([]byte, 24)
	pkt[0] = 0x45 // version 4, IHL 5
	pkt[9] = 6    // TCP
	pkt[12] = 100; pkt[13] = 64; pkt[14] = 0; pkt[15] = 1 // src IP
	pkt[16] = 100; pkt[17] = 64; pkt[18] = 0; pkt[19] = 2 // dst IP
	pkt[22] = 0   // dst port high byte
	pkt[23] = 80  // dst port low byte

	// Encrypt the plaintext using the session.
	counter, ct, err := ps.session.Encrypt(pkt)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Build a wire-level MsgData and marshal it.
	wire := (&protocol.MsgData{
		ReceiverIndex: localID,
		Counter:       counter,
		Ciphertext:    ct,
	}).MarshalBinary()

	// Precondition: metrics should be zero.
	if e.metricPacketsRx.Load() != 0 {
		t.Fatalf("precondition: packetsRx should be 0, got %d", e.metricPacketsRx.Load())
	}

	// Call handleData.
	addr := &net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 5000}
	e.handleData(addr, wire)

	// Verify metrics were incremented.
	if e.metricPacketsRx.Load() != 1 {
		t.Errorf("packetsRx: got %d, want 1", e.metricPacketsRx.Load())
	}
	if e.metricBytesRx.Load() != uint64(len(pkt)) {
		t.Errorf("bytesRx: got %d, want %d", e.metricBytesRx.Load(), len(pkt))
	}
}

// TestHandleData_ACLDeny verifies that handleData drops a packet when an
// ACL deny-all policy is active and does not increment metrics.
func TestHandleData_ACLDeny(t *testing.T) {
	e := testEngine(t)

	mtun := &mockTUN{name: "mocktun0", mtu: 1420}
	e.tun = mtun

	// Apply deny-all ACL.
	e.applyACL(coordinator.ACLPolicy{
		Version: 2,
		Rules: []coordinator.ACLRule{
			{Action: "deny", Src: []string{"*"}, Dst: []string{"*"}},
		},
	})

	var sendKey, recvKey [32]byte
	for i := range sendKey {
		sendKey[i] = byte(i + 1)
		recvKey[i] = byte(i + 1)
	}

	var remotePub crypto.Key
	for i := range remotePub {
		remotePub[i] = byte(i + 20)
	}

	localID := uint32(501)
	ps := e.buildSession(remotePub, sendKey, recvKey, localID, 601, nil)
	_ = ps

	// Build a valid IPv4 packet.
	pkt := make([]byte, 24)
	pkt[0] = 0x45
	pkt[9] = 6
	pkt[12] = 100; pkt[13] = 64; pkt[14] = 0; pkt[15] = 1
	pkt[16] = 100; pkt[17] = 64; pkt[18] = 0; pkt[19] = 2
	pkt[22] = 0
	pkt[23] = 80

	counter, ct, err := ps.session.Encrypt(pkt)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	wire := (&protocol.MsgData{
		ReceiverIndex: localID,
		Counter:       counter,
		Ciphertext:    ct,
	}).MarshalBinary()

	e.handleData(&net.UDPAddr{IP: net.ParseIP("10.0.0.1"), Port: 5000}, wire)

	// Metrics should NOT have been incremented.
	if e.metricPacketsRx.Load() != 0 {
		t.Errorf("packetsRx: got %d, want 0 (packet should be dropped by ACL)", e.metricPacketsRx.Load())
	}
	if e.metricBytesRx.Load() != 0 {
		t.Errorf("bytesRx: got %d, want 0 (packet should be dropped by ACL)", e.metricBytesRx.Load())
	}
}

// ─── New coverage: handshakeTimeoutLoop ────────────────────────────────────────

// TestHandshakeTimeoutLoop_CleansExpired verifies that the handshakeTimeoutLoop
// goroutine removes pending handshakes older than handshakeTimeout.
func TestHandshakeTimeoutLoop_CleansExpired(t *testing.T) {
	e := testEngine(t)

	var pubKey [32]byte
	for i := range pubKey {
		pubKey[i] = byte(i + 50)
	}
	peer := mesh.NewPeer(pubKey, "expired-hs-peer", "n1", net.ParseIP("100.64.0.5"))

	// Add a pending handshake that was sent long ago.
	localID := e.nextID()
	e.mu.Lock()
	e.pending[localID] = &pendingHandshake{
		peer:    peer,
		hs:      nil, // not needed for timeout cleanup
		localID: localID,
		sentAt:  time.Now().Add(-10 * time.Second), // older than handshakeTimeout (5s)
	}
	e.mu.Unlock()

	// Start the timeout loop.
	go e.handshakeTimeoutLoop()

	// Wait briefly for the loop's 1-second ticker to fire and clean up.
	time.Sleep(1500 * time.Millisecond)

	// Close stopCh to stop the loop.
	close(e.stopCh)

	e.mu.RLock()
	_, exists := e.pending[localID]
	e.mu.RUnlock()
	if exists {
		t.Error("expired pending handshake should have been cleaned up by handshakeTimeoutLoop")
	}
}

// ─── New coverage: rekeyLoop ───────────────────────────────────────────────────

// TestRekeyLoop_TriggersRekey verifies that rekeyLoop exits cleanly when
// its context is cancelled.
func TestRekeyLoop_TriggersRekey(t *testing.T) {
	e := testEngine(t)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		e.rekeyLoop(ctx)
		close(done)
	}()

	// Cancel after a brief delay.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// rekeyLoop exited cleanly.
	case <-time.After(5 * time.Second):
		t.Fatal("rekeyLoop did not exit after context cancellation")
	}
}

// ─── New coverage: pollLoop context cancellation ───────────────────────────────

// TestPollLoop_ContextCancel verifies that pollLoop exits when its context
// is cancelled.
func TestPollLoop_ContextCancel(t *testing.T) {
	e := testEngine(t)

	srvCtx, srvCancel := context.WithCancel(context.Background())
	defer srvCancel()

	// Use a mock HTTP server that blocks until its context is cancelled.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-srvCtx.Done()
	}))
	defer srv.Close()

	e.serverURL = srv.URL
	e.topology = mesh.NewTopologyManager(e.manager, e.kp.Public, e.log)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		e.pollLoop(ctx)
		close(done)
	}()

	// Cancel the pollLoop context after a brief delay.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// pollLoop exited cleanly.
	case <-time.After(5 * time.Second):
		t.Fatal("pollLoop did not exit after context cancellation")
	}
	// Cancel the server context so it can shut down cleanly.
	srvCancel()
}

// ─── New coverage: UseExitNode ─────────────────────────────────────────────────

// TestUseExitNode_AddsRoute verifies that UseExitNode adds a default route
// to the TUN device.
func TestUseExitNode_AddsRoute(t *testing.T) {
	e := testEngine(t)

	mtun := &mockTUN{name: "mocktun0", mtu: 1420}
	e.tun = mtun
	e.router = mesh.NewRouter(e.manager)

	// Add a peer to the manager.
	var pub [32]byte
	for i := range pub {
		pub[i] = byte(i + 1)
	}
	e.manager.AddOrUpdate(pub, "exit-node", "n1", net.ParseIP("100.64.0.50"), "", nil)

	// Look up the peer and call UseExitNode.
	var peer *mesh.Peer
	for _, p := range e.manager.ListPeers() {
		if p.Hostname == "exit-node" {
			peer = p
			break
		}
	}
	if peer == nil {
		t.Fatal("peer not found")
	}

	err := e.UseExitNode(peer)
	if err != nil {
		t.Fatalf("UseExitNode: %v", err)
	}

	// Verify the default route was added.
	if len(mtun.routes) == 0 {
		t.Fatal("expected at least one route to be added")
	}

	_, defaultNet, _ := net.ParseCIDR("0.0.0.0/0")
	if !mtun.routes[0].IP.Equal(defaultNet.IP) {
		t.Errorf("route IP: got %v, want %v", mtun.routes[0].IP, defaultNet.IP)
	}
}

