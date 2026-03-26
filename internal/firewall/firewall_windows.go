//go:build windows

package firewall

import (
	"fmt"
	"os/exec"
	"strings"
)

const (
	firewallRuleName = "Karadul VPN"
	firewallDesc     = "Karadul mesh VPN - Allow traffic"
)

// Setup adds Windows Firewall rules to allow Karadul traffic.
// This allows the application to receive and send VPN traffic without interference.
func Setup(exePath string) error {
	if exePath == "" {
		// Try to get the current executable path
		var err error
		exePath, err = exec.LookPath("karadul")
		if err != nil {
			// Fallback: try to find in common locations
			exePath = "karadul.exe"
		}
	}

	// Remove any existing rules first to avoid duplicates
	Remove()

	// Add inbound rule
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s Inbound", firewallRuleName),
		"dir=in",
		"action=allow",
		"program="+exePath,
		"enable=yes",
		fmt.Sprintf("description=%s Inbound", firewallDesc),
		"profile=any")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add inbound firewall rule: %w: %s", err, out)
	}

	// Add outbound rule
	cmd = exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		fmt.Sprintf("name=%s Outbound", firewallRuleName),
		"dir=out",
		"action=allow",
		"program="+exePath,
		"enable=yes",
		fmt.Sprintf("description=%s Outbound", firewallDesc),
		"profile=any")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add outbound firewall rule: %w: %s", err, out)
	}

	// Add rules for UDP/TCP ports commonly used by Karadul (if needed for specific ports)
	// WireGuard typically uses UDP, but we allow the program specifically above

	return nil
}

// Remove deletes the Windows Firewall rules created by Setup.
func Remove() error {
	var lastErr error

	// Remove inbound rule (ignore errors - rule might not exist)
	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf("name=%s Inbound", firewallRuleName))
	if out, err := cmd.CombinedOutput(); err != nil {
		// Only record error if it's not "no rules match"
		if !strings.Contains(string(out), "No rules match") {
			lastErr = fmt.Errorf("failed to remove inbound firewall rule: %w", err)
		}
	}

	// Remove outbound rule (ignore errors - rule might not exist)
	cmd = exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		fmt.Sprintf("name=%s Outbound", firewallRuleName))
	if out, err := cmd.CombinedOutput(); err != nil {
		// Only record error if it's not "no rules match"
		if !strings.Contains(string(out), "No rules match") {
			lastErr = fmt.Errorf("failed to remove outbound firewall rule: %w", err)
		}
	}

	return lastErr
}

// Check returns true if the firewall rules are configured
func Check() bool {
	// Check if inbound rule exists
	cmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule",
		fmt.Sprintf("name=%s Inbound", firewallRuleName))
	out, err := cmd.CombinedOutput()
	if err != nil || strings.Contains(string(out), "No rules match") {
		return false
	}

	// Check if outbound rule exists
	cmd = exec.Command("netsh", "advfirewall", "firewall", "show", "rule",
		fmt.Sprintf("name=%s Outbound", firewallRuleName))
	out, err = cmd.CombinedOutput()
	if err != nil || strings.Contains(string(out), "No rules match") {
		return false
	}

	return true
}

// AllowPort adds a temporary firewall rule for a specific port.
// Useful for allowing relay server ports or custom configurations.
func AllowPort(port int, protocol string) error {
	if protocol != "tcp" && protocol != "udp" {
		return fmt.Errorf("protocol must be 'tcp' or 'udp', got: %s", protocol)
	}

	ruleName := fmt.Sprintf("%s Port %d/%s", firewallRuleName, port, protocol)

	// Remove existing rule for this port first
	exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		"name="+ruleName).Run()

	// Add inbound rule
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+ruleName,
		"dir=in",
		"action=allow",
		fmt.Sprintf("protocol=%s", protocol),
		fmt.Sprintf("localport=%d", port),
		"profile=any")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add port rule: %w: %s", err, out)
	}

	return nil
}

// RemovePort removes a port-specific firewall rule
func RemovePort(port int, protocol string) error {
	ruleName := fmt.Sprintf("%s Port %d/%s", firewallRuleName, port, protocol)
	cmd := exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		"name="+ruleName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to remove port rule: %w: %s", err, out)
	}
	return nil
}
