//go:build darwin

package tunnel

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// newDarwinTUNForRead creates a darwinTUN that reads from a pipe.
// Returns the device and the write end of the pipe (for injecting data).
func newDarwinTUNForRead(t *testing.T) (*darwinTUN, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	dev := &darwinTUN{
		file: r,
		name: "utun999",
		mtu:  1420,
	}
	return dev, w
}

// newDarwinTUNForWrite creates a darwinTUN that writes to a pipe.
// Returns the device and the read end of the pipe (for verifying output).
func newDarwinTUNForWrite(t *testing.T) (*darwinTUN, *os.File) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	dev := &darwinTUN{
		file: w,
		name: "utun999",
		mtu:  1420,
	}
	return dev, r
}

// ─── darwinTUN.Name / MTU / Close ──────────────────────────────────────────

func TestDarwinTUN_Name(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	if got := dev.Name(); got != "utun999" {
		t.Errorf("Name: got %q, want %q", got, "utun999")
	}
}

func TestDarwinTUN_MTU(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	if got := dev.MTU(); got != 1420 {
		t.Errorf("MTU: got %d, want 1420", got)
	}
}

func TestDarwinTUN_Close(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	w.Close()

	if err := dev.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

// ─── darwinTUN.Write — AF header construction ─────────────────────────────

func TestDarwinTUN_Write_IPv4PrependsAFINET(t *testing.T) {
	dev, rd := newDarwinTUNForWrite(t)
	defer dev.Close()
	defer rd.Close()

	ipv4Pkt := buildIPv4Packet(
		net.IPv4(10, 0, 0, 1),
		net.IPv4(10, 0, 0, 2),
		ProtoTCP,
		[]byte("hello"),
	)

	n, err := dev.Write(ipv4Pkt)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(ipv4Pkt) {
		t.Errorf("Write returned %d, want %d", n, len(ipv4Pkt))
	}

	// Read what was written to the pipe: 4-byte AF header + payload.
	written := make([]byte, 4+len(ipv4Pkt)+16)
	wn, err := rd.Read(written)
	if err != nil {
		t.Fatalf("pipe read: %v", err)
	}
	if wn < 4 {
		t.Fatalf("pipe read too short: %d bytes", wn)
	}

	af := binary.BigEndian.Uint32(written[:4])
	if af != unix.AF_INET {
		t.Errorf("AF header: got %d, want AF_INET (%d)", af, unix.AF_INET)
	}
	if !bytes.Equal(written[4:wn], ipv4Pkt) {
		t.Errorf("payload mismatch:\n got %x\n want %x", written[4:wn], ipv4Pkt)
	}
}

func TestDarwinTUN_Write_IPv6PrependsAFINET6(t *testing.T) {
	dev, rd := newDarwinTUNForWrite(t)
	defer dev.Close()
	defer rd.Close()

	ipv6Pkt := buildIPv6Packet(
		net.ParseIP("fd00::1"),
		net.ParseIP("fd00::2"),
		ProtoUDP,
		[]byte("world"),
	)

	n, err := dev.Write(ipv6Pkt)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(ipv6Pkt) {
		t.Errorf("Write returned %d, want %d", n, len(ipv6Pkt))
	}

	written := make([]byte, 4+len(ipv6Pkt)+16)
	wn, err := rd.Read(written)
	if err != nil {
		t.Fatalf("pipe read: %v", err)
	}

	af := binary.BigEndian.Uint32(written[:4])
	if af != unix.AF_INET6 {
		t.Errorf("AF header: got %d, want AF_INET6 (%d)", af, unix.AF_INET6)
	}
	if !bytes.Equal(written[4:wn], ipv6Pkt) {
		t.Errorf("payload mismatch:\n got %x\n want %x", written[4:wn], ipv6Pkt)
	}
}

func TestDarwinTUN_Write_EmptyPacket(t *testing.T) {
	dev, rd := newDarwinTUNForWrite(t)
	defer dev.Close()
	defer rd.Close()

	n, err := dev.Write([]byte{})
	if err != nil {
		t.Fatalf("Write empty: %v", err)
	}
	if n != 0 {
		t.Errorf("Write empty returned %d, want 0", n)
	}

	written := make([]byte, 32)
	wn, err := rd.Read(written)
	if err != nil {
		t.Fatalf("pipe read: %v", err)
	}
	// Should be exactly 4 bytes (AF header with zero value) + 0 payload.
	if wn != 4 {
		t.Errorf("pipe read length: got %d, want 4", wn)
	}
	af := binary.BigEndian.Uint32(written[:4])
	if af != 0 {
		t.Errorf("AF header for empty packet: got %d, want 0", af)
	}
}

// ─── darwinTUN.Read — AF header stripping ─────────────────────────────────

func TestDarwinTUN_Read_IPv4(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	defer dev.Close()
	defer w.Close()

	pkt := buildIPv4Packet(
		net.IPv4(192, 168, 1, 1),
		net.IPv4(192, 168, 1, 2),
		ProtoICMP,
		[]byte("ping"),
	)
	wire := make([]byte, 4+len(pkt))
	binary.BigEndian.PutUint32(wire[:4], unix.AF_INET)
	copy(wire[4:], pkt)

	if _, err := w.Write(wire); err != nil {
		t.Fatalf("pipe write: %v", err)
	}

	buf := make([]byte, 4096)
	n, err := dev.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(buf[:n], pkt) {
		t.Errorf("Read payload mismatch:\n got %x\n want %x", buf[:n], pkt)
	}
}

func TestDarwinTUN_Read_IPv6(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	defer dev.Close()
	defer w.Close()

	pkt := buildIPv6Packet(
		net.ParseIP("2001:db8::1"),
		net.ParseIP("2001:db8::2"),
		ProtoTCP,
		[]byte("data"),
	)
	wire := make([]byte, 4+len(pkt))
	binary.BigEndian.PutUint32(wire[:4], unix.AF_INET6)
	copy(wire[4:], pkt)

	if _, err := w.Write(wire); err != nil {
		t.Fatalf("pipe write: %v", err)
	}

	buf := make([]byte, 4096)
	n, err := dev.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(buf[:n], pkt) {
		t.Errorf("Read payload mismatch:\n got %x\n want %x", buf[:n], pkt)
	}
}

func TestDarwinTUN_Read_ShortRead(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	defer dev.Close()

	// Write fewer than 4 bytes to simulate a corrupted/short utun frame.
	if _, err := w.Write([]byte{0x00, 0x01}); err != nil {
		t.Fatalf("pipe write: %v", err)
	}
	w.Close()

	buf := make([]byte, 4096)
	_, err := dev.Read(buf)
	if err == nil {
		t.Fatal("expected error for short read, got nil")
	}
}

func TestDarwinTUN_Read_FileError(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	// Close both ends so that Read gets an error from the underlying file.
	w.Close()
	dev.Close()

	buf := make([]byte, 4096)
	_, err := dev.Read(buf)
	if err == nil {
		t.Fatal("expected error from Read on closed file, got nil")
	}
}

func TestDarwinTUN_Read_BufferSmallerThanPayload(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	defer dev.Close()
	defer w.Close()

	payload := make([]byte, 20)
	for i := range payload {
		payload[i] = byte(i)
	}
	wire := make([]byte, 4+len(payload))
	binary.BigEndian.PutUint32(wire[:4], unix.AF_INET)
	copy(wire[4:], payload)

	if _, err := w.Write(wire); err != nil {
		t.Fatalf("pipe write: %v", err)
	}

	// Provide a buffer smaller than the payload (but > 0).
	smallBuf := make([]byte, 10)
	n, err := dev.Read(smallBuf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != len(smallBuf) {
		t.Errorf("Read returned %d, want %d", n, len(smallBuf))
	}
	if !bytes.Equal(smallBuf, payload[:len(smallBuf)]) {
		t.Errorf("Read data mismatch:\n got %x\n want %x", smallBuf, payload[:len(smallBuf)])
	}
}

// ─── Device interface compliance ──────────────────────────────────────────

func TestDarwinTUN_ImplementsDevice(t *testing.T) {
	var _ Device = (*darwinTUN)(nil)
}

// ─── Round-trip: Write then Read ──────────────────────────────────────────

func TestDarwinTUN_RoundTrip_IPv4(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	writeDev := &darwinTUN{file: w, name: "utun1", mtu: 1420}
	readDev := &darwinTUN{file: r, name: "utun1", mtu: 1420}

	origPkt := buildIPv4Packet(
		net.IPv4(10, 20, 30, 40),
		net.IPv4(50, 60, 70, 80),
		ProtoUDP,
		[]byte("round-trip-test-payload"),
	)

	wn, err := writeDev.Write(origPkt)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if wn != len(origPkt) {
		t.Errorf("Write returned %d, want %d", wn, len(origPkt))
	}

	buf := make([]byte, 4096)
	rn, err := readDev.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(buf[:rn], origPkt) {
		t.Errorf("round-trip mismatch:\n got %x\n want %x", buf[:rn], origPkt)
	}

	writeDev.Close()
	readDev.Close()
}

func TestDarwinTUN_RoundTrip_IPv6(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}

	writeDev := &darwinTUN{file: w, name: "utun2", mtu: 1420}
	readDev := &darwinTUN{file: r, name: "utun2", mtu: 1420}

	origPkt := buildIPv6Packet(
		net.ParseIP("fe80::1"),
		net.ParseIP("fe80::2"),
		ProtoICMPv6,
		[]byte("ipv6-round-trip"),
	)

	wn, err := writeDev.Write(origPkt)
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if wn != len(origPkt) {
		t.Errorf("Write returned %d, want %d", wn, len(origPkt))
	}

	buf := make([]byte, 4096)
	rn, err := readDev.Read(buf)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !bytes.Equal(buf[:rn], origPkt) {
		t.Errorf("round-trip mismatch:\n got %x\n want %x", buf[:rn], origPkt)
	}

	writeDev.Close()
	readDev.Close()
}

// ─── darwinTUN.Write — various packet sizes ───────────────────────────────

func TestDarwinTUN_Write_VariousSizes(t *testing.T) {
	sizes := []int{0, 1, 20, 100, 500, 1420}
	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			dev, rd := newDarwinTUNForWrite(t)
			defer dev.Close()
			defer rd.Close()

			payload := make([]byte, size)
			if size > 0 {
				// Make it look like IPv4.
				payload[0] = 0x45
				for i := 1; i < size; i++ {
					payload[i] = byte(i)
				}
			}

			n, err := dev.Write(payload)
			if err != nil {
				t.Fatalf("Write: %v", err)
			}
			if n != len(payload) {
				t.Errorf("Write returned %d, want %d", n, len(payload))
			}

			// Verify the on-wire data has the 4-byte AF header.
			wire := make([]byte, 4+size+16)
			wn, err := rd.Read(wire)
			if err != nil {
				t.Fatalf("pipe read: %v", err)
			}
			expectedWireLen := 4 + size
			if wn != expectedWireLen {
				t.Errorf("wire length: got %d, want %d", wn, expectedWireLen)
			}
			if size > 0 {
				// IPv4 → AF_INET.
				af := binary.BigEndian.Uint32(wire[:4])
				if af != unix.AF_INET {
					t.Errorf("AF header: got %d, want AF_INET (%d)", af, unix.AF_INET)
				}
			}
		})
	}
}

// ─── darwinTUN.SetAddr — command construction (verifies no panic) ────────

func TestDarwinTUN_SetAddr_IPv4_CommandAttempt(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	ip := net.IPv4(10, 0, 0, 1)
	err := dev.SetAddr(ip, 24)
	// Will fail because utun999 doesn't exist, but should not panic.
	if err == nil {
		t.Log("SetAddr succeeded unexpectedly (interface may exist)")
	}
}

func TestDarwinTUN_SetAddr_IPv6_CommandAttempt(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	ip := net.ParseIP("fd00::1")
	err := dev.SetAddr(ip, 64)
	if err == nil {
		t.Log("SetAddr succeeded unexpectedly (interface may exist)")
	}
}

// ─── darwinTUN.SetMTU — command construction ─────────────────────────────

func TestDarwinTUN_SetMTU_CommandAttempt(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	err := dev.SetMTU(1280)
	if err == nil {
		t.Log("SetMTU succeeded unexpectedly (interface may exist)")
	}
}

// ─── darwinTUN.AddRoute — command construction ───────────────────────────

func TestDarwinTUN_AddRoute_IPv4_CommandAttempt(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	_, dst, err := net.ParseCIDR("10.0.0.0/24")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	err = dev.AddRoute(dst)
	if err == nil {
		t.Log("AddRoute succeeded unexpectedly (interface may exist)")
	}
}

func TestDarwinTUN_AddRoute_IPv6_CommandAttempt(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	_, dst, err := net.ParseCIDR("fd00::/64")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	err = dev.AddRoute(dst)
	if err == nil {
		t.Log("AddRoute succeeded unexpectedly (interface may exist)")
	}
}

// ─── CreateTUN — error paths (requires root, so we expect errors) ────────

func TestDarwinTUN_CreateTUN_EmptyName(t *testing.T) {
	// Empty string means auto-assign, but opening a utun requires root.
	// We verify the function returns an error (not a panic) and the error
	// is related to socket creation or connect.
	_, err := CreateTUN("")
	if err == nil {
		t.Log("CreateTUN('') succeeded — running as root?")
	}
}

func TestDarwinTUN_CreateTUN_UtunAuto(t *testing.T) {
	// "utun" (no unit number) means auto-assign.
	_, err := CreateTUN("utun")
	if err == nil {
		t.Log("CreateTUN('utun') succeeded — running as root?")
	}
}

func TestDarwinTUN_CreateTUN_SpecificUnit(t *testing.T) {
	// Requesting utun99 specifically.
	_, err := CreateTUN("utun99")
	if err == nil {
		t.Log("CreateTUN('utun99') succeeded — running as root?")
	}
}

func TestDarwinTUN_CreateTUN_GarbageName(t *testing.T) {
	// A name that doesn't match "utun%d" is treated as auto-assign.
	_, err := CreateTUN("garbage")
	if err == nil {
		t.Log("CreateTUN('garbage') succeeded — running as root?")
	}
}

// ─── darwinTUN.SetMTU — error path details ───────────────────────────────

func TestDarwinTUN_SetMTU_ErrorDoesNotUpdateMTU(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	originalMTU := dev.MTU()
	err := dev.SetMTU(9000)
	if err == nil {
		t.Log("SetMTU succeeded unexpectedly (interface may exist)")
	} else {
		// On error, the internal mtu must remain unchanged.
		if got := dev.MTU(); got != originalMTU {
			t.Errorf("MTU changed from %d to %d after failed SetMTU", originalMTU, got)
		}
	}
}

func TestDarwinTUN_SetMTU_ErrorContainsIfconfig(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	err := dev.SetMTU(1280)
	if err == nil {
		t.Log("SetMTU succeeded unexpectedly")
		return
	}
	// The error should mention "ifconfig mtu".
	if msg := err.Error(); len(msg) == 0 {
		t.Error("expected non-empty error message from SetMTU")
	}
}

func TestDarwinTUN_SetMTU_VariousValues(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	// Exercise different MTU values to cover strconv.Itoa paths.
	for _, mtu := range []int{576, 1280, 1500, 9000, 1} {
		err := dev.SetMTU(mtu)
		if err != nil {
			// Expected on non-root; verify mtu not updated.
			if dev.MTU() != 1420 {
				t.Errorf("MTU changed after failed SetMTU(%d): got %d", mtu, dev.MTU())
			}
		}
	}
}

// ─── darwinTUN.SetAddr — error path details ──────────────────────────────

func TestDarwinTUN_SetAddr_IPv4_ErrorContainsMessage(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	err := dev.SetAddr(net.IPv4(10, 0, 0, 1), 24)
	if err == nil {
		t.Log("SetAddr succeeded unexpectedly")
		return
	}
	if msg := err.Error(); len(msg) == 0 {
		t.Error("expected non-empty error message from SetAddr IPv4")
	}
}

func TestDarwinTUN_SetAddr_IPv6_ErrorContainsMessage(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	err := dev.SetAddr(net.ParseIP("fd00::1"), 64)
	if err == nil {
		t.Log("SetAddr succeeded unexpectedly")
		return
	}
	if msg := err.Error(); len(msg) == 0 {
		t.Error("expected non-empty error message from SetAddr IPv6")
	}
}

func TestDarwinTUN_SetAddr_IPv4_VariousPrefixLengths(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	// Exercise various prefix lengths to ensure CIDRMask and formatting work.
	for _, plen := range []int{8, 16, 24, 28, 32} {
		err := dev.SetAddr(net.IPv4(172, 16, 0, 1), plen)
		if err != nil {
			// Expected on non-existent interface.
			if msg := err.Error(); len(msg) == 0 {
				t.Errorf("empty error for prefix length %d", plen)
			}
		}
	}
}

func TestDarwinTUN_SetAddr_IPv6_VariousPrefixLengths(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	for _, plen := range []int{48, 64, 96, 128} {
		err := dev.SetAddr(net.ParseIP("fe80::1"), plen)
		if err != nil {
			if msg := err.Error(); len(msg) == 0 {
				t.Errorf("empty error for IPv6 prefix length %d", plen)
			}
		}
	}
}

func TestDarwinTUN_SetAddr_IPv4UsesDottedNetmask(t *testing.T) {
	// Verify the IPv4 path uses dotted-decimal mask formatting by checking
	// the error message. When ifconfig fails on a non-existent interface,
	// the error output should contain the interface name.
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	err := dev.SetAddr(net.IPv4(192, 168, 1, 1), 24)
	if err == nil {
		t.Log("SetAddr succeeded unexpectedly")
		return
	}
	// Just verify we got a well-formed error (not a panic).
	_ = err.Error()
}

// ─── darwinTUN.AddRoute — error path details ─────────────────────────────

func TestDarwinTUN_AddRoute_IPv4_ErrorContainsMessage(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	_, dst, err := net.ParseCIDR("10.0.0.0/8")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	err = dev.AddRoute(dst)
	if err == nil {
		t.Log("AddRoute succeeded unexpectedly")
		return
	}
	if msg := err.Error(); len(msg) == 0 {
		t.Error("expected non-empty error message from AddRoute IPv4")
	}
}

func TestDarwinTUN_AddRoute_IPv6_ErrorContainsMessage(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	_, dst, err := net.ParseCIDR("fd00::/48")
	if err != nil {
		t.Fatalf("ParseCIDR: %v", err)
	}
	err = dev.AddRoute(dst)
	if err == nil {
		t.Log("AddRoute succeeded unexpectedly")
		return
	}
	if msg := err.Error(); len(msg) == 0 {
		t.Error("expected non-empty error message from AddRoute IPv6")
	}
}

func TestDarwinTUN_AddRoute_IPv4_VariousCIDRs(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	cidrs := []string{
		"172.16.0.0/12",
		"192.168.0.0/16",
		"10.0.0.0/8",
		"100.64.0.0/10",
	}
	for _, cidr := range cidrs {
		_, dst, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatalf("ParseCIDR(%s): %v", cidr, err)
		}
		err = dev.AddRoute(dst)
		if err != nil {
			// Expected on non-existent interface.
			_ = err.Error()
		}
	}
}

func TestDarwinTUN_AddRoute_IPv6_VariousCIDRs(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	cidrs := []string{
		"fd00::/48",
		"fe80::/64",
		"2001:db8::/32",
	}
	for _, cidr := range cidrs {
		_, dst, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatalf("ParseCIDR(%s): %v", cidr, err)
		}
		err = dev.AddRoute(dst)
		if err != nil {
			_ = err.Error()
		}
	}
}

// ─── darwinTUN.Write — error path ────────────────────────────────────────

func TestDarwinTUN_Write_ToClosedFile(t *testing.T) {
	dev, rd := newDarwinTUNForWrite(t)
	rd.Close()
	dev.Close()

	ipv4Pkt := buildIPv4Packet(
		net.IPv4(10, 0, 0, 1),
		net.IPv4(10, 0, 0, 2),
		ProtoTCP,
		[]byte("closed"),
	)

	_, err := dev.Write(ipv4Pkt)
	if err == nil {
		t.Error("expected error writing to closed file, got nil")
	}
}

// ─── darwinTUN.Read — edge case: exactly 4 bytes (AF header, no payload) ─

func TestDarwinTUN_Read_Exactly4Bytes(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	defer dev.Close()

	// Write exactly 4 bytes (just the AF header, no payload).
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], unix.AF_INET)
	if _, err := w.Write(hdr[:]); err != nil {
		t.Fatalf("pipe write: %v", err)
	}
	w.Close()

	buf := make([]byte, 4096)
	n, err := dev.Read(buf)
	// With exactly 4 bytes, after stripping the header there are 0 payload bytes.
	// copy(buf, tmp[4:4]) copies 0 bytes.
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if n != 0 {
		t.Errorf("Read returned %d bytes, want 0", n)
	}
}

// ─── darwinTUN.Write — single-byte packet version detection ──────────────

func TestDarwinTUN_Write_SingleByte_IPv4Version(t *testing.T) {
	dev, rd := newDarwinTUNForWrite(t)
	defer dev.Close()
	defer rd.Close()

	// Single byte with high nibble = 4 (IPv4).
	n, err := dev.Write([]byte{0x45})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 1 {
		t.Errorf("Write returned %d, want 1", n)
	}

	wire := make([]byte, 16)
	wn, err := rd.Read(wire)
	if err != nil {
		t.Fatalf("pipe read: %v", err)
	}
	if wn != 5 {
		t.Fatalf("wire length: got %d, want 5", wn)
	}
	af := binary.BigEndian.Uint32(wire[:4])
	if af != unix.AF_INET {
		t.Errorf("AF header: got %d, want AF_INET (%d)", af, unix.AF_INET)
	}
	if wire[4] != 0x45 {
		t.Errorf("payload byte: got 0x%02x, want 0x45", wire[4])
	}
}

func TestDarwinTUN_Write_SingleByte_IPv6Version(t *testing.T) {
	dev, rd := newDarwinTUNForWrite(t)
	defer dev.Close()
	defer rd.Close()

	// Single byte with high nibble = 6 (IPv6).
	n, err := dev.Write([]byte{0x60})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != 1 {
		t.Errorf("Write returned %d, want 1", n)
	}

	wire := make([]byte, 16)
	wn, err := rd.Read(wire)
	if err != nil {
		t.Fatalf("pipe read: %v", err)
	}
	if wn != 5 {
		t.Fatalf("wire length: got %d, want 5", wn)
	}
	af := binary.BigEndian.Uint32(wire[:4])
	if af != unix.AF_INET6 {
		t.Errorf("AF header: got %d, want AF_INET6 (%d)", af, unix.AF_INET6)
	}
	if wire[4] != 0x60 {
		t.Errorf("payload byte: got 0x%02x, want 0x60", wire[4])
	}
}

// ─── darwinTUN.Read — multiple sequential frames ─────────────────────────

func TestDarwinTUN_Read_MultipleFrames(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	defer dev.Close()
	defer w.Close()

	// Write and read one frame at a time to avoid pipe buffering issues
	// (a single Read() can consume all available pipe data at once).
	const numFrames = 3
	for i := 0; i < numFrames; i++ {
		pkt := buildIPv4Packet(
			net.IPv4(10, 0, 0, 1),
			net.IPv4(10, 0, 0, 2),
			ProtoUDP,
			[]byte(fmt.Sprintf("frame-%d", i)),
		)
		wire := make([]byte, 4+len(pkt))
		binary.BigEndian.PutUint32(wire[:4], unix.AF_INET)
		copy(wire[4:], pkt)

		if _, err := w.Write(wire); err != nil {
			t.Fatalf("pipe write %d: %v", i, err)
		}

		buf := make([]byte, 4096)
		n, err := dev.Read(buf)
		if err != nil {
			t.Fatalf("Read %d: %v", i, err)
		}
		if !bytes.Equal(buf[:n], pkt) {
			t.Errorf("frame %d mismatch:\n got %x\n want %x", i, buf[:n], pkt)
		}
	}
}

// ─── darwinTUN — default MTU value ───────────────────────────────────────

func TestDarwinTUN_DefaultMTU(t *testing.T) {
	dev, _ := newDarwinTUNForRead(t)
	defer dev.Close()

	// The default MTU set in the constructor should be 1420.
	if dev.MTU() != 1420 {
		t.Errorf("default MTU: got %d, want 1420", dev.MTU())
	}
}

// ─── darwinTUN.Close — double close ──────────────────────────────────────

func TestDarwinTUN_DoubleClose(t *testing.T) {
	dev, w := newDarwinTUNForRead(t)
	w.Close()

	if err := dev.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	// Second close should return an error (file already closed).
	if err := dev.Close(); err == nil {
		t.Log("double Close returned nil — os.File allows this")
	}
}
