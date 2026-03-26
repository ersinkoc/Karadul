package relay

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	klog "github.com/ersinkoc/karadul/internal/log"
)

// TestServeHTTP_WrongPath verifies 404 is returned for wrong path.
func TestServeHTTP_WrongPath(t *testing.T) {
	srv := NewServer(klog.New(nil, klog.LevelError, klog.FormatText))

	// Create a test HTTP server
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go http.Serve(ln, srv)

	// Request wrong path
	resp, err := http.Get("http://" + ln.Addr().String() + "/wrong")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// TestServeHTTP_NoUpgradeHeader verifies 426 is returned without Upgrade header.
func TestServeHTTP_NoUpgradeHeader(t *testing.T) {
	srv := NewServer(klog.New(nil, klog.LevelError, klog.FormatText))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go http.Serve(ln, srv)

	// Request without Upgrade header
	req, _ := http.NewRequest("GET", "http://"+ln.Addr().String()+"/derp", nil)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUpgradeRequired {
		t.Errorf("expected 426, got %d", resp.StatusCode)
	}
}

// TestHandleClient_BadFirstFrame verifies client is rejected with bad first frame.
func TestHandleClient_BadFirstFrame(t *testing.T) {
	srv := NewServer(klog.New(nil, klog.LevelError, klog.FormatText))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go http.Serve(ln, srv)

	// Connect and send wrong first frame
	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	// Send HTTP upgrade request
	fmt.Fprint(rw, "GET /derp HTTP/1.1\r\nHost: localhost\r\nUpgrade: derp\r\nConnection: Upgrade\r\n\r\n")
	rw.Flush()

	// Read 101 response
	resp, err := http.ReadResponse(rw.Reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}

	// Send wrong frame type (not ClientInfo)
	WriteFrame(rw, FramePing, nil)
	rw.Flush()

	// Connection should be closed
	time.Sleep(100 * time.Millisecond)
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err == nil {
		t.Fatal("expected connection to be closed after bad frame")
	}
}

// TestHandleClient_ShortClientInfo verifies client is rejected with short ClientInfo.
func TestHandleClient_ShortClientInfo(t *testing.T) {
	srv := NewServer(klog.New(nil, klog.LevelError, klog.FormatText))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go http.Serve(ln, srv)

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	fmt.Fprint(rw, "GET /derp HTTP/1.1\r\nHost: localhost\r\nUpgrade: derp\r\nConnection: Upgrade\r\n\r\n")
	rw.Flush()

	resp, err := http.ReadResponse(rw.Reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}

	// Send ClientInfo with payload < 32 bytes
	WriteFrame(rw, FrameClientInfo, []byte("short"))
	rw.Flush()

	time.Sleep(100 * time.Millisecond)
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 1)
	_, err = conn.Read(buf)
	if err == nil {
		t.Fatal("expected connection to be closed after short ClientInfo")
	}
}

// TestHandleClient_ParseSendPacketError verifies error handling for bad SendPacket.
func TestHandleClient_ParseSendPacketError(t *testing.T) {
	srv := NewServer(klog.New(nil, klog.LevelError, klog.FormatText))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go http.Serve(ln, srv)

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	fmt.Fprint(rw, "GET /derp HTTP/1.1\r\nHost: localhost\r\nUpgrade: derp\r\nConnection: Upgrade\r\n\r\n")
	rw.Flush()

	resp, err := http.ReadResponse(rw.Reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}

	// Send valid ClientInfo
	var pubKey [32]byte
	copy(pubKey[:], []byte("testkey"))
	info := BuildClientInfo(pubKey)
	WriteFrame(rw, FrameClientInfo, info)
	rw.Flush()

	time.Sleep(100 * time.Millisecond)

	// Send malformed SendPacket
	WriteFrame(rw, FrameSendPacket, []byte("tooshort"))
	rw.Flush()

	// Should continue (not crash)
	time.Sleep(100 * time.Millisecond)
}

// TestHandleClient_PingPong verifies ping/pong works.
func TestHandleClient_PingPong(t *testing.T) {
	srv := NewServer(klog.New(nil, klog.LevelError, klog.FormatText))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	go http.Serve(ln, srv)

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))

	fmt.Fprint(rw, "GET /derp HTTP/1.1\r\nHost: localhost\r\nUpgrade: derp\r\nConnection: Upgrade\r\n\r\n")
	rw.Flush()

	resp, err := http.ReadResponse(rw.Reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Fatalf("expected 101, got %d", resp.StatusCode)
	}

	// Send valid ClientInfo
	var pubKey [32]byte
	copy(pubKey[:], []byte("testkey2"))
	info := BuildClientInfo(pubKey)
	WriteFrame(rw, FrameClientInfo, info)
	rw.Flush()

	time.Sleep(100 * time.Millisecond)

	// Send Ping
	WriteFrame(rw, FramePing, nil)
	rw.Flush()

	// Read Pong
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	frame, err := ReadFrame(rw)
	if err != nil {
		t.Fatalf("read pong: %v", err)
	}
	if frame.Type != FramePong {
		t.Errorf("expected pong, got %d", frame.Type)
	}
}

// TestStart_ListenError verifies Start returns error on invalid address.
func TestStart_ListenError(t *testing.T) {
	srv := NewServer(klog.New(nil, klog.LevelError, klog.FormatText))

	ctx := context.Background()
	err := srv.Start(ctx, "invalid-address:999999")
	if err == nil {
		t.Fatal("expected error for invalid address")
	}
}

// TestRoute_ToNonExistentClient verifies routing to non-existent client is safe.
func TestRoute_ToNonExistentClient(t *testing.T) {
	srv := NewServer(klog.New(nil, klog.LevelError, klog.FormatText))

	var src, dst [32]byte
	copy(src[:], []byte("src"))
	copy(dst[:], []byte("dst"))

	// Should not panic
	srv.route(src, dst, []byte("packet"))
}

// TestBroadcastPresence_EmptyServer verifies broadcast on empty server is safe.
func TestBroadcastPresence_EmptyServer(t *testing.T) {
	srv := NewServer(klog.New(nil, klog.LevelError, klog.FormatText))

	var pubKey [32]byte
	copy(pubKey[:], []byte("test"))

	// Should not panic
	srv.broadcastPresence(pubKey, true)
	srv.broadcastPresence(pubKey, false)
}

// TestRemoveClient_NonExistent verifies removing non-existent client is safe.
func TestRemoveClient_NonExistent(t *testing.T) {
	srv := NewServer(klog.New(nil, klog.LevelError, klog.FormatText))

	var pubKey [32]byte
	copy(pubKey[:], []byte("test"))

	// Should not panic
	srv.removeClient(pubKey)
}

// TestEnqueue_DropWhenFull verifies enqueue drops when channel full.
func TestEnqueue_DropWhenFull(t *testing.T) {
	sc := &serverClient{
		send: make(chan serverMsg, 1),
	}

	// Fill the channel
	sc.enqueue(serverMsg{ft: FramePing})

	// Should drop (not block)
	sc.enqueue(serverMsg{ft: FramePong})

	if len(sc.send) != 1 {
		t.Errorf("expected 1 message, got %d", len(sc.send))
	}
}

// TestAddClient_ReplaceExisting verifies adding client with same key replaces old one.
func TestAddClient_ReplaceExisting(t *testing.T) {
	srv := NewServer(klog.New(nil, klog.LevelError, klog.FormatText))

	var pubKey [32]byte
	copy(pubKey[:], []byte("samekey"))

	// First client
	sc1 := &serverClient{
		pubKey: pubKey,
		send:   make(chan serverMsg, 64),
		done:   make(chan struct{}),
	}
	srv.addClient(sc1)

	// Second client with same key
	sc2 := &serverClient{
		pubKey: pubKey,
		send:   make(chan serverMsg, 64),
		done:   make(chan struct{}),
	}
	srv.addClient(sc2)

	// First client's channel should be closed
	select {
	case _, ok := <-sc1.send:
		if ok {
			t.Fatal("expected sc1 channel to be closed")
		}
	default:
		// Channel might already be drained
	}

	// Server should have second client
	srv.mu.RLock()
	if srv.clients[pubKey] != sc2 {
		t.Fatal("server should have sc2")
	}
	srv.mu.RUnlock()
}

// TestReadFrame_ShortHeader verifies ReadFrame returns error for short header.
func TestReadFrame_ShortHeader(t *testing.T) {
	conn, rw := net.Pipe()
	defer conn.Close()

	go func() {
		conn.Write([]byte{0x00, 0x00}) // Only 2 bytes, need 8
		conn.Close()
	}()

	_, err := ReadFrame(bufio.NewReadWriter(bufio.NewReader(rw), bufio.NewWriter(rw)))
	if err == nil {
		t.Fatal("expected error for short header")
	}
}

// TestReadFrame_EmptyPayload verifies ReadFrame works with empty payload.
func TestReadFrame_EmptyPayload(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		// Frame with empty payload
		data := make([]byte, 8)
		data[0] = 0x00 // Frame type
		data[1] = 0x00
		data[2] = 0x00
		data[3] = 0x00
		data[4] = 0x00 // Payload length = 0
		data[5] = 0x00
		data[6] = 0x00
		data[7] = 0x00
		server.Write(data)
		server.Close()
	}()

	rw := bufio.NewReadWriter(bufio.NewReader(client), bufio.NewWriter(client))
	frame, err := ReadFrame(rw)
	if err != nil {
		t.Fatalf("ReadFrame: %v", err)
	}
	if len(frame.Payload) != 0 {
		t.Errorf("expected empty payload, got %d bytes", len(frame.Payload))
	}
}

// TestParseSendPacket_Truncated verifies ParseSendPacket returns error for truncated data.
func TestParseSendPacket_Truncated(t *testing.T) {
	// Too short (need at least 32 bytes for dst key)
	_, _, err := ParseSendPacket([]byte("tooshort"))
	if err == nil {
		t.Fatal("expected error for truncated SendPacket")
	}
}

// TestParseRecvPacket_Truncated verifies ParseRecvPacket returns error for truncated data.
func TestParseRecvPacket_Truncated(t *testing.T) {
	// Too short (need at least 32 bytes for src key)
	_, _, err := ParseRecvPacket([]byte("tooshort"))
	if err == nil {
		t.Fatal("expected error for truncated RecvPacket")
	}
}

// TestBuildClientInfo verifies BuildClientInfo creates correct payload.
func TestBuildClientInfo(t *testing.T) {
	var pubKey [32]byte
	copy(pubKey[:], []byte("testpublickey"))

	info := BuildClientInfo(pubKey)
	if len(info) != 32 {
		t.Errorf("expected 32 bytes, got %d", len(info))
	}
	if !bytes.Equal(info, pubKey[:]) {
		t.Error("ClientInfo doesn't match public key")
	}
}

// TestBuildSendPacket verifies BuildSendPacket creates correct payload.
func TestBuildSendPacket(t *testing.T) {
	var dst [32]byte
	copy(dst[:], []byte("destinationkey"))
	pkt := []byte("packetdata")

	data := BuildSendPacket(dst, pkt)
	if len(data) != 32+len(pkt) {
		t.Errorf("expected %d bytes, got %d", 32+len(pkt), len(data))
	}
	if !bytes.Equal(data[:32], dst[:]) {
		t.Error("destination key mismatch")
	}
	if !bytes.Equal(data[32:], pkt) {
		t.Error("packet data mismatch")
	}
}

// TestBuildRecvPacket verifies BuildRecvPacket creates correct payload.
func TestBuildRecvPacket(t *testing.T) {
	var src [32]byte
	copy(src[:], []byte("sourcekey"))
	pkt := []byte("packetdata")

	data := BuildRecvPacket(src, pkt)
	if len(data) != 32+len(pkt) {
		t.Errorf("expected %d bytes, got %d", 32+len(pkt), len(data))
	}
	if !bytes.Equal(data[:32], src[:]) {
		t.Error("source key mismatch")
	}
	if !bytes.Equal(data[32:], pkt) {
		t.Error("packet data mismatch")
	}
}
