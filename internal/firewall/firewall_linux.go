//go:build linux

package firewall

import "fmt"

// Setup configures firewall rules on Linux using iptables/nftables.
// This is a placeholder for future implementation.
func Setup(exePath string) error {
	// On Linux, firewall rules are typically managed by the administrator
	// or by systemd service configurations
	return nil
}

// Remove deletes the firewall rules.
func Remove() error {
	return nil
}

// Check returns true if the firewall rules are configured.
func Check() bool {
	// On Linux, we assume the admin has configured the firewall
	return true
}

// AllowPort adds a firewall rule for a specific port.
func AllowPort(port int, protocol string) error {
	return fmt.Errorf("firewall management on Linux must be done manually via iptables/nftables")
}

// RemovePort removes a port-specific firewall rule.
func RemovePort(port int, protocol string) error {
	return fmt.Errorf("firewall management on Linux must be done manually via iptables/nftables")
}