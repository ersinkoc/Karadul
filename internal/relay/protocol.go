// Package relay implements the DERP (Designated Encrypted Relay for Packets) protocol.
// DERP frames are exchanged over a persistent TCP connection with an HTTP upgrade.
package relay

import (
	"encoding/binary"
	"fmt"
	"io"
)

// FrameType identifies the type of a DERP frame.
type FrameType uint8

const (
	FrameClientInfo  FrameType = 0x01 // client sends its public key
	FrameSendPacket  FrameType = 0x02 // client → relay: dest pubkey + payload
	FrameRecvPacket  FrameType = 0x03 // relay → client: src pubkey + payload
	FramePeerPresent FrameType = 0x04 // peer connected
	FramePeerGone    FrameType = 0x05 // peer disconnected
	FramePing        FrameType = 0x06 // keepalive ping
	FramePong        FrameType = 0x07 // keepalive pong
	FrameError       FrameType = 0x08 // server → client: error message (UTF-8)
)

const (
	// frameHeaderSize: type(1) + length(4) = 5 bytes
	frameHeaderSize = 5

	// pubKeySize is the wire size of a public key.
	pubKeySize = 32

	// maxFrameSize limits the payload of a single frame.
	maxFrameSize = 64 * 1024
)

// Frame is a decoded DERP protocol frame.
type Frame struct {
	Type    FrameType
	Payload []byte
}

// WriteFrame encodes and writes a frame to w.
func WriteFrame(w io.Writer, ft FrameType, payload []byte) error {
	hdr := [frameHeaderSize]byte{}
	hdr[0] = byte(ft)
	binary.BigEndian.PutUint32(hdr[1:], uint32(len(payload)))
	if _, err := w.Write(hdr[:]); err != nil {
		return fmt.Errorf("write frame header: %w", err)
	}
	if len(payload) > 0 {
		if _, err := w.Write(payload); err != nil {
			return fmt.Errorf("write frame payload: %w", err)
		}
	}
	return nil
}

// ReadFrame reads the next frame from r.
func ReadFrame(r io.Reader) (*Frame, error) {
	var hdr [frameHeaderSize]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, fmt.Errorf("read frame header: %w", err)
	}
	ft := FrameType(hdr[0])
	length := int(binary.BigEndian.Uint32(hdr[1:]))
	if length > maxFrameSize {
		return nil, fmt.Errorf("frame too large: %d", length)
	}
	payload := make([]byte, length)
	if length > 0 {
		if _, err := io.ReadFull(r, payload); err != nil {
			return nil, fmt.Errorf("read frame payload: %w", err)
		}
	}
	return &Frame{Type: ft, Payload: payload}, nil
}

// BuildClientInfo builds the ClientInfo frame payload: pubkey(32).
func BuildClientInfo(pubKey [32]byte) []byte {
	return pubKey[:]
}

// BuildSendPacket builds a SendPacket frame payload: dstPubKey(32) + payload.
func BuildSendPacket(dstPubKey [32]byte, payload []byte) []byte {
	buf := make([]byte, 32+len(payload))
	copy(buf, dstPubKey[:])
	copy(buf[32:], payload)
	return buf
}

// ParseSendPacket parses a SendPacket frame payload.
func ParseSendPacket(payload []byte) (dst [32]byte, pkt []byte, err error) {
	if len(payload) < 32 {
		return dst, nil, fmt.Errorf("send packet too short")
	}
	copy(dst[:], payload[:32])
	return dst, payload[32:], nil
}

// BuildRecvPacket builds a RecvPacket frame: srcPubKey(32) + payload.
func BuildRecvPacket(srcPubKey [32]byte, payload []byte) []byte {
	buf := make([]byte, 32+len(payload))
	copy(buf, srcPubKey[:])
	copy(buf[32:], payload)
	return buf
}

// ParseRecvPacket parses a RecvPacket frame.
func ParseRecvPacket(payload []byte) (src [32]byte, pkt []byte, err error) {
	if len(payload) < 32 {
		return src, nil, fmt.Errorf("recv packet too short")
	}
	copy(src[:], payload[:32])
	return src, payload[32:], nil
}
