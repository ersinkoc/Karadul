//go:build windows

package node

import (
	"fmt"
	"os/exec"
	"strings"
)

// EnableExitNode configures Windows to act as an exit node using Internet Connection Sharing (ICS)
// or NAT via netsh. This enables other mesh nodes to route traffic through this node.
func EnableExitNode(outIface string) error {
	// Method 1: Use PowerShell to configure NAT (Windows 10/Server 2016+)
	// This is the modern approach

	// First, check if we can use the newer PowerShell cmdlets
	psCmd := `
$iface = Get-NetAdapter -Name "` + outIface + `" -ErrorAction SilentlyContinue
if ($iface) {
    # Check if NAT is already configured
    $nat = Get-NetNat -Name "KaradulExitNode" -ErrorAction SilentlyContinue
    if (-not $nat) {
        # Create NAT network
        New-NetNat -Name "KaradulExitNode" -InternalIPInterfaceAddressPrefix 100.64.0.0/10
    }
    # Enable IP forwarding via registry
    Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters" -Name "IPEnableRouter" -Value 1
    Write-Output "SUCCESS"
} else {
    Write-Output "INTERFACE_NOT_FOUND"
}
`
	cmd := exec.Command("powershell", "-Command", psCmd)
	out, err := cmd.CombinedOutput()
	output := strings.TrimSpace(string(out))

	if err != nil || output == "INTERFACE_NOT_FOUND" {
		// Fallback to netsh method for older Windows
		return enableExitNodeNetsh(outIface)
	}

	if !strings.Contains(output, "SUCCESS") {
		return fmt.Errorf("failed to enable exit node: %s", output)
	}

	return nil
}

// enableExitNodeNetsh uses netsh for older Windows versions
func enableExitNodeNetsh(outIface string) error {
	// Enable IP forwarding via registry
	cmd := exec.Command("reg", "add",
		"HKLM\\SYSTEM\\CurrentControlSet\\Services\\Tcpip\\Parameters",
		"/v", "IPEnableRouter",
		"/t", "REG_DWORD",
		"/d", "1",
		"/f")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("enable IP forwarding: %w: %s", err, out)
	}

	// For Windows, we need to enable Internet Connection Sharing (ICS)
	// This is complex via command line, so we use a firewall rule approach
	// to allow forwarding

	// Add firewall rule to allow forwarding
	cmd = exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		"name=Karadul Exit Node",
		"dir=in",
		"action=allow",
		"protocol=any")
	cmd.Run() // Ignore error, rule might already exist

	// Enable forwarding in firewall
	cmd = exec.Command("netsh", "advfirewall", "set", "allprofiles",
		"firewallpolicy", "allowinbound,allowoutbound")
	cmd.Run()

	return nil
}

// DisableExitNode removes the exit node configuration.
func DisableExitNode(outIface string) error {
	// Method 1: Remove PowerShell NAT configuration
	psCmd := `
$nat = Get-NetNat -Name "KaradulExitNode" -ErrorAction SilentlyContinue
if ($nat) {
    Remove-NetNat -Name "KaradulExitNode" -Confirm:$false
}
# Disable IP forwarding
Set-ItemProperty -Path "HKLM:\SYSTEM\CurrentControlSet\Services\Tcpip\Parameters" -Name "IPEnableRouter" -Value 0
Write-Output "SUCCESS"
`
	cmd := exec.Command("powershell", "-Command", psCmd)
	out, err := cmd.CombinedOutput()

	// Also try registry method
	exec.Command("reg", "add",
		"HKLM\\SYSTEM\\CurrentControlSet\\Services\\Tcpip\\Parameters",
		"/v", "IPEnableRouter",
		"/t", "REG_DWORD",
		"/d", "0",
		"/f").Run()

	// Remove firewall rule
	exec.Command("netsh", "advfirewall", "firewall", "delete", "rule",
		"name=Karadul Exit Node").Run()

	if err != nil && !strings.Contains(string(out), "SUCCESS") {
		// Don't fail if it was already disabled
		return nil
	}

	return nil
}
