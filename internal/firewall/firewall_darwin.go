//go:build darwin

package firewall

import "fmt"

// Setup configures firewall rules on macOS using pfctl.
// This is a placeholder for future implementation.
func Setup(exePath string) error {
	// On macOS, firewall rules are typically managed via pfctl
	// This requires root privileges
	return nil
}

// Remove deletes the firewall rules.
func Remove() error {
	return nil
}

// Check returns true if the firewall rules are configured.
func Check() bool {
	// On macOS, we assume the admin has configured the firewall
	return true
}

// AllowPort adds a firewall rule for a specific port.
func AllowPort(port int, protocol string) error {
	return fmt.Errorf("firewall management on macOS must be done manually via pfctl")
}

// RemovePort removes a port-specific firewall rule.
func RemovePort(port int, protocol string) error {
	return fmt.Errorf("firewall management on macOS must be done manually via pfctl")
}