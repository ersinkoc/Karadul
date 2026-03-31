package firewall

import (
	"runtime"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// AllowPort — invalid protocol
// ---------------------------------------------------------------------------

func TestAllowPort_InvalidProtocol(t *testing.T) {
	protocols := []struct {
		name     string
		protocol string
	}{
		{"icmp", "icmp"},
		{"empty", ""},
		{"sctp", "sctp"},
		{"http", "http"},
	}
	for _, tt := range protocols {
		t.Run(tt.name, func(t *testing.T) {
			err := AllowPort(80, tt.protocol)
			if err == nil {
				t.Errorf("AllowPort(80, %q) should fail with invalid protocol", tt.protocol)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RemovePort — invalid protocol
// ---------------------------------------------------------------------------

func TestRemovePort_InvalidProtocol(t *testing.T) {
	protocols := []struct {
		name     string
		protocol string
	}{
		{"icmp", "icmp"},
		{"empty", ""},
		{"sctp", "sctp"},
	}
	for _, tt := range protocols {
		t.Run(tt.name, func(t *testing.T) {
			err := RemovePort(80, tt.protocol)
			if err == nil {
				t.Errorf("RemovePort(80, %q) should fail with invalid protocol", tt.protocol)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AllowPort — valid lowercase protocols must not fail on validation
// ---------------------------------------------------------------------------
// On BSD the function always returns "not implemented", which is acceptable.
// On darwin/linux/windows the call may fail on system commands (no root), but
// the error must NOT be the protocol-validation error.

func TestAllowPort_ValidProtocols(t *testing.T) {
	for _, proto := range []string{"tcp", "udp"} {
		t.Run(proto, func(t *testing.T) {
			err := AllowPort(80, proto)
			if err == nil {
				// Succeeded (unlikely without root, but fine).
				return
			}
			msg := err.Error()
			if strings.Contains(msg, "unsupported protocol") {
				t.Errorf("AllowPort(80, %q) should not fail on protocol validation: %v", proto, err)
			}
			if strings.Contains(msg, "protocol must be 'tcp' or 'udp'") {
				t.Errorf("AllowPort(80, %q) should not fail on protocol validation: %v", proto, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// RemovePort — valid lowercase protocols must not fail on validation
// ---------------------------------------------------------------------------

func TestRemovePort_ValidProtocols(t *testing.T) {
	for _, proto := range []string{"tcp", "udp"} {
		t.Run(proto, func(t *testing.T) {
			err := RemovePort(80, proto)
			if err == nil {
				return
			}
			msg := err.Error()
			if strings.Contains(msg, "unsupported protocol") {
				t.Errorf("RemovePort(80, %q) should not fail on protocol validation: %v", proto, err)
			}
			if strings.Contains(msg, "protocol must be 'tcp' or 'udp'") {
				t.Errorf("RemovePort(80, %q) should not fail on protocol validation: %v", proto, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AllowPort — uppercase protocols: platform-dependent behaviour
// ---------------------------------------------------------------------------
// Darwin and Linux lowercase the protocol internally, so "TCP"/"UDP" pass
// validation.  Windows does NOT lowercase, so "TCP"/"UDP" fail.  BSD always
// returns "not implemented".  We only assert the protocol-validation check
// matches the documented behaviour per platform.

func TestAllowPort_UppercaseProtocol(t *testing.T) {
	for _, proto := range []string{"TCP", "UDP"} {
		t.Run(proto, func(t *testing.T) {
			err := AllowPort(80, proto)
			if err == nil {
				// Accepted — expected on darwin/linux where protocol is lowercased.
				return
			}
			msg := err.Error()

			switch runtime.GOOS {
			case "darwin", "linux":
				// Should NOT be a protocol-validation error.
				if strings.Contains(msg, "unsupported protocol") {
					t.Errorf("AllowPort(80, %q) on %s should not fail on protocol validation: %v", proto, runtime.GOOS, err)
				}
			case "windows":
				// Windows does not lowercase — uppercase MUST fail validation.
				if !strings.Contains(msg, "protocol must be 'tcp' or 'udp'") {
					t.Errorf("AllowPort(80, %q) on Windows should fail with protocol validation error, got: %v", proto, err)
				}
			default:
				// BSD: returns "not implemented" which is fine — not a validation error.
				if strings.Contains(msg, "unsupported protocol") {
					t.Errorf("AllowPort(80, %q) on %s should not fail with unsupported-protocol error: %v", proto, runtime.GOOS, err)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AllowPort — port 0 edge case
// ---------------------------------------------------------------------------
// Port 0 is technically invalid for firewall rules.  We verify the call
// does not panic; the error (if any) depends on the OS backend.

func TestAllowPort_PortZero(t *testing.T) {
	// This should not panic regardless of platform.
	_ = AllowPort(0, "tcp")
}

// ---------------------------------------------------------------------------
// RemovePort — port 0 edge case
// ---------------------------------------------------------------------------

func TestRemovePort_PortZero(t *testing.T) {
	_ = RemovePort(0, "tcp")
}
