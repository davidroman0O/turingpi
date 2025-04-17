package imageops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ApplyNetworkConfig applies network configuration to a mounted filesystem
func ApplyNetworkConfig(mountDir string, hostname string, ipCIDR string, gateway string, dnsServers []string) error {
	// Set hostname
	hostnameFile := filepath.Join(mountDir, "etc/hostname")
	if err := writeToFileAsRoot(hostnameFile, []byte(hostname), 0644); err != nil {
		return fmt.Errorf("failed to write hostname: %w", err)
	}

	// Update hosts file
	hostsContent := fmt.Sprintf("127.0.0.1 localhost\n127.0.1.1 %s\n", hostname)
	hostsFile := filepath.Join(mountDir, "etc/hosts")
	if err := writeToFileAsRoot(hostsFile, []byte(hostsContent), 0644); err != nil {
		return fmt.Errorf("failed to write hosts file: %w", err)
	}

	// Check for netplan vs interfaces
	netplanDir := filepath.Join(mountDir, "etc/netplan")
	if _, err := os.Stat(netplanDir); err == nil {
		// Ubuntu-style netplan configuration
		if err := applyNetplanConfig(mountDir, ipCIDR, gateway, dnsServers); err != nil {
			return fmt.Errorf("failed to apply netplan config: %w", err)
		}
	} else {
		// Debian-style interfaces configuration
		if err := applyInterfacesConfig(mountDir, ipCIDR, gateway, dnsServers); err != nil {
			return fmt.Errorf("failed to apply interfaces config: %w", err)
		}
	}

	return nil
}

// applyNetplanConfig applies Ubuntu-style netplan configuration
func applyNetplanConfig(mountDir string, ipCIDR string, gateway string, dnsServers []string) error {
	netplanConfig := fmt.Sprintf(`network:
  version: 2
  ethernets:
    eth0:
      addresses:
        - %s
      gateway4: %s
      nameservers:
        addresses: [%s]
`, ipCIDR, gateway, strings.Join(dnsServers, ", "))

	netplanFile := filepath.Join(mountDir, "etc/netplan/01-netcfg.yaml")
	if err := writeToFileAsRoot(netplanFile, []byte(netplanConfig), 0644); err != nil {
		return fmt.Errorf("failed to write netplan config: %w", err)
	}

	return nil
}

// applyInterfacesConfig applies Debian-style interfaces configuration
func applyInterfacesConfig(mountDir string, ipCIDR string, gateway string, dnsServers []string) error {
	interfacesConfig := fmt.Sprintf(`# This file describes the network interfaces available on your system
# and how to activate them. For more information, see interfaces(5).

source /etc/network/interfaces.d/*

# The loopback network interface
auto lo
iface lo inet loopback

# The primary network interface
auto eth0
iface eth0 inet static
    address %s
    gateway %s
    dns-nameservers %s
`, ipCIDR, gateway, strings.Join(dnsServers, " "))

	interfacesFile := filepath.Join(mountDir, "etc/network/interfaces")
	if err := writeToFileAsRoot(interfacesFile, []byte(interfacesConfig), 0644); err != nil {
		return fmt.Errorf("failed to write interfaces config: %w", err)
	}

	return nil
}
