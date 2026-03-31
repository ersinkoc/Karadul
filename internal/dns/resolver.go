// Package dns implements a minimal UDP DNS server and magic DNS resolver.
// DNS wire format is parsed manually per RFC 1035.
// Only A, AAAA, and PTR queries are handled.
package dns

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	klog "github.com/karadul/karadul/internal/log"
)

// testDialUDP is a test hook to inject mock connections for testing.
// If nil, net.DialUDP is used.
var testDialUDP func(network string, laddr, raddr *net.UDPAddr) (net.Conn, error)

const (
	// MagicDomain is the suffix for karadul mesh DNS names.
	MagicDomain = "web.karadul"

	dnsMaxPacket = 512
	dnsPort      = 53
)

// DNS record types.
const (
	dnsTypeA    = 1
	dnsTypeAAAA = 28
	dnsTypePTR  = 12
	dnsTypeNS   = 2

	dnsClassIN = 1

	dnsRcodeNoError  = 0
	dnsRcodeNXDomain = 3
	dnsRcodeServFail = 2

	dnsFlagQR = 1 << 15
	dnsFlagAA = 1 << 10
	dnsFlagRD = 1 << 8
	dnsFlagRA = 1 << 7
)

// Resolver is a minimal UDP DNS server.
type Resolver struct {
	listenAddr string
	upstream   string
	magic      *MagicDNS
	log        *klog.Logger

	mu   sync.Mutex
	conn *net.UDPConn

	// Limit concurrent DNS handler goroutines.
	sem chan struct{}
}

// NewResolver creates a Resolver.
// listenAddr is e.g. "100.64.0.53:53".
// upstream is the upstream DNS server (e.g. "1.1.1.1:53").
func NewResolver(listenAddr, upstream string, magic *MagicDNS, log *klog.Logger) *Resolver {
	return &Resolver{
		listenAddr: listenAddr,
		upstream:   upstream,
		magic:      magic,
		log:        log,
		sem:       make(chan struct{}, 128),
	}
}

// Start begins listening for DNS queries. Blocks until conn is closed.
func (r *Resolver) Start() error {
	addr, err := net.ResolveUDPAddr("udp", r.listenAddr)
	if err != nil {
		return fmt.Errorf("resolve listen addr: %w", err)
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("listen dns: %w", err)
	}
	r.mu.Lock()
	r.conn = conn
	r.mu.Unlock()

	r.log.Info("dns resolver started", "addr", r.listenAddr)

	buf := make([]byte, dnsMaxPacket)
	for {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			return err
		}
		pkt := make([]byte, n)
		copy(pkt, buf[:n])
		select {
		case r.sem <- struct{}{}:
			go func() {
				r.handle(conn, src, pkt)
				<-r.sem
			}()
		default:
			// Too many concurrent queries; drop.
		}
	}
}

// Close stops the resolver.
func (r *Resolver) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func (r *Resolver) handle(conn *net.UDPConn, src *net.UDPAddr, pkt []byte) {
	resp, err := r.processQuery(pkt)
	if err != nil {
		r.log.Debug("dns: process query failed", "err", err)
		return
	}
	if resp != nil {
		_, _ = conn.WriteToUDP(resp, src)
	}
}

func (r *Resolver) processQuery(pkt []byte) ([]byte, error) {
	if len(pkt) < 12 {
		return nil, fmt.Errorf("dns packet too short")
	}

	txID := binary.BigEndian.Uint16(pkt[0:])
	flags := binary.BigEndian.Uint16(pkt[2:])

	// Only handle queries (QR=0).
	if flags&dnsFlagQR != 0 {
		return nil, nil
	}

	qdCount := int(binary.BigEndian.Uint16(pkt[4:]))
	if qdCount == 0 {
		return nil, nil
	}

	// Parse the first question.
	name, qtype, pos, err := parseQuestion(pkt, 12)
	if err != nil {
		return buildError(txID, dnsRcodeServFail), nil
	}
	_ = pos

	nameLower := strings.ToLower(name)

	// Check if it's a magic DNS query.
	if r.magic != nil && strings.HasSuffix(nameLower, "."+MagicDomain) || nameLower == MagicDomain {
		hostname := strings.TrimSuffix(nameLower, "."+MagicDomain)
		hostname = strings.TrimSuffix(hostname, ".")

		ip := r.magic.Lookup(hostname)
		if ip == nil {
			return buildNXDomain(txID, pkt[12:pos]), nil
		}

		switch qtype {
		case dnsTypeA:
			if ip4 := ip.To4(); ip4 != nil {
				return buildAResponse(txID, pkt[12:pos], ip4), nil
			}
			return buildNXDomain(txID, pkt[12:pos]), nil
		case dnsTypeAAAA:
			if ip.To4() == nil {
				return buildAAAAResponse(txID, pkt[12:pos], ip.To16()), nil
			}
			return buildNXDomain(txID, pkt[12:pos]), nil
		}
	}

	// Forward to upstream.
	return r.forward(pkt)
}

// forward sends the query to the upstream resolver and returns its response.
func (r *Resolver) forward(pkt []byte) ([]byte, error) {
	// Use test hook if available (for testing).
	if testDialUDP != nil {
		conn, err := testDialUDP("udp", nil, &net.UDPAddr{
			IP:   net.ParseIP(strings.TrimSuffix(r.upstream, ":53")),
			Port: 53,
		})
		if err != nil {
			return nil, fmt.Errorf("upstream connect: %w", err)
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
		if _, err := conn.Write(pkt); err != nil {
			return nil, err
		}
		buf := make([]byte, dnsMaxPacket)
		n, err := conn.Read(buf)
		if err != nil {
			return nil, err
		}
		return buf[:n], nil
	}

	conn, err := net.DialUDP("udp", nil, &net.UDPAddr{
		IP:   net.ParseIP(strings.TrimSuffix(r.upstream, ":53")),
		Port: 53,
	})
	if err != nil {
		// Try parsing as host:port.
		conn2, err2 := net.Dial("udp", r.upstream)
		if err2 != nil {
			return nil, fmt.Errorf("upstream connect: %w", err)
		}
		defer conn2.Close()
		_ = conn2.SetDeadline(time.Now().Add(3 * time.Second))
		if _, err := conn2.Write(pkt); err != nil {
			return nil, err
		}
		buf := make([]byte, dnsMaxPacket)
		n, err := conn2.Read(buf)
		if err != nil {
			return nil, err
		}
		return buf[:n], nil
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write(pkt); err != nil {
		return nil, err
	}
	buf := make([]byte, dnsMaxPacket)
	n, _, err := conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	}
	return buf[:n], nil
}

// --- DNS wire format helpers ---

// parseQuestion parses a DNS question from buf starting at pos.
// Returns name, qtype, and the new pos after the question.
func parseQuestion(buf []byte, pos int) (name string, qtype uint16, end int, err error) {
	name, pos, err = parseName(buf, pos)
	if err != nil {
		return "", 0, 0, err
	}
	if pos+4 > len(buf) {
		return "", 0, 0, fmt.Errorf("question too short")
	}
	qtype = binary.BigEndian.Uint16(buf[pos:])
	// qclass := binary.BigEndian.Uint16(buf[pos+2:])
	end = pos + 4
	return name, qtype, end, nil
}

// parseName parses a DNS encoded domain name. Returns the name and new offset.
func parseName(buf []byte, pos int) (string, int, error) {
	var parts []string
	for {
		if pos >= len(buf) {
			return "", 0, fmt.Errorf("name: out of bounds")
		}
		length := int(buf[pos])
		if length == 0 {
			pos++
			break
		}
		if length&0xC0 == 0xC0 {
			// Compression pointer.
			if pos+2 > len(buf) {
				return "", 0, fmt.Errorf("name: compression pointer OOB")
			}
			ptr := int(binary.BigEndian.Uint16(buf[pos:]) &^ 0xC000)
			label, _, err := parseName(buf, ptr)
			if err != nil {
				return "", 0, err
			}
			parts = append(parts, label)
			pos += 2
			break
		}
		pos++
		if pos+length > len(buf) {
			return "", 0, fmt.Errorf("name: label OOB")
		}
		parts = append(parts, string(buf[pos:pos+length]))
		pos += length
	}
	return strings.Join(parts, "."), pos, nil
}

// encodeName encodes a domain name in DNS wire format.
func encodeName(name string) []byte {
	var buf []byte
	labels := strings.Split(strings.TrimSuffix(name, "."), ".")
	for _, label := range labels {
		if label == "" {
			continue
		}
		buf = append(buf, byte(len(label)))
		buf = append(buf, []byte(label)...)
	}
	buf = append(buf, 0)
	return buf
}

// buildError builds a minimal DNS error response.
func buildError(txID uint16, rcode uint16) []byte {
	buf := make([]byte, 12)
	binary.BigEndian.PutUint16(buf[0:], txID)
	binary.BigEndian.PutUint16(buf[2:], uint16(dnsFlagQR|dnsFlagAA)|rcode)
	return buf
}

func buildNXDomain(txID uint16, question []byte) []byte {
	buf := make([]byte, 12+len(question))
	binary.BigEndian.PutUint16(buf[0:], txID)
	binary.BigEndian.PutUint16(buf[2:], uint16(dnsFlagQR|dnsFlagAA|dnsRcodeNXDomain))
	binary.BigEndian.PutUint16(buf[4:], 1) // qdcount
	copy(buf[12:], question)
	return buf
}

func buildAResponse(txID uint16, question, ip4 []byte) []byte {
	hdr := make([]byte, 12)
	binary.BigEndian.PutUint16(hdr[0:], txID)
	binary.BigEndian.PutUint16(hdr[2:], uint16(dnsFlagQR|dnsFlagAA|dnsFlagRA))
	binary.BigEndian.PutUint16(hdr[4:], 1) // qdcount
	binary.BigEndian.PutUint16(hdr[6:], 1) // ancount

	// Answer: name ptr(2) + type(2) + class(2) + ttl(4) + rdlen(2) + rdata(4)
	ans := make([]byte, 16)
	binary.BigEndian.PutUint16(ans[0:], 0xC00C) // pointer to question name
	binary.BigEndian.PutUint16(ans[2:], dnsTypeA)
	binary.BigEndian.PutUint16(ans[4:], dnsClassIN)
	binary.BigEndian.PutUint32(ans[6:], 60) // TTL 60s
	binary.BigEndian.PutUint16(ans[10:], 4)
	copy(ans[12:], ip4[:4])

	return concat(hdr, question, ans)
}

func buildAAAAResponse(txID uint16, question, ip6 []byte) []byte {
	hdr := make([]byte, 12)
	binary.BigEndian.PutUint16(hdr[0:], txID)
	binary.BigEndian.PutUint16(hdr[2:], uint16(dnsFlagQR|dnsFlagAA|dnsFlagRA))
	binary.BigEndian.PutUint16(hdr[4:], 1)
	binary.BigEndian.PutUint16(hdr[6:], 1)

	ans := make([]byte, 28)
	binary.BigEndian.PutUint16(ans[0:], 0xC00C)
	binary.BigEndian.PutUint16(ans[2:], dnsTypeAAAA)
	binary.BigEndian.PutUint16(ans[4:], dnsClassIN)
	binary.BigEndian.PutUint32(ans[6:], 60)
	binary.BigEndian.PutUint16(ans[10:], 16)
	copy(ans[12:], ip6[:16])

	return concat(hdr, question, ans)
}

func concat(parts ...[]byte) []byte {
	total := 0
	for _, p := range parts {
		total += len(p)
	}
	buf := make([]byte, total)
	pos := 0
	for _, p := range parts {
		copy(buf[pos:], p)
		pos += len(p)
	}
	return buf
}
