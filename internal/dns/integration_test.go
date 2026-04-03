package dns

import (
	"encoding/binary"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	klog "github.com/karadul/karadul/internal/log"
)

// TestMagicDNS_ConcurrentSetLookup verifies thread safety of Set/Lookup.
func TestMagicDNS_ConcurrentSetLookup(t *testing.T) {
	m := NewMagicDNS()
	var wg sync.WaitGroup
	var lookups int32

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				ip := net.ParseIP("100.64.0.1").To4()
				ip[3] = byte(n)
				m.Set("host", ip)
			}
		}(i)
	}

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = m.Lookup("host")
				atomic.AddInt32(&lookups, 1)
			}
		}()
	}

	wg.Wait()
	if atomic.LoadInt32(&lookups) != 400 {
		t.Fatalf("expected 400 lookups, got %d", lookups)
	}
}

// TestMagicDNS_ConcurrentUpdateAndLookup verifies Update is safe under concurrent reads.
func TestMagicDNS_ConcurrentUpdateAndLookup(t *testing.T) {
	m := NewMagicDNS()
	m.Set("host-a", net.ParseIP("100.64.0.1"))

	var wg sync.WaitGroup
	stop := make(chan struct{})

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					m.Lookup("host-a")
				}
			}
		}()
	}

	for i := 0; i < 10; i++ {
		entries := map[string]net.IP{
			"host-b": net.ParseIP("100.64.0.2"),
		}
		m.Update(entries)
	}
	close(stop)
	wg.Wait()
}

// TestMagicDNS_AllIsSnapshot verifies All returns a snapshot (mutation safe).
func TestMagicDNS_AllIsSnapshot(t *testing.T) {
	m := NewMagicDNS()
	m.Set("host-a", net.ParseIP("100.64.0.1"))

	snapshot := m.All()
	m.Set("host-b", net.ParseIP("100.64.0.2"))

	if len(snapshot) != 1 {
		t.Fatalf("snapshot should have 1 entry, got %d", len(snapshot))
	}
	if _, ok := snapshot["host-b"]; ok {
		t.Fatal("snapshot should not contain host-b added after All()")
	}
}

// TestMagicDNS_DeleteNonExistent verifies Delete on a non-existent key is safe.
func TestMagicDNS_DeleteNonExistent(t *testing.T) {
	m := NewMagicDNS()
	m.Delete("nonexistent")
	if m.Lookup("nonexistent") != nil {
		t.Fatal("lookup after deleting nonexistent should return nil")
	}
}

// TestMagicDNS_SetOverwrite verifies Set overwrites existing entry.
func TestMagicDNS_SetOverwrite(t *testing.T) {
	m := NewMagicDNS()
	m.Set("host", net.ParseIP("100.64.0.1"))
	m.Set("host", net.ParseIP("100.64.0.2"))
	got := m.Lookup("host")
	if !got.Equal(net.ParseIP("100.64.0.2")) {
		t.Fatalf("expected IP 100.64.0.2 after overwrite, got %v", got)
	}
}

// TestMagicDNS_UpdateEmpty verifies Update with empty map clears all entries.
func TestMagicDNS_UpdateEmpty(t *testing.T) {
	m := NewMagicDNS()
	m.Set("host-a", net.ParseIP("100.64.0.1"))
	m.Update(map[string]net.IP{})
	if m.Lookup("host-a") != nil {
		t.Fatal("Update with empty map should clear entries")
	}
	if len(m.All()) != 0 {
		t.Fatal("All() should be empty after Update with empty map")
	}
}

// TestConcat verifies concat helper joins multiple byte slices correctly.
func TestConcat(t *testing.T) {
	result := concat([]byte("hello"), []byte(" "), []byte("world"))
	if string(result) != "hello world" {
		t.Errorf("concat: want %q, got %q", "hello world", string(result))
	}

	empty := concat()
	if len(empty) != 0 {
		t.Errorf("concat() should return empty slice, got %d bytes", len(empty))
	}

	single := concat([]byte("solo"))
	if string(single) != "solo" {
		t.Errorf("concat single: want %q, got %q", "solo", string(single))
	}
}

// TestBuildNXDomain_WithQuestion verifies the NXDomain response includes the question section.
func TestBuildNXDomain_WithQuestion(t *testing.T) {
	q := encodeName("missing.web.karadul")
	q = append(q, 0x00, byte(dnsTypeA), 0x00, 0x01)

	resp := buildNXDomain(0x1234, q)
	qdCount := binary.BigEndian.Uint16(resp[4:])
	if qdCount != 1 {
		t.Errorf("qdcount: want 1, got %d", qdCount)
	}
	if len(resp) <= 12 {
		t.Fatal("NXDomain response should include question section")
	}
}

// TestBuildAResponse_IPInAnswer verifies the A record contains the correct IP.
func TestBuildAResponse_IPInAnswer(t *testing.T) {
	q := encodeName("myhost.web.karadul")
	q = append(q, 0x00, byte(dnsTypeA), 0x00, 0x01)
	ip := net.ParseIP("100.64.0.5").To4()

	resp := buildAResponse(0x5555, q, ip)
	ansStart := len(resp) - 16
	rdlen := binary.BigEndian.Uint16(resp[ansStart+10 : ansStart+12])
	if rdlen != 4 {
		t.Errorf("rdlen: want 4, got %d", rdlen)
	}
	rdata := resp[ansStart+12 : ansStart+16]
	if !net.IP(rdata).Equal(ip) {
		t.Errorf("rdata: want %v, got %v", ip, rdata)
	}
}

// TestResolver_FullIntegration_ARecord verifies a complete DNS query-response
// cycle through a live UDP socket with A record resolution.
func TestResolver_FullIntegration_ARecord(t *testing.T) {
	magic := NewMagicDNS()
	magic.Set("integration", net.ParseIP("100.64.0.42"))

	r := NewResolver("127.0.0.1:0", "127.0.0.1:1", magic,
		klog.New(nil, klog.LevelError, klog.FormatText))

	started := make(chan *net.UDPAddr, 1)
	go func() { _ = r.Start() }()
	t.Cleanup(func() { r.Close() })

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

	var addr *net.UDPAddr
	select {
	case addr = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("resolver did not start")
	}

	client, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	pkt := buildQuery(0xAAAA, "integration.web.karadul", dnsTypeA)
	_ = client.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := client.Write(pkt); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp := make([]byte, 512)
	n, err := client.Read(resp)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	resp = resp[:n]

	txID := binary.BigEndian.Uint16(resp[0:])
	if txID != 0xAAAA {
		t.Errorf("txID: want 0xAAAA, got 0x%04X", txID)
	}
	anCount := binary.BigEndian.Uint16(resp[6:])
	if anCount != 1 {
		t.Errorf("ancount: want 1, got %d", anCount)
	}
}

// TestResolver_FullIntegration_NXDomain verifies NXDOMAIN through the full pipeline.
func TestResolver_FullIntegration_NXDomain(t *testing.T) {
	magic := NewMagicDNS()

	r := NewResolver("127.0.0.1:0", "127.0.0.1:1", magic,
		klog.New(nil, klog.LevelError, klog.FormatText))

	started := make(chan *net.UDPAddr, 1)
	go func() { _ = r.Start() }()
	t.Cleanup(func() { r.Close() })

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

	var addr *net.UDPAddr
	select {
	case addr = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("resolver did not start")
	}

	client, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	pkt := buildQuery(0xCCCC, "missing.web.karadul", dnsTypeA)
	_ = client.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := client.Write(pkt); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp := make([]byte, 512)
	n, err := client.Read(resp)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	resp = resp[:n]

	flags := binary.BigEndian.Uint16(resp[2:])
	rcode := flags & 0x000F
	if rcode != dnsRcodeNXDomain {
		t.Errorf("want NXDOMAIN (3), got rcode %d", rcode)
	}
}

// TestResolver_FullIntegration_AAAA verifies AAAA query through the full pipeline.
func TestResolver_FullIntegration_AAAA(t *testing.T) {
	magic := NewMagicDNS()
	magic.Set("v6host", net.ParseIP("2001:db8::1"))

	r := NewResolver("127.0.0.1:0", "127.0.0.1:1", magic,
		klog.New(nil, klog.LevelError, klog.FormatText))

	started := make(chan *net.UDPAddr, 1)
	go func() { _ = r.Start() }()
	t.Cleanup(func() { r.Close() })

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

	var addr *net.UDPAddr
	select {
	case addr = <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("resolver did not start")
	}

	client, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	pkt := buildQuery(0xBBBB, "v6host.web.karadul", dnsTypeAAAA)
	_ = client.SetWriteDeadline(time.Now().Add(time.Second))
	if _, err := client.Write(pkt); err != nil {
		t.Fatal(err)
	}
	_ = client.SetReadDeadline(time.Now().Add(2 * time.Second))
	resp := make([]byte, 512)
	n, err := client.Read(resp)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	resp = resp[:n]

	flags := binary.BigEndian.Uint16(resp[2:])
	rcode := flags & 0x000F
	if rcode != 0 {
		t.Errorf("want rcode 0, got %d", rcode)
	}
	anCount := binary.BigEndian.Uint16(resp[6:])
	if anCount != 1 {
		t.Errorf("ancount: want 1, got %d", anCount)
	}
}
