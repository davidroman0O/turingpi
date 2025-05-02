package operations

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// NetworkOperations provides network operations that can be executed
// either directly on a Linux host or inside a container on non-Linux systems
type NetworkOperations struct {
	executor CommandExecutor
	fs       *FilesystemOperations
}

// NewNetworkOperations creates a new NetworkOperations instance
func NewNetworkOperations(executor CommandExecutor) *NetworkOperations {
	return &NetworkOperations{
		executor: executor,
		fs:       NewFilesystemOperations(executor),
	}
}

// ApplyNetworkConfig applies network configuration to the mounted system
func (n *NetworkOperations) ApplyNetworkConfig(ctx context.Context, mountDir, hostname, ipCIDR, gateway string, dnsServers []string) error {
	// Log what we're applying for debugging
	fmt.Printf("Applying network configuration:\n")
	fmt.Printf("Hostname: %s\n", hostname)
	fmt.Printf("IP CIDR: %s\n", ipCIDR)
	fmt.Printf("Gateway: %s\n", gateway)
	fmt.Printf("DNS Servers (input): %v\n", dnsServers)

	// Sanitize the DNS servers - ensure there are no formatting issues
	var cleanDNSServers []string
	for _, dns := range dnsServers {
		// Clean up any formatting that might have occurred
		dns = strings.TrimSpace(dns)
		dns = strings.Trim(dns, "[]")
		dns = strings.Trim(dns, "\"")

		// Skip empty entries
		if dns == "" {
			continue
		}

		// If a DNS entry contains multiple IPs (like "8.8.8.8, 8.8.4.4"), split them
		if strings.Contains(dns, ",") {
			parts := strings.Split(dns, ",")
			for _, part := range parts {
				cleanPart := strings.TrimSpace(part)
				if cleanPart != "" {
					cleanDNSServers = append(cleanDNSServers, cleanPart)
				}
			}
		} else {
			cleanDNSServers = append(cleanDNSServers, dns)
		}
	}

	// Use the cleaned DNS servers from now on
	if len(cleanDNSServers) > 0 {
		fmt.Printf("Using cleaned DNS servers: %v\n", cleanDNSServers)
		dnsServers = cleanDNSServers
	}

	// Set hostname
	fmt.Printf("Setting hostname to: %s\n", hostname)
	if err := n.fs.WriteFile(mountDir, "etc/hostname", []byte(hostname+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write hostname file: %w", err)
	}

	// Update /etc/hosts file
	hostsContent := fmt.Sprintf("127.0.0.1\tlocalhost\n127.0.1.1\t%s\n\n# The following lines are desirable for IPv6 capable hosts\n::1\tlocalhost ip6-localhost ip6-loopback\nff02::1\tip6-allnodes\nff02::2\tip6-allrouters\n", hostname)
	if err := n.fs.WriteFile(mountDir, "etc/hosts", []byte(hostsContent), 0644); err != nil {
		return fmt.Errorf("failed to update hosts file: %w", err)
	}

	// Check if image uses Netplan (Ubuntu/newer Debian) or traditional interfaces
	usesNetplan := n.fs.FileExists(mountDir, "etc/netplan")
	usesSystemd := n.fs.FileExists(mountDir, "etc/systemd/network")

	fmt.Printf("Detected network configuration: Netplan: %t, SystemdNetworkd: %t\n", usesNetplan, usesSystemd)

	if usesNetplan {
		fmt.Printf("Configuring using Netplan\n")
		return n.configureNetplan(ctx, mountDir, ipCIDR, gateway, dnsServers)
	} else if usesSystemd {
		fmt.Printf("Configuring using systemd-networkd\n")
		return n.configureSystemdNetworkd(ctx, mountDir, ipCIDR, gateway, dnsServers)
	} else {
		fmt.Printf("Configuring using traditional interfaces\n")
		return n.configureInterfaces(mountDir, ipCIDR, gateway, dnsServers)
	}
}

// configureNetplan creates Netplan configuration for Ubuntu/newer Debian
func (n *NetworkOperations) configureNetplan(ctx context.Context, mountDir, ipCIDR, gateway string, dnsServers []string) error {
	// Create Netplan directory if it doesn't exist
	if err := n.fs.MakeDirectory(mountDir, "etc/netplan", 0755); err != nil {
		return fmt.Errorf("failed to create netplan directory: %w", err)
	}

	// Clean and sanitize DNS servers
	var cleanedDNSList []string
	for _, dns := range dnsServers {
		// Clean the DNS server address
		dns = strings.TrimSpace(dns)

		// Remove quotes, brackets and other characters that might be present
		dns = strings.Trim(dns, "[]\"'")

		// Skip empty entries
		if dns == "" {
			continue
		}

		// Handle case where multiple DNS entries are in a single string
		if strings.Contains(dns, ",") {
			parts := strings.Split(dns, ",")
			for _, part := range parts {
				cleanPart := strings.TrimSpace(part)
				if cleanPart != "" {
					cleanedDNSList = append(cleanedDNSList, cleanPart)
				}
			}
		} else {
			cleanedDNSList = append(cleanedDNSList, dns)
		}
	}

	// Format DNS servers for YAML - each as individual quoted item in comma-separated list
	dnsAddrs := ""
	if len(cleanedDNSList) > 0 {
		// Convert the list to a quoted, comma-separated string
		quotedList := make([]string, len(cleanedDNSList))
		for i, dns := range cleanedDNSList {
			quotedList[i] = dns
		}
		dnsAddrs = strings.Join(quotedList, ", ")
		fmt.Printf("DNS Addresses for netplan: [%s]\n", dnsAddrs)
	} else {
		fmt.Printf("Warning: No valid DNS servers provided for network configuration\n")
		// Provide fallback DNS servers (Google DNS)
		dnsAddrs = "8.8.8.8, 8.8.4.4"
		fmt.Printf("Using fallback DNS servers: [%s]\n", dnsAddrs)
	}

	// Check if we should use gateway4 (older) or routes (newer)
	useRoutes := false
	if n.fs.FileExists(mountDir, "etc/netplan") {
		// Check for any existing netplan files to determine format
		// This is a rudimentary check and may need to be enhanced
		files, err := n.fs.ListFilesBasic(ctx, filepath.Join(mountDir, "etc/netplan"))
		if err == nil && len(files) > 0 {
			// Check a sample file for gateway4 vs routes format
			for _, file := range files {
				if strings.HasSuffix(file, ".yaml") {
					content, err := n.fs.ReadFile(mountDir, "etc/netplan/"+filepath.Base(file))
					if err == nil {
						if strings.Contains(string(content), "routes:") && !strings.Contains(string(content), "gateway4:") {
							useRoutes = true
							break
						}
					}
				}
			}
		}
	}

	var netplanYaml string
	if useRoutes {
		// Newer netplan format with routes
		netplanYaml = fmt.Sprintf(`# Generated by Turing Pi Tools
network:
  version: 2
  ethernets:
    eth0:
      dhcp4: no
      addresses: [%s]
      routes:
        - to: default
          via: %s
      nameservers:
        addresses: [%s]
`, ipCIDR, gateway, dnsAddrs)
	} else {
		// Older netplan format with gateway4
		netplanYaml = fmt.Sprintf(`# Generated by Turing Pi Tools
network:
  version: 2
  ethernets:
    eth0:
      dhcp4: no
      addresses: [%s]
      gateway4: %s
      nameservers:
        addresses: [%s]
`, ipCIDR, gateway, dnsAddrs)
	}

	// Write netplan config
	if err := n.fs.WriteFile(mountDir, "etc/netplan/01-netcfg.yaml", []byte(netplanYaml), 0644); err != nil {
		return fmt.Errorf("failed to write netplan config: %w", err)
	}

	fmt.Printf("Successfully wrote netplan configuration\n")
	fmt.Printf("Netplan content:\n%s\n", netplanYaml)
	return nil
}

// configureSystemdNetworkd creates systemd-networkd configuration
func (n *NetworkOperations) configureSystemdNetworkd(ctx context.Context, mountDir, ipCIDR, gateway string, dnsServers []string) error {
	// Create necessary directory
	if err := n.fs.MakeDirectory(mountDir, "etc/systemd/network", 0755); err != nil {
		return fmt.Errorf("failed to create systemd network directory: %w", err)
	}

	// Build DNS configuration
	dnsConfig := ""
	for _, dns := range dnsServers {
		dns = strings.TrimSpace(dns)
		if dns != "" {
			dnsConfig += fmt.Sprintf("DNS=%s\n", dns)
		}
	}
	fmt.Printf("Systemd-networkd DNS config:\n%s\n", dnsConfig)

	// Create the network configuration
	networkConfig := fmt.Sprintf(`[Match]
Name=eth0

[Network]
Address=%s
Gateway=%s
%s
`, ipCIDR, gateway, dnsConfig)

	// Write systemd-networkd config
	if err := n.fs.WriteFile(mountDir, "etc/systemd/network/20-wired.network", []byte(networkConfig), 0644); err != nil {
		return fmt.Errorf("failed to write systemd network config: %w", err)
	}

	// Enable the systemd-networkd service
	if err := n.fs.MakeDirectory(mountDir, "etc/systemd/system/multi-user.target.wants", 0755); err != nil {
		return fmt.Errorf("failed to create systemd wants directory: %w", err)
	}

	// Create symlinks to enable services if they don't already exist
	// This mirrors what systemctl enable would do
	wantsDir := filepath.Join(mountDir, "etc/systemd/system/multi-user.target.wants")

	// Check if files exist and create symlinks if needed
	if n.fs.FileExists(mountDir, "lib/systemd/system/systemd-networkd.service") {
		// Create symlink from service to wants directory
		linkCmd := fmt.Sprintf("ln -sf /lib/systemd/system/systemd-networkd.service %s/systemd-networkd.service", wantsDir)
		_, err := n.executor.Execute(ctx, "sh", "-c", linkCmd)
		if err != nil {
			fmt.Printf("Warning: Failed to enable systemd-networkd: %v\n", err)
		}
	}

	if n.fs.FileExists(mountDir, "lib/systemd/system/systemd-resolved.service") {
		// Create symlink for resolved service
		linkCmd := fmt.Sprintf("ln -sf /lib/systemd/system/systemd-resolved.service %s/systemd-resolved.service", wantsDir)
		_, err := n.executor.Execute(ctx, "sh", "-c", linkCmd)
		if err != nil {
			fmt.Printf("Warning: Failed to enable systemd-resolved: %v\n", err)
		}
	}

	fmt.Printf("Successfully configured systemd-networkd\n")
	return nil
}

// configureInterfaces creates traditional network interfaces configuration for Debian
func (n *NetworkOperations) configureInterfaces(mountDir, ipCIDR, gateway string, dnsServers []string) error {
	// Extract IP and network bits
	parts := strings.Split(ipCIDR, "/")
	if len(parts) != 2 {
		return fmt.Errorf("invalid IP CIDR format: %s", ipCIDR)
	}
	ipAddr := parts[0]
	networkBits := parts[1]

	// Calculate netmask (simplistic for common masks)
	var netmask string
	switch networkBits {
	case "8":
		netmask = "255.0.0.0"
	case "16":
		netmask = "255.255.0.0"
	case "24":
		netmask = "255.255.255.0"
	case "32":
		netmask = "255.255.255.255"
	default:
		// For other CIDR notations, this is a simplification
		// In production, a proper CIDR to netmask conversion should be used
		netmask = "255.255.255.0"
		fmt.Printf("Warning: Using default netmask 255.255.255.0 for CIDR %s\n", networkBits)
	}

	// Build interfaces config with properly formatted DNS servers
	var cleanedDNS []string
	for _, dns := range dnsServers {
		dns = strings.TrimSpace(dns)
		if dns != "" {
			cleanedDNS = append(cleanedDNS, dns)
		}
	}
	dnsLine := "dns-nameservers " + strings.Join(cleanedDNS, " ")
	fmt.Printf("DNS Line for interfaces file: %s\n", dnsLine)

	interfacesContent := fmt.Sprintf(`# Generated by Turing Pi Tools
# The loopback network interface
auto lo
iface lo inet loopback

# The primary network interface
auto eth0
iface eth0 inet static
    address %s
    netmask %s
    gateway %s
    %s
`, ipAddr, netmask, gateway, dnsLine)

	// Ensure network directory exists
	if err := n.fs.MakeDirectory(mountDir, "etc/network", 0755); err != nil {
		return fmt.Errorf("failed to create network directory: %w", err)
	}

	// Write interfaces config
	if err := n.fs.WriteFile(mountDir, "etc/network/interfaces", []byte(interfacesContent), 0644); err != nil {
		return fmt.Errorf("failed to write interfaces file: %w", err)
	}

	// Write resolv.conf file with DNS configuration
	resolvContent := "# Generated by Turing Pi Tools\n"
	for _, dns := range cleanedDNS {
		if dns != "" {
			resolvContent += fmt.Sprintf("nameserver %s\n", dns)
		}
	}
	fmt.Printf("Writing resolv.conf with content:\n%s\n", resolvContent)

	if err := n.fs.WriteFile(mountDir, "etc/resolv.conf", []byte(resolvContent), 0644); err != nil {
		fmt.Printf("Warning: Failed to write resolv.conf: %v\n", err)
	}

	fmt.Printf("Successfully configured traditional interfaces\n")
	return nil
}
