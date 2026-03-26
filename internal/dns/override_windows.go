//go:build windows

package dns

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// Override sets the system DNS resolver on Windows using netsh.
// It returns a restore function to revert DNS settings.
func Override(listenAddr string) (restoreFunc func() error, err error) {
	// Extract just the IP from listenAddr (strip port).
	ip := listenAddr
	for i := len(listenAddr) - 1; i >= 0; i-- {
		if listenAddr[i] == ':' {
			ip = listenAddr[:i]
			break
		}
	}

	// Get the active interface name
	iface, err := getActiveInterface()
	if err != nil {
		return nil, fmt.Errorf("get active interface: %w", err)
	}

	// Save original DNS settings
	original, err := getDNSServers(iface)
	if err != nil {
		// If we can't get current settings, assume DHCP
		original = "dhcp"
	}

	// Set our DNS resolver as primary
	cmd := exec.Command("netsh", "interface", "ip", "set", "dns",
		fmt.Sprintf("name=%s", iface),
		"source=static",
		ip)
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("set dns: %w: %s", err, out)
	}

	restore := func() error {
		if original == "dhcp" || original == "" {
			// Restore DHCP
			cmd := exec.Command("netsh", "interface", "ip", "set", "dns",
				fmt.Sprintf("name=%s", iface),
				"source=dhcp")
			return cmd.Run()
		}
		// Restore original static DNS
		servers := strings.Fields(original)
		if len(servers) == 0 {
			return nil
		}

		// Set primary DNS
		cmd := exec.Command("netsh", "interface", "ip", "set", "dns",
			fmt.Sprintf("name=%s", iface),
			"source=static",
			servers[0])
		if err := cmd.Run(); err != nil {
			return err
		}

		// Add secondary DNS if present
		for i := 1; i < len(servers); i++ {
			cmd := exec.Command("netsh", "interface", "ip", "add", "dns",
				fmt.Sprintf("name=%s", iface),
				servers[i],
				"index=2")
			cmd.Run() // Ignore errors for secondary
		}
		return nil
	}

	return restore, nil
}

// getActiveInterface returns the name of the active network interface
func getActiveInterface() (string, error) {
	// Get interface with default route (active internet connection)
	cmd := exec.Command("netsh", "interface", "ipv4", "show", "route")
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("netsh show route: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		// Look for 0.0.0.0/0 route which indicates default gateway
		if strings.Contains(line, "0.0.0.0") && strings.Contains(line, "0.0.0.0") {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				// The interface name is typically at the end
				// Extract from the line - format varies by Windows version
				continue
			}
		}
	}

	// Alternative: try to get interface from route table
	cmd = exec.Command("route", "print", "0.0.0.0")
	out, err = cmd.Output()
	if err == nil {
		outLines := strings.Split(string(out), "\n")
		for _, line := range outLines {
			if strings.Contains(line, "0.0.0.0") && !strings.Contains(line, "Network Destination") {
				// Parse route table line
				fields := strings.Fields(line)
				if len(fields) >= 4 {
					// Get interface name from interface index
					idx := fields[3]
					return getInterfaceNameByIndex(idx)
				}
			}
		}
	}

	// Fallback: list interfaces and pick first non-loopback
	cmd = exec.Command("netsh", "interface", "show", "interface")
	out, err = cmd.Output()
	if err != nil {
		return "", fmt.Errorf("list interfaces: %w", err)
	}

	interfaceLines := strings.Split(string(out), "\n")
	for _, line := range interfaceLines {
		fields := strings.Fields(line)
		if len(fields) >= 4 {
			// Format: AdminState State Type Name
			// Look for "Connected" state
			if len(fields) >= 3 && (fields[1] == "Connected" || fields[2] == "Connected") {
				// Interface name is everything from field 3 onwards
				nameStart := strings.Index(line, fields[3])
				if nameStart > 0 {
					name := strings.TrimSpace(line[nameStart:])
					if name != "Loopback Pseudo-Interface 1" && !strings.Contains(name, "Loopback") {
						return name, nil
					}
				}
			}
		}
	}

	return "Ethernet", nil // common fallback
}

// getInterfaceNameByIndex converts interface index to name
func getInterfaceNameByIndex(idx string) (string, error) {
	cmd := exec.Command("netsh", "interface", "show", "interface")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, idx) {
			fields := strings.Fields(line)
			if len(fields) >= 4 {
				nameStart := strings.Index(line, fields[3])
				if nameStart > 0 {
					return strings.TrimSpace(line[nameStart:]), nil
				}
			}
		}
	}
	return "", fmt.Errorf("interface not found")
}

// getDNSServers returns the current DNS servers for an interface
func getDNSServers(iface string) (string, error) {
	cmd := exec.Command("netsh", "interface", "ip", "show", "dns",
		fmt.Sprintf("name=%s", iface))
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	output := string(out)
	// Check if using DHCP
	if strings.Contains(output, "DHCP") || strings.Contains(output, "dhcp") {
		return "dhcp", nil
	}

	// Parse static DNS servers from output
	var servers []string
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "DNS servers") || strings.Contains(line, "DNS Servers") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				server := strings.TrimSpace(parts[1])
				if server != "" && server != "None" {
					servers = append(servers, server)
				}
			}
		}
		// Alternative: look for IP addresses in output
		fields := strings.Fields(line)
		for _, f := range fields {
			if strings.Count(f, ".") == 3 {
				// Might be an IP address
				if net.ParseIP(f) != nil {
					servers = append(servers, f)
				}
			}
		}
	}

	return strings.Join(servers, " "), nil
}
