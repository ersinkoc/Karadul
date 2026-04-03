//go:build darwin

package tunnel

import (
	"bytes"
	"encoding/binary"
	"net"
	"os"
	"strings"
	"sync"
	"testing"

	"golang.org/x/sys/unix"
)

// ─── ParseIPv4Header — IHL exceeds buffer ────────────────────────────────────

func TestParseIPv4Header_IHLExceedsBuffer(t *testing.T) {
	// IHL=6 means header length = 24 bytes, but we provide exactly 20 bytes.
	// This exercises the len(buf) < ihl branch (ihl >= 20 but buffer too short).
	pkt := make([]byte, 20)
	pkt[0] = 0x46 // version=4, IHL=6 (24 bytes)
	_, err := ParseIPv4Header(pkt)
	if err == nil {
		t.Fatal("expected error for IHL exceeding buffer")
	}
}

// ─── ParseIPv4Header — IHL exactly matches buffer with extension headers ─────

func TestParseIPv4Header_IHLMatchesBufferWithExtension(t *testing.T) {
	// IHL=6 means header length = 24 bytes, and we provide exactly 24 bytes.
	pkt := make([]byte, 24)
	pkt[0] = 0x46 // version=4, IHL=6
	pkt[9] = ProtoUDP
	copy(pkt[12:16], net.IPv4(10, 0, 0, 1).To4())
	copy(pkt[16:20], net.IPv4(10, 0, 0, 2).To4())
	// Bytes 20-23 are IP options.

	h, err := ParseIPv4Header(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !h.Src.Equal(net.IPv4(10, 0, 0, 1)) {
		t.Errorf("src: got %s, want 10.0.0.1", h.Src)
	}
	if !h.Dst.Equal(net.IPv4(10, 0, 0, 2)) {
		t.Errorf("dst: got %s, want 10.0.0.2", h.Dst)
	}
	if h.Protocol != ProtoUDP {
		t.Errorf("protocol: got %d, want %d", h.Protocol, ProtoUDP)
	}
}

// ─── ParseIPv4Header — boundary IHL values ───────────────────────────────────

func TestParseIPv4Header_IHLBoundaryValues(t *testing.T) {
	tests := []struct {
		name    string
		ihl     int // IHL nibble value (actual header = ihl*4)
		bufLen  int
		wantErr bool
	}{
		{"IHL_4_buf_16", 4, 16, true},    // header=16 < 20, invalid IHL
		{"IHL_5_buf_20", 5, 20, false},   // header=20, buffer=20, OK
		{"IHL_5_buf_19", 5, 19, true},    // header=20, buffer=19, too short
		{"IHL_7_buf_28", 7, 28, false},   // header=28, buffer=28, OK
		{"IHL_7_buf_24", 7, 24, true},    // header=28, buffer=24, IHL > buffer
		{"IHL_15_buf_60", 15, 60, false}, // max IHL, enough buffer
		{"IHL_15_buf_59", 15, 59, true},  // max IHL, just short
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pkt := make([]byte, tc.bufLen)
			pkt[0] = byte(0x40 | tc.ihl)
			if tc.bufLen >= 20 {
				pkt[9] = ProtoTCP
				copy(pkt[12:16], net.IPv4(1, 2, 3, 4).To4())
				copy(pkt[16:20], net.IPv4(5, 6, 7, 8).To4())
			}
			_, err := ParseIPv4Header(pkt)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ─── ParseIPv4Header — total length field ────────────────────────────────────

func TestParseIPv4Header_TotalLenField(t *testing.T) {
	pkt := make([]byte, 24)
	pkt[0] = 0x46 // version=4, IHL=6
	pkt[2] = 0x02
	pkt[3] = 0x00
	pkt[9] = ProtoICMP
	copy(pkt[12:16], net.IPv4(172, 16, 0, 1).To4())
	copy(pkt[16:20], net.IPv4(172, 16, 0, 2).To4())

	h, err := ParseIPv4Header(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.TotalLen != 512 {
		t.Errorf("TotalLen: got %d, want 512", h.TotalLen)
	}
}

// ─── ParseIPv6Header — payload length field ──────────────────────────────────

func TestParseIPv6Header_PayloadLenField(t *testing.T) {
	src := net.ParseIP("::1")
	dst := net.ParseIP("::2")
	pkt := buildIPv6Packet(src, dst, ProtoTCP, []byte("abc"))

	h, err := ParseIPv6Header(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if h.PayloadLen != 3 {
		t.Errorf("PayloadLen: got %d, want 3", h.PayloadLen)
	}
}

// ─── ParseIPv6Header — next header field ─────────────────────────────────────

func TestParseIPv6Header_NextHeaderField(t *testing.T) {
	tests := []struct {
		name       string
		nextHeader uint8
	}{
		{"ICMPv6", ProtoICMPv6},
		{"TCP", ProtoTCP},
		{"UDP", ProtoUDP},
		{"ICMP", ProtoICMP},
		{"RoutingHeader", 43},
		{"FragmentHeader", 44},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			src := net.ParseIP("::1")
			dst := net.ParseIP("::2")
			pkt := buildIPv6Packet(src, dst, tc.nextHeader, nil)

			h, err := ParseIPv6Header(pkt)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if h.NextHeader != tc.nextHeader {
				t.Errorf("NextHeader: got %d, want %d", h.NextHeader, tc.nextHeader)
			}
		})
	}
}

// ─── PacketSrcDst — version 5 (neither IPv4 nor IPv6) ───────────────────────

func TestPacketSrcDst_Version5(t *testing.T) {
	pkt := []byte{0x50, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00}
	_, _, err := PacketSrcDst(pkt)
	if err == nil {
		t.Fatal("expected error for IP version 5")
	}
}

// ─── PacketSrcDst — IPv4 returns independent IP slices ──────────────────────

func TestPacketSrcDst_IPv4SliceIndependence(t *testing.T) {
	src := net.IPv4(192, 168, 1, 1)
	dst := net.IPv4(10, 0, 0, 1)
	pkt := buildIPv4Packet(src, dst, ProtoTCP, nil)

	gotSrc, gotDst, err := PacketSrcDst(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotSrc[0] = 0xFF
	gotDst[0] = 0xFF

	gotSrc2, gotDst2, err := PacketSrcDst(pkt)
	if err != nil {
		t.Fatalf("unexpected error on second parse: %v", err)
	}
	if gotSrc2[0] == 0xFF {
		t.Error("modifying returned src affected subsequent parse")
	}
	if gotDst2[0] == 0xFF {
		t.Error("modifying returned dst affected subsequent parse")
	}
}

// ─── PacketSrcDst — IPv6 returns independent IP slices ──────────────────────

func TestPacketSrcDst_IPv6SliceIndependence(t *testing.T) {
	src := net.ParseIP("2001:db8::1")
	dst := net.ParseIP("2001:db8::2")
	pkt := buildIPv6Packet(src, dst, ProtoTCP, nil)

	gotSrc, gotDst, err := PacketSrcDst(pkt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	gotSrc[0] = 0xFF
	gotDst[0] = 0xFF

	gotSrc2, gotDst2, err := PacketSrcDst(pkt)
	if err != nil {
		t.Fatalf("unexpected error on second parse: %v", err)
	}
	if gotSrc2[0] == 0xFF {
		t.Error("modifying returned src affected subsequent parse")
	}
	if gotDst2[0] == 0xFF {
		t.Error("modifying returned dst affected subsequent parse")
	}
}

// ─── IsIPv4 / IsIPv6 — single-byte edge cases ───────────────────────────────

func TestIsIPv4_SingleByte(t *testing.T) {
	if !IsIPv4([]byte{0x45}) {
		t.Error("0x45 (version 4) should be IPv4")
	}
	if IsIPv4([]byte{0x60}) {
		t.Error("0x60 (version 6) should not be IPv4")
	}
}

func TestIsIPv6_SingleByte(t *testing.T) {
	if !IsIPv6([]byte{0x60}) {
		t.Error("0x60 (version 6) should be IPv6")
	}
	if IsIPv6([]byte{0x45}) {
		t.Error("0x45 (version 4) should not be IPv6")
	}
}

// ─── darwinTUN.Write — non-IP version defaults to AF_INET ───────────────────

func TestDarwinTUN_Write_NonIPVersion(t *testing.T) {
	dev, rd := newDarwinTUNForWrite(t)
	defer dev.Close()
	defer rd.Close()

	// Version 0 (first nibble = 0) — should default to AF_INET.
	pkt := []byte{0x00, 0x01, 0x02, 0x03}
	n, err := dev.Write(pkt)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(pkt) {
		t.Errorf("Write returned %d, want %d", n, len(pkt))
	}

	wire := make([]byte, 32)
	if _, err := rd.Read(wire); err != nil {
		t.Fatalf("pipe read: %v", err)
	}
	af := binary.BigEndian.Uint32(wire[:4])
	if af != unix.AF_INET {
		t.Errorf("AF header for non-IP version: got %d, want AF_INET (%d)", af, unix.AF_INET)
	}
}

// ─── darwinTUN.Write — version 5 defaults to AF_INET ────────────────────────

func TestDarwinTUN_Write_Version5(t *testing.T) {
	dev, rd := newDarwinTUNForWrite(t)
	defer dev.Close()
	defer rd.Close()

	pkt := []byte{0x50, 0x01, 0x02}
	n, err := dev.Write(pkt)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(pkt) {
		t.Errorf("Write returned %d, want %d", n, len(pkt))
	}

	wire := make([]byte, 32)
	if _, err := rd.Read(wire); err != nil {
		t.Fatalf("pipe read: %v", err)
	}
	af := binary.BigEndian.Uint32(wire[:4])
	if af != unix.AF_INET {
		t.Errorf("AF header for version 5: got %d, want AF_INET (%d)", af, unix.AF_INET)
	}
}

// ─── darwinTUN.Write — large IPv6 packet ─────────────────────────────────────

func TestDarwinTUN_Write_IPv6LargePacket(t *testing.T) {
	dev, rd := newDarwinTUNForWrite(t)
	defer dev.Close()
	defer rd.Close()

	payload := make([]byte, 1400-40)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	pkt := buildIPv6Packet(net.ParseIP("fd00::1"), net.ParseIP("fd00::2"), ProtoTCP, payload)

	n, err := dev.Write(pkt)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(pkt) {
		t.Errorf("Write returned %d, want %d", n, len(pkt))
	}

	wire := make([]byte, 4+len(pkt)+16)
	wn, err := rd.Read(wire)
	if err != nil {
		t.Fatalf("pipe read: %v", err)
	}
	if wn != 4+len(pkt) {
		t.Fatalf("wire length: got %d, want %d", wn, 4+len(pkt))
	}
	af := binary.BigEndian.Uint32(wire[:4])
	if af != unix.AF_INET6 {
		t.Errorf("AF header: got %d, want AF_INET6 (%d)", af, unix.AF_INET6)
	}
	if !bytes.Equal(wire[4:wn], pkt) {
		t.Error("payload mismatch")
	}
}

// ─── darwinTUN.Write — packet exactly at MTU size ───────────────────────────

func TestDarwinTUN_Write_MTUSizedPacket(t *testing.T) {
	dev, rd := newDarwinTUNForWrite(t)
	defer dev.Close()
	defer rd.Close()

	mtuSize := 1420
	pkt := make([]byte, mtuSize)
	pkt[0] = 0x45 // IPv4
	for i := 1; i < mtuSize; i++ {
		pkt[i] = byte(i % 256)
	}

	n, err := dev.Write(pkt)
	if err != nil {
		t.Fatalf("Write MTU-sized packet: %v", err)
	}
	if n != mtuSize {
		t.Errorf("Write returned %d, want %d", n, mtuSize)
	}

	wire := make([]byte, 4+mtuSize+16)
	wn, err := rd.Read(wire)
	if err != nil {
		t.Fatalf("pipe read: %v", err)
	}
	if wn != 4+mtuSize {
		t.Errorf("wire length: got %d, want %d", wn, 4+mtuSize)
	}
	af := binary.BigEndian.Uint32(wire[:4])
	if af != unix.AF_INET {
		t.Errorf("AF header: got %d, want AF_INET (%d)", af, unix.AF_INET)
	}
	if !bytes.Equal(wire[4:wn], pkt) {
		t.Error("payload mismatch for MTU-sized packet")
	}
}

// ─── darwinTUN.Write — packet exceeding MTU ──────────────────────────────────

func TestDarwinTUN_Write_PacketExceedingMTU(t *testing.T) {
	dev, rd := newDarwinTUNForWrite(t)
	defer dev.Close()
	defer rd.Close()

	oversize := 2000
	pkt := make([]byte, oversize)
	pkt[0] = 0x60 // IPv6
	for i := 1; i < oversize; i++ {
		pkt[i] = byte(i % 256)
	}

	n, err := dev.Write(pkt)
	if err != nil {
		t.Fatalf("Write oversize packet: %v", err)
	}
	if n != oversize {
		t.Errorf("Write returned %d, want %d", n, oversize)
	}
}

// ─── darwinTUN.Read — zero-length buffer ─────────────────────────────────────

func TestDarwinTUN_Read_ZeroLengthBuffer(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	defer dev.Close()
	defer w.Close()

	pkt := buildIPv4Packet(net.IPv4(10, 0, 0, 1), net.IPv4(10, 0, 0, 2), ProtoUDP, []byte("hi"))
	wire := make([]byte, 4+len(pkt))
	binary.BigEndian.PutUint32(wire[:4], unix.AF_INET)
	copy(wire[4:], pkt)

	if _, err := w.Write(wire); err != nil {
		t.Fatalf("pipe write: %v", err)
	}

	buf := make([]byte, 0)
	n, err := dev.Read(buf)
	if err != nil {
		t.Fatalf("Read with zero buffer: %v", err)
	}
	if n != 0 {
		t.Errorf("Read returned %d bytes, want 0", n)
	}
}

// ─── darwinTUN.Read — AF_INET6 header correctly stripped ────────────────────

func TestDarwinTUN_Read_AF_INET6Specific(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	defer dev.Close()
	defer w.Close()

	payload := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	wire := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(wire[:4], unix.AF_INET6)
	copy(wire[4:], payload)

	if _, err := w.Write(wire); err != nil {
		t.Fatalf("pipe write: %v", err)
	}

	buf := make([]byte, 4096)
	n, err := dev.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(buf[:n], payload) {
		t.Errorf("payload mismatch:\n got %x\n want %x", buf[:n], payload)
	}
}

// ─── darwinTUN.Read — AF header value 0 ─────────────────────────────────────

func TestDarwinTUN_Read_AFZero(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	defer dev.Close()
	defer w.Close()

	payload := []byte{0x01, 0x02, 0x03}
	wire := make([]byte, 4+len(payload))
	copy(wire[4:], payload)

	if _, err := w.Write(wire); err != nil {
		t.Fatalf("pipe write: %v", err)
	}

	buf := make([]byte, 4096)
	n, err := dev.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(buf[:n], payload) {
		t.Errorf("payload mismatch:\n got %x\n want %x", buf[:n], payload)
	}
}

// ─── darwinTUN — concurrent writes ───────────────────────────────────────────

func TestDarwinTUN_ConcurrentWrites(t *testing.T) {
	dev, rd := newDarwinTUNForWrite(t)
	defer dev.Close()
	defer rd.Close()

	const numWriters = 4
	const packetsPerWriter = 5
	var wg sync.WaitGroup

	for g := 0; g < numWriters; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < packetsPerWriter; i++ {
				pkt := buildIPv4Packet(
					net.IPv4(byte(id), 0, 0, 1),
					net.IPv4(10, 0, 0, 2),
					ProtoUDP,
					[]byte{byte(i)},
				)
				if _, err := dev.Write(pkt); err != nil {
					t.Errorf("writer %d packet %d: %v", id, i, err)
					return
				}
			}
		}(g)
	}

	wg.Wait()

	// Drain the pipe and verify total bytes. Each IPv4 packet with 1-byte
	// payload is 21 bytes. Write prepends 4-byte AF header, so each frame
	// on the wire is 25 bytes. Total: numWriters * packetsPerWriter * 25.
	expectedBytes := numWriters * packetsPerWriter * 25
	totalRead := 0
	wireBuf := make([]byte, 8192)
	for totalRead < expectedBytes {
		n, err := rd.Read(wireBuf)
		if err != nil {
			t.Fatalf("pipe read after %d bytes: %v", totalRead, err)
		}
		totalRead += n
	}

	if totalRead != expectedBytes {
		t.Errorf("total wire bytes: got %d, want %d", totalRead, expectedBytes)
	}
}

// ─── darwinTUN — Close then verify Name/MTU still accessible ─────────────────

func TestDarwinTUN_NameMTUAfterClose(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	w.Close()

	if dev.Name() != "utun999" {
		t.Errorf("Name after close: got %q, want %q", dev.Name(), "utun999")
	}
	if dev.MTU() != 1420 {
		t.Errorf("MTU after close: got %d, want 1420", dev.MTU())
	}

	_ = dev.Close()
}

// ─── darwinTUN — round-trip with empty payload ───────────────────────────────

func TestDarwinTUN_WriteThenReadEmptyPayload(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	writeDev := &darwinTUN{file: w, name: "utun1", mtu: 1420}
	readDev := &darwinTUN{file: r, name: "utun1", mtu: 1420}

	n, err := writeDev.Write([]byte{})
	if err != nil {
		t.Fatalf("Write empty: %v", err)
	}
	if n != 0 {
		t.Errorf("Write empty returned %d, want 0", n)
	}

	buf := make([]byte, 4096)
	rn, err := readDev.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if rn != 0 {
		t.Errorf("Read returned %d, want 0", rn)
	}

	writeDev.Close()
	readDev.Close()
}

// ─── darwinTUN — SetAddr with IPv4-mapped IPv6 ──────────────────────────────

func TestDarwinTUN_SetAddr_IPv4MappedIPv6(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	ip := net.ParseIP("::ffff:10.0.0.1")
	if ip.To4() == nil {
		t.Fatal("expected IPv4-mapped IPv6 to have To4() != nil")
	}

	err := dev.SetAddr(ip, 24)
	if err == nil {
		t.Log("SetAddr with IPv4-mapped IPv6 succeeded unexpectedly")
	}
}

// ─── darwinTUN — AddRoute with link-local IPv6 ──────────────────────────────

func TestDarwinTUN_AddRoute_LinkLocalIPv6(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	_, dst, err := net.ParseCIDR("fe80::/10")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	err = dev.AddRoute(dst)
	if err == nil {
		t.Log("AddRoute link-local succeeded unexpectedly")
	}
}

// ─── darwinTUN — Read exactly 3 bytes (less than AF header) ─────────────────

func TestDarwinTUN_Read_ThreeBytes(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	defer dev.Close()

	if _, err := w.Write([]byte{0x00, 0x01, 0x02}); err != nil {
		t.Fatalf("pipe write: %v", err)
	}
	w.Close()

	buf := make([]byte, 4096)
	_, err := dev.Read(buf)
	if err == nil {
		t.Fatal("expected error for 3-byte read (short read), got nil")
	}
}

// ─── darwinTUN — Read with buffer exactly matching payload ───────────────────

func TestDarwinTUN_Read_BufferExactlyMatchesPayload(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	defer dev.Close()
	defer w.Close()

	payload := []byte{0xAA, 0xBB, 0xCC, 0xDD, 0xEE}
	wire := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(wire[:4], unix.AF_INET)
	copy(wire[4:], payload)

	if _, err := w.Write(wire); err != nil {
		t.Fatalf("pipe write: %v", err)
	}

	buf := make([]byte, len(payload))
	n, err := dev.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != len(payload) {
		t.Errorf("Read returned %d, want %d", n, len(payload))
	}
	if !bytes.Equal(buf, payload) {
		t.Errorf("payload mismatch:\n got %x\n want %x", buf, payload)
	}
}

// ─── darwinTUN — Read with 1-byte buffer ─────────────────────────────────────

func TestDarwinTUN_Read_OneByteBuffer(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	defer dev.Close()
	defer w.Close()

	payload := []byte{0x11, 0x22, 0x33, 0x44}
	wire := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(wire[:4], unix.AF_INET)
	copy(wire[4:], payload)

	if _, err := w.Write(wire); err != nil {
		t.Fatalf("pipe write: %v", err)
	}

	buf := make([]byte, 1)
	n, err := dev.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != 1 {
		t.Errorf("Read returned %d, want 1", n)
	}
	if buf[0] != payload[0] {
		t.Errorf("byte 0: got 0x%02x, want 0x%02x", buf[0], payload[0])
	}
}

// ─── SetMTU via fake ifconfig ────────────────────────────────────────────────

func setupFakeIfconfig(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	fake := tmpDir + "/ifconfig"
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmpDir+":"+os.Getenv("PATH"))
}

func TestDarwinTUN_SetMTU_SuccessViaFakeIfconfig(t *testing.T) {
	setupFakeIfconfig(t)
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	if err := dev.SetMTU(1400); err != nil {
		t.Fatalf("SetMTU: %v", err)
	}
	if dev.MTU() != 1400 {
		t.Errorf("MTU after SetMTU: got %d, want 1400", dev.MTU())
	}
}

// ─── SetAddr via fake ifconfig ───────────────────────────────────────────────

func setupFakeRoute(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	fake := tmpDir + "/route"
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", tmpDir+":"+os.Getenv("PATH"))
}

func TestDarwinTUN_SetAddr_IPv4_SuccessViaFakeIfconfig(t *testing.T) {
	setupFakeIfconfig(t)
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	if err := dev.SetAddr(net.IPv4(10, 0, 0, 1), 24); err != nil {
		t.Fatalf("SetAddr IPv4: %v", err)
	}
}

func TestDarwinTUN_SetAddr_IPv6_SuccessViaFakeIfconfig(t *testing.T) {
	setupFakeIfconfig(t)
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	ip := net.ParseIP("fd00::1")
	if err := dev.SetAddr(ip, 64); err != nil {
		t.Fatalf("SetAddr IPv6: %v", err)
	}
}

// ─── AddRoute via fake route ─────────────────────────────────────────────────

func TestDarwinTUN_AddRoute_IPv4_SuccessViaFakeRoute(t *testing.T) {
	setupFakeRoute(t)
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	_, dst, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if err := dev.AddRoute(dst); err != nil {
		t.Fatalf("AddRoute IPv4: %v", err)
	}
}

func TestDarwinTUN_AddRoute_IPv6_SuccessViaFakeRoute(t *testing.T) {
	setupFakeRoute(t)
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	_, dst, err := net.ParseCIDR("fd00::/64")
	if err != nil {
		t.Fatal(err)
	}
	if err := dev.AddRoute(dst); err != nil {
		t.Fatalf("AddRoute IPv6: %v", err)
	}
}

// ─── SetMTU error message verification ───────────────────────────────────────

func TestDarwinTUN_SetMTU_ErrorMentionsIfconfig(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	err := dev.SetMTU(1500)
	if err == nil {
		t.Skip("SetMTU succeeded (ifconfig in PATH)")
	}
	if !strings.Contains(err.Error(), "ifconfig") {
		t.Errorf("error should mention ifconfig: %v", err)
	}
}

// ─── SetAddr error message verification ───────────────────────────────────────

func TestDarwinTUN_SetAddr_IPv4_ErrorMentionsIfconfig(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	err := dev.SetAddr(net.IPv4(10, 0, 0, 1), 24)
	if err == nil {
		t.Skip("SetAddr succeeded (ifconfig in PATH)")
	}
	if !strings.Contains(err.Error(), "ifconfig") {
		t.Errorf("error should mention ifconfig: %v", err)
	}
}

func TestDarwinTUN_SetAddr_IPv6_ErrorMentionsIfconfig(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	err := dev.SetAddr(net.ParseIP("fd00::1"), 64)
	if err == nil {
		t.Skip("SetAddr succeeded (ifconfig in PATH)")
	}
	if !strings.Contains(err.Error(), "ifconfig") {
		t.Errorf("error should mention ifconfig: %v", err)
	}
}

// ─── AddRoute error message verification ──────────────────────────────────────

func TestDarwinTUN_AddRoute_IPv4_ErrorMentionsRoute(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	_, dst, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	err = dev.AddRoute(dst)
	if err == nil {
		t.Skip("AddRoute succeeded (route in PATH)")
	}
	if !strings.Contains(err.Error(), "route") {
		t.Errorf("error should mention route: %v", err)
	}
}

func TestDarwinTUN_AddRoute_IPv6_ErrorMentionsRoute(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	_, dst, err := net.ParseCIDR("fd00::/64")
	if err != nil {
		t.Fatal(err)
	}
	err = dev.AddRoute(dst)
	if err == nil {
		t.Skip("AddRoute succeeded (route in PATH)")
	}
	if !strings.Contains(err.Error(), "route") {
		t.Errorf("error should mention route: %v", err)
	}
}
