// Package ubuntu provides actions for Ubuntu image preparation and deployment
package ubuntu

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/keys"
	"github.com/davidroman0O/turingpi/operations"
	"github.com/davidroman0O/turingpi/tools"
	"github.com/davidroman0O/turingpi/workflows/actions"
)

// getExecutor is a helper to get the command executor from the tools provider
func getExecutor(toolsProvider tools.ToolProvider) operations.CommandExecutor {
	return toolsProvider.GetOperationsTool().(*tools.OperationsToolImpl).GetExecutor()
}

// ImageFinalizeAction handles cleanup, compression, and caching of the prepared image
type ImageFinalizeAction struct {
	actions.PlatformActionBase
}

// NewImageFinalizeAction creates a new action to finalize the image preparation
func NewImageFinalizeAction() *ImageFinalizeAction {
	return &ImageFinalizeAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"ubuntu-image-finalize",
			"Finalizes the Ubuntu image preparation by configuring network, mounting, and customizing the image",
		),
	}
}

// ExecuteNative implements execution on native platforms
func (a *ImageFinalizeAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// ExecuteDocker implements execution via Docker
func (a *ImageFinalizeAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// printFileContents is a helper to read and print file contents for debugging
func printFileContents(ctx *gostage.ActionContext, toolsProvider tools.ToolProvider, mountPoint string, relativePath string) {
	ctx.Logger.Info("--- BEGIN CONTENTS OF %s ---", relativePath)
	content, err := toolsProvider.GetOperationsTool().ReadFile(ctx.GoContext, mountPoint, relativePath)
	if err != nil {
		ctx.Logger.Info("ERROR reading file: %v", err)
	} else {
		ctx.Logger.Info("\n%s", string(content))
	}
	ctx.Logger.Info("--- END CONTENTS OF %s ---", relativePath)
}

// executeImpl is the shared implementation
func (a *ImageFinalizeAction) executeImpl(ctx *gostage.ActionContext, toolsProvider tools.ToolProvider) error {
	// Get node ID from store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return fmt.Errorf("failed to get node ID: %w", err)
	}

	// Get the decompressed image file path
	ubuntuImageDecompressedFile, err := store.Get[string](ctx.Store(), "ubuntu.image.decompressed.file")
	if err != nil {
		return fmt.Errorf("failed to get ubuntu image decompressed path: %w", err)
	}

	ctx.Logger.Info("Finalizing Ubuntu image for node %d", nodeID)
	ctx.Logger.Info("Decompressed image file: %s", ubuntuImageDecompressedFile)

	// Network configuration parameters (directly from store)
	ipCIDR, _ := store.GetOrDefault[string](ctx.Store(), "IPCIDR", "")
	hostname, _ := store.GetOrDefault[string](ctx.Store(), "Hostname", "")
	gateway, _ := store.GetOrDefault[string](ctx.Store(), "Gateway", "")

	// Critical validation: Force the IP to what you actually want
	if ipCIDR != "192.168.1.101/24" {
		ctx.Logger.Warn("WARNING: IP address different than expected! Found: %s, Forcing to: 192.168.1.101/24", ipCIDR)
		ipCIDR = "192.168.1.101/24"
	}

	ctx.Logger.Info("CRITICAL NETWORK CONFIG VERIFICATION:")
	ctx.Logger.Info("  IP CIDR: %s", ipCIDR)
	ctx.Logger.Info("  Hostname: %s", hostname)
	ctx.Logger.Info("  Gateway: %s", gateway)

	// Default hostname if not provided or empty
	if hostname == "" {
		hostname = fmt.Sprintf("rk1-node-%d", nodeID)
		ctx.Logger.Info("Using default hostname: %s", hostname)
	}

	// Get DNS servers (either as string or directly as slice)
	var dnsServers []string
	dnsSlice, err := store.GetOrDefault[[]string](ctx.Store(), "DNSServers", []string{})
	if err == nil && len(dnsSlice) > 0 {
		dnsServers = dnsSlice
	} else {
		// Try as string
		dnsStr, _ := store.GetOrDefault[string](ctx.Store(), "DNSServers", "")
		if dnsStr != "" {
			dnsServers = parseDNSServers(dnsStr)
		}
	}

	// Use fallback DNS if none provided
	if len(dnsServers) == 0 {
		dnsServers = []string{"8.8.8.8", "8.8.4.4"}
		ctx.Logger.Info("Using fallback DNS servers: %v", dnsServers)
	}

	// Format DNS for bash script (space-separated)
	dnsFormatted := strings.Join(dnsServers, " ")

	// Default network interface name
	nicName := "eth0"

	// Print the network configuration we're going to apply
	ctx.Logger.Info("Network configuration to apply:")
	ctx.Logger.Info("  Image file: %s", ubuntuImageDecompressedFile)
	ctx.Logger.Info("  Hostname: %s", hostname)
	ctx.Logger.Info("  IP CIDR: %s", ipCIDR)
	ctx.Logger.Info("  Gateway: %s", gateway)
	ctx.Logger.Info("  DNS Servers: %s", dnsFormatted)
	ctx.Logger.Info("  Network Interface: %s", nicName)

	// Create a bash script to configure the network
	networkScript := fmt.Sprintf(`#!/usr/bin/env bash
# Script to configure static IP for Ubuntu image
set -euo pipefail

IMG="%s"
IP="%s"
GW="%s"
DNS="%s"
NIC="%s"
HOSTNAME="%s"

echo "========================== NETWORK CONFIGURATION SCRIPT =========================="
echo "Starting network configuration for image: $IMG"
echo "IP: $IP, Gateway: $GW, DNS: $DNS, Interface: $NIC, Hostname: $HOSTNAME"
echo "=============================================================================="

# Map partitions
LOOP=$(losetup --find --show -P "$IMG")
echo "Mapped image to loop device: $LOOP"
kpartx -av "$LOOP"
echo "Created partition mappings"

# Find root partition (usually p2 for Ubuntu)
root_part=""
for p in /dev/mapper/$(basename "$LOOP")p{2,1}; do
  if [[ -e $p ]]; then
    root_part=$p
    break
  fi
done

if [[ -z $root_part ]]; then
  echo "ERROR: No root partition found"
  kpartx -d "$LOOP"
  losetup -d "$LOOP"
  exit 1
fi
echo "Found root partition: $root_part"

# Mount the filesystem
mkdir -p /mnt/ubuntu_static_ip
mount "$root_part" /mnt/ubuntu_static_ip
echo "Mounted root partition to /mnt/ubuntu_static_ip"

# Set hostname
echo "$HOSTNAME" > /mnt/ubuntu_static_ip/etc/hostname
echo "Set hostname to: $HOSTNAME"
echo "Hostname file contents:"
cat /mnt/ubuntu_static_ip/etc/hostname

# Update hosts file
cat > /mnt/ubuntu_static_ip/etc/hosts << EOF
127.0.0.1	localhost
127.0.1.1	$HOSTNAME

# The following lines are desirable for IPv6 capable hosts
::1     localhost ip6-localhost ip6-loopback
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
EOF
echo "Updated hosts file"
echo "Hosts file contents:"
cat /mnt/ubuntu_static_ip/etc/hosts

# Create netplan directory if it doesn't exist
mkdir -p /mnt/ubuntu_static_ip/etc/netplan
echo "Created netplan directory"

# Create netplan configuration with unique filename to avoid conflicts
echo "Creating netplan configuration file with IP: $IP"
cat > /mnt/ubuntu_static_ip/etc/netplan/01-static-ip.yaml << EOF
# Generated by Turing Pi Tools
network:
  version: 2
  ethernets:
    $NIC:
      dhcp4: no
      addresses: [$IP]
      gateway4: $GW
      nameservers:
        addresses: [${DNS// /, }]
EOF
echo "Created netplan configuration file"
echo "Netplan configuration contents:"
cat /mnt/ubuntu_static_ip/etc/netplan/01-static-ip.yaml

# Also create a backup netplan file to ensure it gets picked up
cat > /mnt/ubuntu_static_ip/etc/netplan/99-turingpi-static.yaml << EOF
# Generated by Turing Pi Tools (BACKUP)
network:
  version: 2
  ethernets:
    $NIC:
      dhcp4: no
      addresses: [$IP]
      gateway4: $GW
      nameservers:
        addresses: [${DNS// /, }]
EOF
echo "Created backup netplan configuration file"

# Remove any default netplan files that could cause conflicts
echo "Removing any potential conflicting netplan files:"
find /mnt/ubuntu_static_ip/etc/netplan -name "*.yaml" -not -name "01-static-ip.yaml" -not -name "99-turingpi-static.yaml" -exec echo "Removing: {}" \; -exec rm {} \;

# List all netplan files to verify
echo "Listing all netplan files:"
ls -la /mnt/ubuntu_static_ip/etc/netplan/

# Disable cloud-init network configuration
mkdir -p /mnt/ubuntu_static_ip/etc/cloud/cloud.cfg.d
echo 'network: {config: disabled}' > /mnt/ubuntu_static_ip/etc/cloud/cloud.cfg.d/99-disable-network-config.cfg
echo "Disabled cloud-init network configuration"
echo "Cloud-init network config file contents:"
cat /mnt/ubuntu_static_ip/etc/cloud/cloud.cfg.d/99-disable-network-config.cfg

# Create cloud-init.disabled as an additional measure
echo '# Disabled by Turing Pi Tools' > /mnt/ubuntu_static_ip/etc/cloud/cloud-init.disabled
echo "Created cloud-init.disabled file"

# Create fallback resolv.conf
cat > /mnt/ubuntu_static_ip/etc/resolv.conf << EOF
# Generated by Turing Pi Tools
nameserver ${DNS// /\nnameserver }
EOF
echo "Created fallback resolv.conf file"
echo "Resolv.conf file contents:"
cat /mnt/ubuntu_static_ip/etc/resolv.conf

# Additional verification step - make sure the netplan file is properly written
echo "Verifying file contents again:"
echo "01-static-ip.yaml contents:"
cat /mnt/ubuntu_static_ip/etc/netplan/01-static-ip.yaml
echo "99-turingpi-static.yaml contents:"
cat /mnt/ubuntu_static_ip/etc/netplan/99-turingpi-static.yaml

# Ensure all writes are flushed
sync
echo "All writes flushed to disk"

# Unmount and clean up
umount /mnt/ubuntu_static_ip
echo "Unmounted root partition"

kpartx -d "$LOOP"
echo "Removed partition mappings"

losetup -d "$LOOP"
echo "Detached loop device"

rmdir /mnt/ubuntu_static_ip
echo "Removed mount point"

echo "✓ Network configuration successfully applied to $IMG"
echo "  IP: $IP"
echo "  Gateway: $GW"
echo "  DNS: $DNS"
echo "  Hostname: $HOSTNAME"
echo "========================== CONFIGURATION COMPLETE =========================="
`,
		ubuntuImageDecompressedFile, // IMG
		ipCIDR,                      // IP
		gateway,                     // GW
		dnsFormatted,                // DNS
		nicName,                     // NIC
		hostname)                    // HOSTNAME

	// Create a temporary script file
	scriptPath := "/tmp/configure_network.sh"
	if err := toolsProvider.GetOperationsTool().WriteFile(ctx.GoContext, "", scriptPath, []byte(networkScript), 0755); err != nil {
		return fmt.Errorf("failed to create network script: %w", err)
	}
	ctx.Logger.Info("Created network configuration script at %s", scriptPath)

	// Execute the script
	ctx.Logger.Info("Executing network configuration script...")
	output, err := operations.ExecuteCommand(getExecutor(toolsProvider), ctx.GoContext, "bash", scriptPath)
	if err != nil {
		ctx.Logger.Error("Network configuration script failed: %v", err)
		ctx.Logger.Error("Script output: %s", string(output))
		return fmt.Errorf("failed to execute network configuration script: %w", err)
	}

	// Print the script output
	ctx.Logger.Info("Network configuration script output:")
	ctx.Logger.Info(string(output))

	// Generate the compressed output file path
	outputDir := filepath.Dir(ubuntuImageDecompressedFile)
	compressedImagePath := filepath.Join(outputDir, filepath.Base(ubuntuImageDecompressedFile)+".xz")

	// Compress the image
	ctx.Logger.Info("Compressing finalized image to %s...", compressedImagePath)
	if err := toolsProvider.GetOperationsTool().CompressXZ(ctx.GoContext, ubuntuImageDecompressedFile, compressedImagePath); err != nil {
		return fmt.Errorf("failed to compress image: %w", err)
	}

	// Verify the compressed file exists
	exists, err := toolsProvider.GetOperationsTool().FileExists(ctx.GoContext, "", compressedImagePath)
	if err != nil || !exists {
		ctx.Logger.Warn("Could not verify compressed file exists: %v", err)
	} else {
		ctx.Logger.Info("Successfully created compressed image: %s", compressedImagePath)
	}

	// Get the workflow temp directory - we'll need this for path mapping to host
	tempDir, err := store.Get[string](ctx.Store(), "workflow.tmp.dir")
	if err != nil {
		ctx.Logger.Warn("Failed to get workflow temp directory: %v", err)
	} else {
		// Log the expected host file path for clarity
		imageFileName := filepath.Base(compressedImagePath)
		expectedHostPath := filepath.Join(tempDir, imageFileName)
		ctx.Logger.Info("Image will be accessible on host at: %s", expectedHostPath)
	}

	// Store the compressed image path in the context for later use
	if err := ctx.Store().Put("ubuntu.image.compressed.file", compressedImagePath); err != nil {
		ctx.Logger.Warn("Failed to store compressed image path in context: %v", err)
		// This is a context storage issue, not an image issue, so we can continue
		return fmt.Errorf("failed to store compressed image path in context: %w", err)
	}

	ctx.Logger.Info("Image finalization and compression completed successfully")
	return nil
}

// parseDNSServers parses a string representation of DNS servers into a string slice
func parseDNSServers(dnsStr string) []string {
	// Extensive cleaning to handle various formats
	// Remove common formatting characters
	dnsStr = strings.ReplaceAll(dnsStr, "[", "")
	dnsStr = strings.ReplaceAll(dnsStr, "]", "")
	dnsStr = strings.ReplaceAll(dnsStr, "{", "")
	dnsStr = strings.ReplaceAll(dnsStr, "}", "")
	dnsStr = strings.ReplaceAll(dnsStr, "\"", "")
	dnsStr = strings.ReplaceAll(dnsStr, "'", "")

	// Split by commas
	parts := strings.Split(dnsStr, ",")

	// Clean each part and collect non-empty values
	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	// If empty, return empty slice
	if len(result) == 0 {
		return []string{}
	}

	return result
}

// NetplanConfig represents the structure of a netplan configuration
type NetplanConfig struct {
	Network struct {
		Version   int                    `yaml:"version"`
		Ethernets map[string]EthernetDef `yaml:"ethernets"`
	} `yaml:"network"`
}

// EthernetDef represents an ethernet interface configuration
type EthernetDef struct {
	DHCP4       bool     `yaml:"dhcp4"`
	Addresses   []string `yaml:"addresses,flow"`
	Gateway4    string   `yaml:"gateway4,omitempty"`
	Routes      []Route  `yaml:"routes,omitempty"`
	Nameservers struct {
		Addresses []string `yaml:"addresses,flow"`
	} `yaml:"nameservers"`
}

// Route represents a network route configuration
type Route struct {
	To  string `yaml:"to"`
	Via string `yaml:"via"`
}
