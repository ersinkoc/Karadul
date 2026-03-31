package auth

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
)

// ACLRule describes a single access-control rule.
type ACLRule struct {
	Action string   `json:"action"` // "allow" or "deny"
	Src    []string `json:"src"`    // CIDR, "tag:X", "group:X", "*"
	Dst    []string `json:"dst"`
	Ports  []string `json:"ports,omitempty"` // e.g. "80", "443", "1000-2000"
}

// ACLPolicy is the complete network access-control policy.
type ACLPolicy struct {
	Version int                 `json:"version"`
	Rules   []ACLRule           `json:"rules"`
	Groups  map[string][]string `json:"groups,omitempty"` // group name → list of CIDRs/IPs
	Tags    map[string][]string `json:"tags,omitempty"`   // tag name → list of node IDs
}

// Engine evaluates ACL rules against packet metadata.
type Engine struct {
	mu     sync.RWMutex
	policy ACLPolicy
}

// NewEngine creates an Engine with the given policy.
func NewEngine(policy ACLPolicy) *Engine {
	return &Engine{policy: policy}
}

// UpdatePolicy atomically replaces the policy.
func (e *Engine) UpdatePolicy(policy ACLPolicy) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.policy = policy
}

// Allow returns true if a packet from srcIP to dstIP:dstPort is permitted.
// An empty policy (no rules) defaults to allow-all.
func (e *Engine) Allow(srcIP, dstIP net.IP, dstPort uint16) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.policy.Rules) == 0 {
		return true
	}

	for _, rule := range e.policy.Rules {
		if !matchesAddrList(rule.Src, srcIP, e.policy.Groups) {
			continue
		}
		if !matchesAddrList(rule.Dst, dstIP, e.policy.Groups) {
			continue
		}
		if len(rule.Ports) > 0 && !matchesPort(rule.Ports, dstPort) {
			continue
		}
		return rule.Action == "allow"
	}
	// Default deny if rules exist but none matched.
	return false
}

func matchesAddrList(list []string, ip net.IP, groups map[string][]string) bool {
	for _, entry := range list {
		if entry == "*" {
			return true
		}
		if strings.HasPrefix(entry, "group:") {
			gname := entry[len("group:"):]
			if members, ok := groups[gname]; ok {
				for _, m := range members {
					if matchesCIDR(m, ip) {
						return true
					}
				}
			}
			continue
		}
		if matchesCIDR(entry, ip) {
			return true
		}
	}
	return false
}

func matchesCIDR(cidr string, ip net.IP) bool {
	// Plain IP comparison.
	if !strings.Contains(cidr, "/") {
		return ip.Equal(net.ParseIP(cidr))
	}
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	return network.Contains(ip)
}

func matchesPort(ports []string, port uint16) bool {
	for _, spec := range ports {
		if strings.Contains(spec, "-") {
			parts := strings.SplitN(spec, "-", 2)
			lo, err1 := strconv.ParseUint(parts[0], 10, 16)
			hi, err2 := strconv.ParseUint(parts[1], 10, 16)
			if err1 == nil && err2 == nil && port >= uint16(lo) && port <= uint16(hi) {
				return true
			}
		} else {
			p, err := strconv.ParseUint(spec, 10, 16)
			if err == nil && uint16(p) == port {
				return true
			}
		}
	}
	return false
}

// Validate checks the policy for structural errors.
func (p *ACLPolicy) Validate() error {
	// Validate groups contain valid CIDRs or plain IPs.
	for gname, members := range p.Groups {
		for _, m := range members {
			if m == "*" {
				continue
			}
			if strings.Contains(m, "/") {
				if _, _, err := net.ParseCIDR(m); err != nil {
					return fmt.Errorf("group %q: invalid CIDR %q", gname, m)
				}
			} else if net.ParseIP(m) == nil {
				return fmt.Errorf("group %q: invalid IP %q", gname, m)
			}
		}
	}

	for i, rule := range p.Rules {
		if rule.Action != "allow" && rule.Action != "deny" {
			return fmt.Errorf("rule %d: action must be 'allow' or 'deny'", i)
		}
		if len(rule.Src) == 0 {
			return fmt.Errorf("rule %d: src is required", i)
		}
		if len(rule.Dst) == 0 {
			return fmt.Errorf("rule %d: dst is required", i)
		}
		for _, port := range rule.Ports {
			if !isValidPort(port) {
				return fmt.Errorf("rule %d: invalid port %q", i, port)
			}
		}
	}
	return nil
}

// isValidPort checks that a port spec is a valid number or range (e.g. "80", "443", "8080-8090").
func isValidPort(s string) bool {
	if s == "" || s == "*" {
		return true
	}
	// Check for range (e.g. "8080-8090").
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		return isValidPortNum(s[:idx]) && isValidPortNum(s[idx+1:])
	}
	return isValidPortNum(s)
}

func isValidPortNum(s string) bool {
	if s == "" {
		return false
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return false
	}
	return n >= 0 && n <= 65535
}
