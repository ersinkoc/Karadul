package coordinator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// ─── 1. TestBuildPeersFromNodes ────────────────────────────────────────────────

func TestBuildPeersFromNodes(t *testing.T) {
	now := time.Now()
	oldTime := now.Add(-10 * time.Minute) // older than 5-minute threshold

	activeRecent := &Node{
		ID: "active-recent", PublicKey: "pk1", Hostname: "ar",
		VirtualIP: "100.64.0.1", Status: NodeStatusActive,
		Endpoint: "1.1.1.1:5000", LastSeen: now, RegisteredAt: now,
	}
	activeRecentNoEP := &Node{
		ID: "active-recent-noep", PublicKey: "pk2", Hostname: "arn",
		VirtualIP: "100.64.0.2", Status: NodeStatusActive,
		Endpoint: "", LastSeen: now, RegisteredAt: now,
	}
	activeOld := &Node{
		ID: "active-old", PublicKey: "pk3", Hostname: "ao",
		VirtualIP: "100.64.0.3", Status: NodeStatusActive,
		Endpoint: "3.3.3.3:5000", LastSeen: oldTime, RegisteredAt: oldTime,
	}
	activeOldNoEP := &Node{
		ID: "active-old-noep", PublicKey: "pk4", Hostname: "aon",
		VirtualIP: "100.64.0.4", Status: NodeStatusActive,
		Endpoint: "", LastSeen: oldTime, RegisteredAt: oldTime,
	}
	inactiveNode := &Node{
		ID: "inactive", PublicKey: "pk5", Hostname: "in",
		VirtualIP: "100.64.0.5", Status: NodeStatusDisabled,
		Endpoint: "5.5.5.5:5000", LastSeen: now, RegisteredAt: now,
	}

	tests := []struct {
		name      string
		nodes     []*Node
		wantCount int
		wantState map[string]string // node ID -> expected state
	}{
		{
			name:      "empty_nodes",
			nodes:     nil,
			wantCount: 0,
			wantState: nil,
		},
		{
			name:      "all_inactive",
			nodes:     []*Node{inactiveNode},
			wantCount: 0,
			wantState: nil,
		},
		{
			name:      "active_recent_with_endpoint",
			nodes:     []*Node{activeRecent},
			wantCount: 1,
			wantState: map[string]string{"active-recent": "Direct"},
		},
		{
			name:      "active_recent_no_endpoint",
			nodes:     []*Node{activeRecentNoEP},
			wantCount: 1,
			wantState: map[string]string{"active-recent-noep": "Relayed"},
		},
		{
			name:      "active_old_with_endpoint",
			nodes:     []*Node{activeOld},
			wantCount: 1,
			wantState: map[string]string{"active-old": "Discovered"},
		},
		{
			name:      "active_old_no_endpoint",
			nodes:     []*Node{activeOldNoEP},
			wantCount: 1,
			wantState: map[string]string{"active-old-noep": "Idle"},
		},
		{
			name: "mixed",
			nodes: []*Node{activeRecent, activeRecentNoEP, activeOld, activeOldNoEP, inactiveNode},
			wantCount: 4,
			wantState: map[string]string{
				"active-recent":      "Direct",
				"active-recent-noep": "Relayed",
				"active-old":         "Discovered",
				"active-old-noep":    "Idle",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			peers := buildPeersFromNodes(tt.nodes)
			if len(peers) != tt.wantCount {
				t.Fatalf("expected %d peers, got %d", tt.wantCount, len(peers))
			}
			for _, p := range peers {
				if want, ok := tt.wantState[p.ID]; ok {
					if p.State != want {
						t.Errorf("peer %q: state = %q, want %q", p.ID, p.State, want)
					}
				}
			}
		})
	}
}

// ─── 2. TestBuildTopologyFromNodes ─────────────────────────────────────────────

func TestBuildTopologyFromNodes(t *testing.T) {
	now := time.Now()
	oldTime := now.Add(-10 * time.Minute)

	tests := []struct {
		name            string
		nodes           []*Node
		wantConnections int
		wantConnType    string // expected connection type (for single-connection cases)
	}{
		{
			name:            "empty_nodes",
			nodes:           nil,
			wantConnections: 0,
		},
		{
			name: "one_active_node",
			nodes: []*Node{{
				ID: "n1", Status: NodeStatusActive, Endpoint: "1.1.1.1:5000",
				LastSeen: now, RegisteredAt: now,
			}},
			wantConnections: 0,
		},
		{
			name: "two_active_recent_with_endpoints",
			nodes: []*Node{
				{ID: "a", Status: NodeStatusActive, Endpoint: "1.1.1.1:5000", LastSeen: now, RegisteredAt: now},
				{ID: "b", Status: NodeStatusActive, Endpoint: "2.2.2.2:5000", LastSeen: now, RegisteredAt: now},
			},
			wantConnections: 1,
			wantConnType:    "direct",
		},
		{
			name: "two_active_recent_one_without_endpoint",
			nodes: []*Node{
				{ID: "a", Status: NodeStatusActive, Endpoint: "1.1.1.1:5000", LastSeen: now, RegisteredAt: now},
				{ID: "b", Status: NodeStatusActive, Endpoint: "", LastSeen: now, RegisteredAt: now},
			},
			wantConnections: 1,
			wantConnType:    "relay",
		},
		{
			name: "inactive_node_skipped",
			nodes: []*Node{
				{ID: "a", Status: NodeStatusActive, Endpoint: "1.1.1.1:5000", LastSeen: now, RegisteredAt: now},
				{ID: "b", Status: NodeStatusDisabled, Endpoint: "2.2.2.2:5000", LastSeen: now, RegisteredAt: now},
			},
			wantConnections: 0, // only 1 active recent, so no pairs
		},
		{
			name: "old_last_seen_no_connection",
			nodes: []*Node{
				{ID: "a", Status: NodeStatusActive, Endpoint: "1.1.1.1:5000", LastSeen: now, RegisteredAt: now},
				{ID: "b", Status: NodeStatusActive, Endpoint: "2.2.2.2:5000", LastSeen: oldTime, RegisteredAt: oldTime},
			},
			wantConnections: 0, // b is not recent, so no connection from a to b
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTopologyFromNodes(tt.nodes)
			conns, ok := result["connections"].([]TopologyConnection)
			if !ok {
				if tt.wantConnections > 0 {
					t.Fatalf("expected %d connections but got none or wrong type", tt.wantConnections)
				}
				return
			}
			if len(conns) != tt.wantConnections {
				t.Fatalf("expected %d connections, got %d", tt.wantConnections, len(conns))
			}
			if tt.wantConnections > 0 && tt.wantConnType != "" {
				if conns[0].Type != tt.wantConnType {
					t.Errorf("connection type = %q, want %q", conns[0].Type, tt.wantConnType)
				}
			}
		})
	}
}

// ─── 3. TestHub_CheckOrigin ───────────────────────────────────────────────────

func TestHub_CheckOrigin(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	tests := []struct {
		name           string
		allowedOrigins []string
		origin         string
		host           string
		want           bool
	}{
		{
			name:           "empty_origin",
			allowedOrigins: nil,
			origin:         "",
			host:           "example.com",
			want:           true,
		},
		{
			name:           "empty_allowedOrigins_origin_matches_host",
			allowedOrigins: nil,
			origin:         "http://example.com",
			host:           "example.com",
			want:           true,
		},
		{
			name:           "empty_allowedOrigins_bad_url",
			allowedOrigins: nil,
			origin:         "://bad-url",
			host:           "example.com",
			want:           false,
		},
		{
			name:           "empty_allowedOrigins_empty_host",
			allowedOrigins: nil,
			origin:         "http://example.com",
			host:           "",
			want:           false,
		},
		{
			name:           "allowedOrigins_wildcard",
			allowedOrigins: []string{"*"},
			origin:         "http://anything.example.com",
			host:           "different.com",
			want:           true,
		},
		{
			name:           "allowedOrigins_exact_match",
			allowedOrigins: []string{"http://trusted.example.com"},
			origin:         "http://trusted.example.com",
			host:           "different.com",
			want:           true,
		},
		{
			name:           "allowedOrigins_no_match",
			allowedOrigins: []string{"http://trusted.example.com"},
			origin:         "http://evil.example.com",
			host:           "different.com",
			want:           false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hub := NewHub(store, tt.allowedOrigins, "")
			defer hub.Close()

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			req.Host = tt.host

			got := hub.checkOrigin(req)
			if got != tt.want {
				t.Errorf("checkOrigin() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ─── 4. TestNewHub ────────────────────────────────────────────────────────────

func TestNewHub(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	origins := []string{"http://example.com"}
	hub := NewHub(store, origins, "secret123")
	defer hub.Close()

	if hub.store != store {
		t.Error("store not set correctly")
	}
	if hub.clients == nil {
		t.Error("clients map should be initialized")
	}
	if hub.broadcast == nil {
		t.Error("broadcast channel should be initialized")
	}
	if hub.register == nil {
		t.Error("register channel should be initialized")
	}
	if hub.unregister == nil {
		t.Error("unregister channel should be initialized")
	}
	if hub.done == nil {
		t.Error("done channel should be initialized")
	}
	if len(hub.allowedOrigins) != 1 || hub.allowedOrigins[0] != "http://example.com" {
		t.Errorf("allowedOrigins: want [http://example.com], got %v", hub.allowedOrigins)
	}
	if hub.adminSecret != "secret123" {
		t.Errorf("adminSecret: want 'secret123', got %q", hub.adminSecret)
	}
	if hub.startTime.IsZero() {
		t.Error("startTime should be set")
	}
	if hub.cpuSampler == nil {
		t.Error("cpuSampler should be initialized")
	}
}

// ─── 5. TestHub_Close ─────────────────────────────────────────────────────────

func TestHub_Close(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	t.Run("close_works", func(t *testing.T) {
		hub := NewHub(store, nil, "")
		hub.Close()

		select {
		case <-hub.done:
			// Expected: done channel is closed.
		default:
			t.Fatal("done channel should be closed after Close()")
		}
	})

	t.Run("double_close_safe", func(t *testing.T) {
		hub := NewHub(store, nil, "")
		hub.Close()
		hub.Close() // Should not panic.
	})
}

// ─── 6. TestHub_SendInitialState ───────────────────────────────────────────────

func TestHub_SendInitialState(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, nil, "")
	defer hub.Close()

	// Add a node so the initial state has something to report.
	store.AddNode(&Node{
		ID: "n1", PublicKey: "pk1", Hostname: "test",
		VirtualIP: "100.64.0.1", Status: NodeStatusActive,
		RegisteredAt: time.Now(), LastSeen: time.Now(),
	})

	// Create a mock client with a buffered send channel.
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 16),
	}

	hub.sendInitialState(client)

	// Expect 4 messages: nodes, peers, topology, stats.
	expectedMessages := 4
	received := 0
	timeout := time.After(2 * time.Second)
	for received < expectedMessages {
		select {
		case msg := <-client.send:
			received++
			// Verify each message is valid JSON with a "type" field.
			var envelope map[string]interface{}
			if err := json.Unmarshal(msg, &envelope); err != nil {
				t.Fatalf("message %d: invalid JSON: %v", received, err)
			}
			if _, ok := envelope["type"]; !ok {
				t.Fatalf("message %d: missing 'type' field", received)
			}
		case <-timeout:
			t.Fatalf("timed out waiting for messages, received %d of %d", received, expectedMessages)
		}
	}

	if received != expectedMessages {
		t.Errorf("expected %d messages, got %d", expectedMessages, received)
	}
}

// ─── 7. TestHub_Run_RegisterUnregister ────────────────────────────────────────

func TestHub_Run_RegisterUnregister(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, nil, "")

	// Start hub event loop.
	done := make(chan struct{})
	go func() {
		hub.Run()
		close(done)
	}()

	// Register a client.
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}
	hub.register <- client

	// Give the event loop time to process the registration.
	time.Sleep(50 * time.Millisecond)

	// Verify client is in the clients map.
	hub.mu.RLock()
	_, exists := hub.clients[client]
	hub.mu.RUnlock()
	if !exists {
		t.Fatal("client should be registered in hub")
	}

	// Drain messages from sendInitialState so the channel is not full.
_drain:
	for {
		select {
		case <-client.send:
		default:
			break _drain
		}
	}

	// Unregister the client.
	hub.unregister <- client

	// Give the event loop time to process the unregister.
	time.Sleep(50 * time.Millisecond)

	// Verify client is removed.
	hub.mu.RLock()
	_, exists = hub.clients[client]
	hub.mu.RUnlock()
	if exists {
		t.Fatal("client should be unregistered from hub")
	}

	// Verify the send channel was closed.
	select {
	case _, ok := <-client.send:
		if ok {
			t.Fatal("send channel should be closed after unregister")
		}
	default:
		// Also acceptable: channel may have been drained and closed.
	}

	// Shut down the hub.
	hub.Close()

	select {
	case <-done:
		// Hub exited cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("hub.Run() did not exit after Close()")
	}
}

// ─── 8. TestPoller_Snapshot ───────────────────────────────────────────────────

func TestPoller_Snapshot(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	store.AddNode(&Node{
		ID: "snap1", PublicKey: "pk1", Hostname: "snapshot-node",
		VirtualIP: "100.64.0.1", Status: NodeStatusActive,
		RegisteredAt: now, LastSeen: now,
	})

	poller := NewPoller(store)
	state := poller.snapshot()

	if len(state.Nodes) != 1 {
		t.Fatalf("expected 1 node in snapshot, got %d", len(state.Nodes))
	}
	if state.Nodes[0].ID != "snap1" {
		t.Errorf("node ID: want 'snap1', got %q", state.Nodes[0].ID)
	}
	if state.Version == 0 {
		t.Error("version should be non-zero after AddNode")
	}
	if state.UpdatedAt == "" {
		t.Error("updatedAt should not be empty")
	}
	if state.DERPMap != nil {
		t.Error("DERPMap should be nil when no DERP map function is set")
	}
}

// ─── 9. TestPoller_SnapshotWithDERPMap ─────────────────────────────────────────

func TestPoller_SnapshotWithDERPMap(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	poller := NewPoller(store)

	// Set a DERP map function.
	testDERPMap := &DERPMap{
		Regions: []*DERPRegion{{
			RegionID:   1,
			RegionCode: "test",
			RegionName: "Test Relay",
			Nodes: []*DERPNode{{
				Name:     "test-relay",
				RegionID: 1,
				HostName: "relay.example.com",
				DERPPort: 443,
			}},
		}},
	}
	poller.SetDERPMapFn(func() *DERPMap { return testDERPMap })

	state := poller.snapshot()

	if state.DERPMap == nil {
		t.Fatal("DERPMap should not be nil when SetDERPMapFn is configured")
	}
	if len(state.DERPMap.Regions) != 1 {
		t.Fatalf("expected 1 DERP region, got %d", len(state.DERPMap.Regions))
	}
	if state.DERPMap.Regions[0].RegionCode != "test" {
		t.Errorf("region code: want 'test', got %q", state.DERPMap.Regions[0].RegionCode)
	}
}

// ─── 10. TestPoller_WaitForUpdate_ImmediateReturn ─────────────────────────────

func TestPoller_WaitForUpdate_ImmediateReturn(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStore(filepath.Join(dir, "state.json"))
	if err != nil {
		t.Fatal(err)
	}

	// Add a node so the store has a non-zero UpdatedAt timestamp.
	store.AddNode(&Node{
		ID: "w1", PublicKey: "pk1", Hostname: "wait-node",
		VirtualIP: "100.64.0.2", Status: NodeStatusActive,
		RegisteredAt: time.Now(), LastSeen: time.Now(),
	})

	poller := NewPoller(store)

	// sinceVersion = 0 is older than the current UpdatedAt.UnixNano().
	// WaitForUpdate should return immediately without blocking.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	start := time.Now()
	state := poller.WaitForUpdate(ctx, 0)
	elapsed := time.Since(start)

	if elapsed > 1*time.Second {
		t.Fatalf("WaitForUpdate should have returned immediately, took %v", elapsed)
	}
	if len(state.Nodes) == 0 {
		t.Fatal("expected nodes in immediate poll response")
	}
	if state.Version == 0 {
		t.Error("version should be non-zero after AddNode")
	}
}

// ─── 11. TestHub_Run_StartsAndStops ───────────────────────────────────────────

func TestHub_Run_StartsAndStops(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, nil, "")

	done := make(chan struct{})
	go func() {
		hub.Run()
		close(done)
	}()

	// Give the event loop a moment to start.
	time.Sleep(20 * time.Millisecond)

	// Close should cause Run to exit.
	hub.Close()

	select {
	case <-done:
		// Success: Run exited.
	case <-time.After(2 * time.Second):
		t.Fatal("hub.Run() did not exit after Close()")
	}
}

// ─── 12. TestHub_Run_DrainsClientsOnShutdown ──────────────────────────────────

func TestHub_Run_DrainsClientsOnShutdown(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, nil, "")

	done := make(chan struct{})
	go func() {
		hub.Run()
		close(done)
	}()

	// Register several clients.
	const numClients = 3
	clients := make([]*Client, numClients)
	for i := range clients {
		clients[i] = &Client{
			hub:  hub,
			send: make(chan []byte, 256),
		}
		hub.register <- clients[i]
	}

	// Wait for registrations to be processed.
	time.Sleep(50 * time.Millisecond)

	// Drain initial state messages so send channels are not full.
	for _, c := range clients {
		drainChan(c.send)
	}

	// Close hub — should drain all clients.
	hub.Close()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("hub.Run() did not exit")
	}

	// All client send channels should be closed.
	for i, c := range clients {
		select {
		case _, ok := <-c.send:
			if ok {
				t.Errorf("client %d: send channel should be closed", i)
			}
		default:
			// Channel may be closed but we hit default; read again to confirm.
			_, ok := <-c.send
			if ok {
				t.Errorf("client %d: send channel should be closed on second read", i)
			}
		}
	}

	hub.mu.RLock()
	count := len(hub.clients)
	hub.mu.RUnlock()
	if count != 0 {
		t.Fatalf("expected 0 clients after shutdown, got %d", count)
	}
}

// ─── 13. TestHub_BroadcastUpdate ──────────────────────────────────────────────

func TestHub_BroadcastUpdate(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	// Add a node so the update has data.
	now := time.Now()
	store.AddNode(&Node{
		ID: "bcast1", PublicKey: "pk-bcast", Hostname: "bcast-node",
		VirtualIP: "100.64.0.10", Status: NodeStatusActive,
		RegisteredAt: now, LastSeen: now,
	})

	hub := NewHub(store, nil, "")
	defer hub.Close()

	hub.broadcastUpdate()

	// broadcastUpdate should have queued 4 messages (nodes, peers, topology, stats).
	expectedTypes := map[string]bool{
		"nodes":    false,
		"peers":    false,
		"topology": false,
		"stats":    false,
	}

	timeout := time.After(2 * time.Second)
	for i := 0; i < 4; i++ {
		select {
		case msg := <-hub.broadcast:
			var envelope map[string]interface{}
			if err := json.Unmarshal(msg, &envelope); err != nil {
				t.Fatalf("message %d: invalid JSON: %v", i+1, err)
			}
			msgType, _ := envelope["type"].(string)
			if _, ok := expectedTypes[msgType]; !ok {
				t.Errorf("unexpected message type: %q", msgType)
			}
			expectedTypes[msgType] = true
		case <-timeout:
			t.Fatalf("timed out waiting for message %d", i+1)
		}
	}

	for typ, seen := range expectedTypes {
		if !seen {
			t.Errorf("message type %q was not received", typ)
		}
	}
}

// ─── 14. TestHub_BroadcastUpdate_SkipsWhenFull ───────────────────────────────

func TestHub_BroadcastUpdate_SkipsWhenFull(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, nil, "")
	defer hub.Close()

	// Fill the broadcast channel to capacity (64).
	for i := 0; i < cap(hub.broadcast); i++ {
		hub.broadcast <- []byte("filler")
	}

	// broadcastUpdate should not block even though the channel is full.
	done := make(chan struct{})
	go func() {
		hub.broadcastUpdate()
		close(done)
	}()

	select {
	case <-done:
		// Success: broadcastUpdate returned without blocking.
	case <-time.After(2 * time.Second):
		t.Fatal("broadcastUpdate blocked when broadcast channel was full")
	}

	// Drain the channel so the hub can be closed cleanly.
	for i := 0; i < cap(hub.broadcast); i++ {
		<-hub.broadcast
	}
}

// ─── 15. TestHub_Run_BroadcastToClients ──────────────────────────────────────

func TestHub_Run_BroadcastToClients(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, nil, "")

	runDone := make(chan struct{})
	go func() {
		hub.Run()
		close(runDone)
	}()

	// Register a client.
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 256),
	}
	hub.register <- client
	time.Sleep(30 * time.Millisecond)

	// Drain initial state messages.
	for {
		select {
		case <-client.send:
		default:
			goto done1
		}
	}
done1:

	// Broadcast a message through the hub.
	testMsg := []byte(`{"type":"test"}`)
	hub.broadcast <- testMsg

	// Client should receive the message.
	select {
	case msg := <-client.send:
		if string(msg) != string(testMsg) {
			t.Fatalf("received message %q, want %q", string(msg), string(testMsg))
		}
	case <-time.After(2 * time.Second):
		t.Fatal("client did not receive broadcast message")
	}

	hub.Close()
	<-runDone
}

// ─── 16. TestHub_Run_BroadcastSlowClient ─────────────────────────────────────

func TestHub_Run_BroadcastSlowClient(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, nil, "")

	runDone := make(chan struct{})
	go func() {
		hub.Run()
		close(runDone)
	}()

	// Register a client with a tiny send buffer.
	client := &Client{
		hub:  hub,
		send: make(chan []byte, 1),
	}
	hub.register <- client
	time.Sleep(30 * time.Millisecond)

	// Drain initial state messages.
	for {
		select {
		case <-client.send:
		default:
			goto done2
		}
	}
done2:

	// Fill the send channel so the next broadcast can't deliver.
	client.send <- []byte("blocking-msg")

	// Broadcast another message; the slow client should be evicted.
	hub.broadcast <- []byte("evict-msg")
	time.Sleep(50 * time.Millisecond)

	// Client should have been removed from the hub.
	hub.mu.RLock()
	_, exists := hub.clients[client]
	hub.mu.RUnlock()
	if exists {
		t.Fatal("slow client should have been removed from hub")
	}

	hub.Close()
	<-runDone
}

// ─── 17. TestHub_ServeWS_Success ─────────────────────────────────────────────

func TestHub_ServeWS_Success(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	// Add a node so the initial state has data.
	now := time.Now()
	store.AddNode(&Node{
		ID: "ws1", PublicKey: "pk-ws", Hostname: "ws-node",
		VirtualIP: "100.64.0.20", Status: NodeStatusActive,
		RegisteredAt: now, LastSeen: now,
	})

	hub := NewHub(store, []string{"*"}, "")

	// Start the hub event loop.
	go hub.Run()

	// Set up an HTTP server that delegates to ServeWS.
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	defer hub.Close()

	// Connect with a WebSocket client.
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("failed to dial WebSocket: %v", err)
	}
	defer wsConn.Close()

	// Read messages from the server. We expect the initial state messages
	// (nodes, peers, topology, stats).
	drainInitialState(t, wsConn)
}

// ─── 18. TestHub_ServeWS_Unauthorized ────────────────────────────────────────

func TestHub_ServeWS_Unauthorized(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, []string{"*"}, "my-secret")

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	defer hub.Close()

	// Try to connect without a token — should fail with 401.
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected error dialing without token")
	}
	if resp != nil && resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", resp.StatusCode)
	}

	// Try with the correct token as a query parameter — should succeed.
	wsURLWithToken := wsURL + "?token=my-secret"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURLWithToken, nil)
	if err != nil {
		t.Fatalf("expected successful connection with valid token, got: %v", err)
	}
	wsConn.Close()

	// Try with the correct token via Authorization header.
	hdr := http.Header{}
	hdr.Set("Authorization", "Bearer my-secret")
	wsConn2, _, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil {
		t.Fatalf("expected successful connection with Bearer token, got: %v", err)
	}
	wsConn2.Close()

	// Try with a wrong token.
	wsURLBadToken := wsURL + "?token=wrong-secret"
	_, resp3, err := websocket.DefaultDialer.Dial(wsURLBadToken, nil)
	if err == nil {
		t.Fatal("expected error dialing with wrong token")
	}
	if resp3 != nil && resp3.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected status 401 for wrong token, got %d", resp3.StatusCode)
	}
}

// ─── 19. TestHub_ServeWS_ForbiddenOrigin ─────────────────────────────────────

func TestHub_ServeWS_ForbiddenOrigin(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, []string{"http://trusted.example.com"}, "")

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	defer hub.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"

	// Dial with a disallowed origin.
	hdr := http.Header{}
	hdr.Set("Origin", "http://evil.example.com")
	_, resp, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err == nil {
		t.Fatal("expected error dialing with forbidden origin")
	}
	if resp != nil && resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", resp.StatusCode)
	}
}

// ─── 20. TestHub_ServeWS_HubShuttingDown ─────────────────────────────────────

func TestHub_ServeWS_HubShuttingDown(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, []string{"*"}, "")

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	// Close the hub before trying to connect.
	hub.Close()

	// Attempting to connect should fail or immediately close because the hub's
	// done channel is closed. The upgrade itself succeeds, but ServeWS closes
	// the connection via the <-h.done branch. The client should observe an error
	// either on dial or on the first read.
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		// Dial itself failed — this is acceptable.
		return
	}
	defer wsConn.Close()

	// If dial succeeded, the server should close the connection immediately.
	// A subsequent read should fail.
	wsConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err = wsConn.ReadMessage()
	if err == nil {
		t.Fatal("expected error reading from a hub that is shut down")
	}
}

// ─── 21. TestClient_ReadPump_UnregistersOnClose ─────────────────────────────

func TestClient_ReadPump_UnregistersOnClose(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, []string{"*"}, "")

	runDone := make(chan struct{})
	go func() {
		hub.Run()
		close(runDone)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	// Wait for the client to be registered.
	time.Sleep(50 * time.Millisecond)

	hub.mu.RLock()
	initialCount := len(hub.clients)
	hub.mu.RUnlock()
	if initialCount != 1 {
		t.Fatalf("expected 1 client, got %d", initialCount)
	}

	// Close the client connection — readPump should detect it and unregister.
	wsConn.Close()

	// Wait for readPump to unregister the client.
	time.Sleep(100 * time.Millisecond)

	hub.mu.RLock()
	finalCount := len(hub.clients)
	hub.mu.RUnlock()
	if finalCount != 0 {
		t.Fatalf("expected 0 clients after disconnect, got %d", finalCount)
	}

	hub.Close()
	<-runDone
}

// ─── 22. TestClient_WritePump_SendsMessages ─────────────────────────────────

func TestClient_WritePump_SendsMessages(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, []string{"*"}, "")

	runDone := make(chan struct{})
	go func() {
		hub.Run()
		close(runDone)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer wsConn.Close()

	// Read and discard the initial state messages.
	drainInitialState(t, wsConn)

	// Send a broadcast through the hub.
	testPayload := `{"type":"custom-broadcast"}`
	hub.broadcast <- []byte(testPayload)

	// The client should receive the broadcast.
	wsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, msg, err := wsConn.ReadMessage()
	if err != nil {
		t.Fatalf("reading broadcast: %v", err)
	}
	if string(msg) != testPayload {
		t.Errorf("received %q, want %q", string(msg), testPayload)
	}

	hub.Close()
	<-runDone
}

// ─── 23. TestClient_WritePump_ExitsOnSendClose ──────────────────────────────

func TestClient_WritePump_ExitsOnSendClose(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, []string{"*"}, "")

	runDone := make(chan struct{})
	go func() {
		hub.Run()
		close(runDone)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer wsConn.Close()

	// Wait for client to be registered.
	time.Sleep(50 * time.Millisecond)

	// Snapshot the client so we can close its send channel.
	hub.mu.RLock()
	var client *Client
	for c := range hub.clients {
		client = c
		break
	}
	hub.mu.RUnlock()

	if client == nil {
		t.Fatal("no client found in hub")
	}

	// Unregister the client via the unregister channel to close the send channel.
	hub.unregister <- client
	time.Sleep(50 * time.Millisecond)

	// The writePump goroutine should have exited and sent a CloseMessage.
	// The WebSocket connection should be closed on the server side.
	// Read messages until we get an error (connection closed).
	wsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	gotError := false
	for {
		_, _, err = wsConn.ReadMessage()
		if err != nil {
			gotError = true
			break
		}
	}
	if !gotError {
		t.Fatal("expected error after writePump closes the connection")
	}

	hub.Close()
	<-runDone
}

// ─── 24. TestClient_ReadPump_ReadsMessages ───────────────────────────────────

func TestClient_ReadPump_ReadsMessages(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, []string{"*"}, "")

	runDone := make(chan struct{})
	go func() {
		hub.Run()
		close(runDone)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer wsConn.Close()

	// Send a few messages from the client. The readPump should consume them
	// without errors. We verify the connection stays alive.
	for i := 0; i < 3; i++ {
		if err := wsConn.WriteMessage(websocket.TextMessage, []byte("hello")); err != nil {
			t.Fatalf("write message %d: %v", i, err)
		}
	}

	// Read the initial state messages to confirm the connection is still alive.
	drainInitialState(t, wsConn)

	hub.Close()
	<-runDone
}

// ─── 25. TestClient_WritePump_BatchedMessages ───────────────────────────────

func TestClient_WritePump_BatchedMessages(t *testing.T) {
	store := newTestStore(t)
	defer store.StopGC()

	hub := NewHub(store, []string{"*"}, "")

	runDone := make(chan struct{})
	go func() {
		hub.Run()
		close(runDone)
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", hub.ServeWS)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer wsConn.Close()

	// Wait for client to be registered and drain initial state.
	time.Sleep(50 * time.Millisecond)
	drainInitialState(t, wsConn)

	// Send two broadcasts in quick succession. writePump may batch them
	// into a single WebSocket frame (separated by newlines) or deliver
	// them as separate frames depending on timing.
	msg1 := []byte(`{"type":"batch1"}`)
	msg2 := []byte(`{"type":"batch2"}`)
	hub.broadcast <- msg1
	hub.broadcast <- msg2

	// Read messages until we've seen both batch1 and batch2, or time out.
	wsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	var allReceived string
	for {
		_, received, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		allReceived += string(received) + "\n"
		if strings.Contains(allReceived, "batch1") && strings.Contains(allReceived, "batch2") {
			break
		}
	}

	if !strings.Contains(allReceived, "batch1") {
		t.Errorf("expected 'batch1' in received data, got %q", allReceived)
	}
	if !strings.Contains(allReceived, "batch2") {
		t.Errorf("expected 'batch2' in received data, got %q", allReceived)
	}

	hub.Close()
	<-runDone
}

// drainChan drains all pending values from a byte channel without blocking.
func drainChan(ch chan []byte) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

// drainInitialState reads WebSocket frames until all 4 initial message types
// (nodes, peers, topology, stats) have been received. It handles the case
// where writePump batches multiple messages into a single frame (separated by
// newlines). After draining, the read deadline is cleared.
func drainInitialState(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	seen := map[string]bool{"nodes": false, "peers": false, "topology": false, "stats": false}
	for {
		done := true
		for _, v := range seen {
			if !v {
				done = false
				break
			}
		}
		if done {
			break
		}
		_, msg, err := conn.ReadMessage()
		if err != nil {
			// Timeout or connection closed before all types seen.
			var missing []string
			for k, v := range seen {
				if !v {
					missing = append(missing, k)
				}
			}
			t.Fatalf("error reading initial state (missing: %v): %v", missing, err)
		}
		// Handle batched messages (multiple JSON objects separated by newlines).
		for _, part := range strings.Split(string(msg), "\n") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			var env map[string]interface{}
			if json.Unmarshal([]byte(part), &env) == nil {
				if typ, ok := env["type"].(string); ok {
					seen[typ] = true
				}
			}
		}
	}
	conn.SetReadDeadline(time.Time{}) // clear deadline
}
