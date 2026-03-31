package mesh

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// PeerState is the lifecycle state of a mesh peer.
type PeerState int

const (
	PeerDiscovered PeerState = iota
	PeerConnecting
	PeerDirect  // connected via direct UDP
	PeerRelayed // connected via DERP relay
	PeerIdle    // connected but no recent traffic
	PeerExpired // connection lost / timed out
)

func (s PeerState) String() string {
	switch s {
	case PeerDiscovered:
		return "discovered"
	case PeerConnecting:
		return "connecting"
	case PeerDirect:
		return "direct"
	case PeerRelayed:
		return "relayed"
	case PeerIdle:
		return "idle"
	case PeerExpired:
		return "expired"
	default:
		return fmt.Sprintf("state(%d)", int(s))
	}
}

const (
	idleTimeout    = 5 * time.Minute
	expiredTimeout = 10 * time.Minute
)

// Peer represents a remote mesh node.
type Peer struct {
	mu sync.RWMutex

	// Identity
	PublicKey [32]byte
	Hostname  string
	NodeID    string

	// Addressing
	VirtualIP net.IP
	Endpoint  *net.UDPAddr // current best-known UDP endpoint
	Routes    []*net.IPNet // subnets advertised by this peer

	// Session
	State         PeerState
	lastSeen      time.Time
	lastHandshake time.Time

	// Callbacks
	onStateChange func(*Peer, PeerState, PeerState)
}

// NewPeer creates a Peer in the Discovered state.
func NewPeer(pubKey [32]byte, hostname, nodeID string, vip net.IP) *Peer {
	return &Peer{
		PublicKey: pubKey,
		Hostname:  hostname,
		NodeID:    nodeID,
		VirtualIP: vip,
		State:     PeerDiscovered,
		lastSeen:  time.Now(),
	}
}

// SetOnStateChange installs a callback invoked on every state transition.
func (p *Peer) SetOnStateChange(fn func(*Peer, PeerState, PeerState)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onStateChange = fn
}

// Transition moves the peer to newState and fires the callback.
func (p *Peer) Transition(newState PeerState) {
	p.mu.Lock()
	old := p.State
	p.State = newState
	cb := p.onStateChange
	p.mu.Unlock()

	if cb != nil && old != newState {
		cb(p, old, newState)
	}
}

// Touch records that a packet was received from this peer.
func (p *Peer) Touch() {
	p.mu.Lock()
	old := p.State
	p.lastSeen = time.Now()
	if p.State == PeerIdle {
		p.State = PeerDirect
	}
	cb := p.onStateChange
	p.mu.Unlock()

	if cb != nil && old != PeerDirect {
		cb(p, old, PeerDirect)
	}
}

// SetEndpoint updates the known UDP endpoint.
func (p *Peer) SetEndpoint(addr *net.UDPAddr) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Endpoint = addr
}

// GetEndpoint returns a copy of the current endpoint.
func (p *Peer) GetEndpoint() *net.UDPAddr {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if p.Endpoint == nil {
		return nil
	}
	cp := *p.Endpoint
	return &cp
}

// GetState returns the current state.
func (p *Peer) GetState() PeerState {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State
}

// IsExpired reports whether the peer should be removed.
func (p *Peer) IsExpired() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.State == PeerExpired || time.Since(p.lastSeen) > expiredTimeout
}

// IdleCheck transitions an active peer to Idle if it has not been heard from recently.
func (p *Peer) IdleCheck() {
	p.mu.Lock()
	old := p.State
	if (p.State == PeerDirect || p.State == PeerRelayed) && time.Since(p.lastSeen) > idleTimeout {
		p.State = PeerIdle
	}
	if p.State == PeerIdle && time.Since(p.lastSeen) > expiredTimeout {
		p.State = PeerExpired
	}
	newState := p.State
	cb := p.onStateChange
	p.mu.Unlock()

	if cb != nil && old != newState {
		cb(p, old, newState)
	}
}
