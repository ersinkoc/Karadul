package coordinator

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
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
