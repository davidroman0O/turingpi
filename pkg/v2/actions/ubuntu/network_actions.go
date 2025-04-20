// Package ubuntu provides Ubuntu-specific actions for TuringPi
package ubuntu

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/davidroman0O/gostate"
	"github.com/davidroman0O/gostate/store"
	"github.com/davidroman0O/turingpi/pkg/v2/actions"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
)

// NetworkConfig holds the network configuration for a node
type NetworkConfig struct {
	Hostname   string   // Hostname for the node
	IPCIDR     string   // IP address with CIDR suffix
	Gateway    string   // Gateway IP address
	DNSServers []string // List of DNS server IP addresses
}

// ConfigureNetworkAction modifies the Ubuntu OS image to set network configuration
type ConfigureNetworkAction struct {
	actions.PlatformActionBase
	nodeID int
}

// NewConfigureNetworkAction creates a new ConfigureNetworkAction
func NewConfigureNetworkAction(nodeID int) *ConfigureNetworkAction {
	return &ConfigureNetworkAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"ConfigureNetwork",
			fmt.Sprintf("Configure network settings for node %d", nodeID),
		),
		nodeID: nodeID,
	}
}

// ExecuteNative implements the native Linux execution path
func (a *ConfigureNetworkAction) ExecuteNative(ctx *gostate.ActionContext, toolProvider tools.ToolProvider) error {
	// Get required tools
	imageTool := toolProvider.GetImageTool()
	if imageTool == nil {
		return fmt.Errorf("image tool is required but not available")
	}

	fsTool := toolProvider.GetFSTool()
	if fsTool == nil {
		return fmt.Errorf("filesystem tool is required but not available")
	}

	// Get the decompressed image path from the context
	decompressedImagePath, err := store.Get[string](ctx.Store, "decompressedImagePath")
	if err != nil {
		return fmt.Errorf("decompressed image path not found in context: %w", err)
	}

	// Get network configuration
	networkConfig, err := getNetworkConfig(ctx, a.nodeID)
	if err != nil {
		return fmt.Errorf("failed to get network configuration: %w", err)
	}

	// Create a temporary mount point
	mountDir, err := store.GetOrDefault(ctx.Store, "mountPoint", filepath.Join(os.TempDir(), "turingpi-mnt"))
	if err != nil {
		return fmt.Errorf("failed to get mount point: %w", err)
	}

	// Ensure mount directory exists
	if err := os.MkdirAll(mountDir, 0755); err != nil {
		return fmt.Errorf("failed to create mount directory: %w", err)
	}

	ctx.Logger.Info("Configuring network for node %d: %s (%s)", a.nodeID, networkConfig.Hostname, networkConfig.IPCIDR)

	// Map partitions
	devicePath, err := imageTool.MapPartitions(context.Background(), decompressedImagePath)
	if err != nil {
		return fmt.Errorf("failed to map partitions: %w", err)
	}

	// Ensure partitions are unmapped when done
	defer func() {
		ctx.Logger.Info("Unmapping partitions")
		_ = imageTool.UnmapPartitions(context.Background(), decompressedImagePath)
	}()

	// Mount the filesystem
	if err := imageTool.MountFilesystem(context.Background(), devicePath, mountDir); err != nil {
		return fmt.Errorf("failed to mount filesystem: %w", err)
	}

	// Ensure filesystem is unmounted when done
	defer func() {
		ctx.Logger.Info("Unmounting filesystem")
		_ = imageTool.UnmountFilesystem(context.Background(), mountDir)
	}()

	// Apply network configuration
	if err := applyNetworkConfig(ctx, imageTool, mountDir, networkConfig); err != nil {
		return fmt.Errorf("failed to apply network configuration: %w", err)
	}

	ctx.Logger.Info("Network configuration completed successfully")
	return nil
}

// ExecuteDocker implements the Docker-based execution path for non-Linux platforms
func (a *ConfigureNetworkAction) ExecuteDocker(ctx *gostate.ActionContext, toolProvider tools.ToolProvider) error {
	// Get required tools
	containerTool := toolProvider.GetContainerTool()
	if containerTool == nil {
		return fmt.Errorf("container tool is required but not available")
	}

	// Get the decompressed image path from the context
	decompressedImagePath, err := store.Get[string](ctx.Store, "decompressedImagePath")
	if err != nil {
		return fmt.Errorf("decompressed image path not found in context: %w", err)
	}

	// Get network configuration
	networkConfig, err := getNetworkConfig(ctx, a.nodeID)
	if err != nil {
		return fmt.Errorf("failed to get network configuration: %w", err)
	}

	// Get temp directory for mounting
	tempDir, err := store.GetOrDefault(ctx.Store, "tempDir", os.TempDir())
	if err != nil {
		return fmt.Errorf("failed to get temp directory: %w", err)
	}

	// Create mount point path for logging
	mountPoint := filepath.Join(tempDir, "mnt")

	ctx.Logger.Info("Configuring network in Docker container for node %d: %s (%s) using mount point %s", a.nodeID, networkConfig.Hostname, networkConfig.IPCIDR, mountPoint)

	// Create a container for the network configuration
	config := createImageConfigContainer(decompressedImagePath, tempDir)
	container, err := containerTool.CreateContainer(context.Background(), config)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Ensure cleanup
	defer func() {
		ctx.Logger.Info("Cleaning up container")
		_ = containerTool.RemoveContainer(context.Background(), container.ID())
	}()

	// Map partitions in container
	containerImgPath := filepath.Join("/images", filepath.Base(decompressedImagePath))
	mapCmd := []string{"kpartx", "-av", containerImgPath}
	output, err := containerTool.RunCommand(context.Background(), container.ID(), mapCmd)
	if err != nil {
		return fmt.Errorf("failed to map partitions in container: %w", err)
	}

	// Parse the output to find the device path
	devicePath, err := parseKpartxOutput(output)
	if err != nil {
		return fmt.Errorf("failed to parse kpartx output: %w", err)
	}

	// Ensure partitions are unmapped when done
	defer func() {
		ctx.Logger.Info("Unmapping partitions in container")
		unmapCmd := []string{"kpartx", "-d", containerImgPath}
		_, _ = containerTool.RunCommand(context.Background(), container.ID(), unmapCmd)
	}()

	// Create mount directory in container
	mkdirCmd := []string{"mkdir", "-p", "/mnt"}
	_, err = containerTool.RunCommand(context.Background(), container.ID(), mkdirCmd)
	if err != nil {
		return fmt.Errorf("failed to create mount directory in container: %w", err)
	}

	// Mount the filesystem in container
	mountCmd := []string{"mount", devicePath, "/mnt"}
	_, err = containerTool.RunCommand(context.Background(), container.ID(), mountCmd)
	if err != nil {
		return fmt.Errorf("failed to mount filesystem in container: %w", err)
	}

	// Ensure filesystem is unmounted when done
	defer func() {
		ctx.Logger.Info("Unmounting filesystem in container")
		unmountCmd := []string{"umount", "/mnt"}
		_, _ = containerTool.RunCommand(context.Background(), container.ID(), unmountCmd)
	}()

	// Apply network configuration in container
	if err := applyNetworkConfigInContainer(ctx, containerTool, container.ID(), networkConfig); err != nil {
		return fmt.Errorf("failed to apply network configuration in container: %w", err)
	}

	ctx.Logger.Info("Network configuration completed successfully in container")
	return nil
}

// applyNetworkConfig applies network configuration to a mounted filesystem
func applyNetworkConfig(ctx *gostate.ActionContext, imageTool tools.ImageTool, mountDir string, config NetworkConfig) error {
	// Set hostname
	if err := imageTool.WriteFile(context.Background(), mountDir, "etc/hostname", []byte(config.Hostname), 0644); err != nil {
		return fmt.Errorf("failed to set hostname: %w", err)
	}

	// Update hosts file
	hostsContent := fmt.Sprintf("127.0.0.1\tlocalhost\n127.0.1.1\t%s\n\n", config.Hostname)
	if err := imageTool.WriteFile(context.Background(), mountDir, "etc/hosts", []byte(hostsContent), 0644); err != nil {
		return fmt.Errorf("failed to update hosts file: %w", err)
	}

	// Determine which network configuration system to use
	// For simplicity, let's assume Ubuntu server images use Netplan
	usesNetplan := true

	if usesNetplan {
		return applyNetplanConfig(ctx, imageTool, mountDir, config)
	} else {
		return applyInterfacesConfig(ctx, imageTool, mountDir, config)
	}
}

// applyNetplanConfig applies network configuration using Netplan
func applyNetplanConfig(ctx *gostate.ActionContext, imageTool tools.ImageTool, mountDir string, config NetworkConfig) error {
	// Parse IP and CIDR
	ipAddress := config.IPCIDR
	cidrSuffix := "/24" // Default

	if strings.Contains(ipAddress, "/") {
		parts := strings.Split(ipAddress, "/")
		ipAddress = parts[0]
		if len(parts) > 1 {
			cidrSuffix = "/" + parts[1]
		}
	}

	// Create Netplan YAML configuration
	netplanYaml := fmt.Sprintf(`# Network configuration generated by TuringPi
network:
  version: 2
  ethernets:
    eth0:
      dhcp4: false
      addresses: [%s%s]
      gateway4: %s
      nameservers:
        addresses: [%s]
`, ipAddress, cidrSuffix, config.Gateway, strings.Join(config.DNSServers, ", "))

	// Ensure Netplan directory exists
	netplanDir := filepath.Join(mountDir, "etc/netplan")
	os.MkdirAll(netplanDir, 0755)

	// Write the Netplan configuration
	if err := imageTool.WriteFile(context.Background(), mountDir, "etc/netplan/01-netcfg.yaml", []byte(netplanYaml), 0644); err != nil {
		return fmt.Errorf("failed to write netplan configuration: %w", err)
	}

	return nil
}

// applyInterfacesConfig applies network configuration using /etc/network/interfaces
func applyInterfacesConfig(ctx *gostate.ActionContext, imageTool tools.ImageTool, mountDir string, config NetworkConfig) error {
	// Parse IP and CIDR
	ipAddress := config.IPCIDR
	cidrSuffix := "/24" // Default

	if strings.Contains(ipAddress, "/") {
		parts := strings.Split(ipAddress, "/")
		ipAddress = parts[0]
		if len(parts) > 1 {
			cidrSuffix = "/" + parts[1]
		}
	}

	// Create interfaces file content
	interfacesContent := fmt.Sprintf(`# Network configuration generated by TuringPi
auto lo
iface lo inet loopback

auto eth0
iface eth0 inet static
  address %s
  netmask %s
  gateway %s
  dns-nameservers %s
`, ipAddress, convertCIDRToNetmask(cidrSuffix), config.Gateway, strings.Join(config.DNSServers, " "))

	// Ensure network directory exists
	networkDir := filepath.Join(mountDir, "etc/network")
	os.MkdirAll(networkDir, 0755)

	// Write the interfaces file
	if err := imageTool.WriteFile(context.Background(), mountDir, "etc/network/interfaces", []byte(interfacesContent), 0644); err != nil {
		return fmt.Errorf("failed to write interfaces configuration: %w", err)
	}

	return nil
}

// applyNetworkConfigInContainer applies network configuration in a container
func applyNetworkConfigInContainer(ctx *gostate.ActionContext, containerTool tools.ContainerTool, containerID string, config NetworkConfig) error {
	// Set hostname
	hostnameCmd := fmt.Sprintf("echo '%s' > /mnt/etc/hostname", config.Hostname)
	_, err := containerTool.RunCommand(context.Background(), containerID, []string{"bash", "-c", hostnameCmd})
	if err != nil {
		return fmt.Errorf("failed to set hostname in container: %w", err)
	}

	// Update hosts file
	hostsContent := fmt.Sprintf("127.0.0.1\tlocalhost\n127.0.1.1\t%s\n\n", config.Hostname)
	hostsCmd := fmt.Sprintf("echo '%s' > /mnt/etc/hosts", hostsContent)
	_, err = containerTool.RunCommand(context.Background(), containerID, []string{"bash", "-c", hostsCmd})
	if err != nil {
		return fmt.Errorf("failed to update hosts file in container: %w", err)
	}

	// Check if image uses Netplan
	checkNetplanCmd := "[ -d /mnt/etc/netplan ] && echo 'netplan' || echo 'interfaces'"
	output, err := containerTool.RunCommand(context.Background(), containerID, []string{"bash", "-c", checkNetplanCmd})
	if err != nil {
		return fmt.Errorf("failed to check netplan directory in container: %w", err)
	}

	// Determine which network configuration system to use
	usesNetplan := strings.TrimSpace(output) == "netplan"

	// Parse IP and CIDR
	ipAddress := config.IPCIDR
	cidrSuffix := "/24" // Default

	if strings.Contains(ipAddress, "/") {
		parts := strings.Split(ipAddress, "/")
		ipAddress = parts[0]
		if len(parts) > 1 {
			cidrSuffix = "/" + parts[1]
		}
	}

	if usesNetplan {
		// Create Netplan YAML configuration
		netplanYaml := fmt.Sprintf(`# Network configuration generated by TuringPi
network:
  version: 2
  ethernets:
    eth0:
      dhcp4: false
      addresses: [%s%s]
      gateway4: %s
      nameservers:
        addresses: [%s]
`, ipAddress, cidrSuffix, config.Gateway, strings.Join(config.DNSServers, ", "))

		// Write the Netplan configuration
		netplanCmd := fmt.Sprintf("mkdir -p /mnt/etc/netplan && echo '%s' > /mnt/etc/netplan/01-netcfg.yaml", netplanYaml)
		_, err = containerTool.RunCommand(context.Background(), containerID, []string{"bash", "-c", netplanCmd})
		if err != nil {
			return fmt.Errorf("failed to write netplan configuration in container: %w", err)
		}
	} else {
		// Create interfaces file content
		interfacesContent := fmt.Sprintf(`# Network configuration generated by TuringPi
auto lo
iface lo inet loopback

auto eth0
iface eth0 inet static
  address %s
  netmask %s
  gateway %s
  dns-nameservers %s
`, ipAddress, convertCIDRToNetmask(cidrSuffix), config.Gateway, strings.Join(config.DNSServers, " "))

		// Write the interfaces file
		interfacesCmd := fmt.Sprintf("mkdir -p /mnt/etc/network && echo '%s' > /mnt/etc/network/interfaces", interfacesContent)
		_, err = containerTool.RunCommand(context.Background(), containerID, []string{"bash", "-c", interfacesCmd})
		if err != nil {
			return fmt.Errorf("failed to write interfaces configuration in container: %w", err)
		}
	}

	return nil
}

// getNetworkConfig retrieves network configuration from the context
func getNetworkConfig(ctx *gostate.ActionContext, nodeID int) (NetworkConfig, error) {
	// Try to get network config from context
	networkConfig, err := store.Get[NetworkConfig](ctx.Store, "networkConfig")
	if err == nil {
		return networkConfig, nil
	}

	// If not found, check for individual fields
	hostname, err := store.GetOrDefault(ctx.Store, "hostname", fmt.Sprintf("node%d", nodeID))
	if err != nil {
		return NetworkConfig{}, fmt.Errorf("failed to get hostname: %w", err)
	}

	ipCIDR, err := store.Get[string](ctx.Store, "ipCIDR")
	if err != nil {
		return NetworkConfig{}, fmt.Errorf("IP address is required but not found in context: %w", err)
	}

	gateway, err := store.Get[string](ctx.Store, "gateway")
	if err != nil {
		return NetworkConfig{}, fmt.Errorf("gateway is required but not found in context: %w", err)
	}

	dnsServers, err := store.GetOrDefault(ctx.Store, "dnsServers", []string{"8.8.8.8", "8.8.4.4"})
	if err != nil {
		return NetworkConfig{}, fmt.Errorf("failed to get DNS servers: %w", err)
	}

	return NetworkConfig{
		Hostname:   hostname,
		IPCIDR:     ipCIDR,
		Gateway:    gateway,
		DNSServers: dnsServers,
	}, nil
}

// Helper function to create a container configuration for image operations
func createImageConfigContainer(imagePath, mountDir string) container.ContainerConfig {
	return container.ContainerConfig{
		Image:      "ubuntu:latest",
		Name:       "turingpi-network-config",
		Command:    []string{"sleep", "infinity"}, // Keep container running
		WorkDir:    "/workspace",
		Privileged: true, // Needed for mount operations
		Capabilities: []string{
			"SYS_ADMIN",
			"MKNOD",
		},
		Mounts: map[string]string{
			filepath.Dir(imagePath): "/images",
			mountDir:                "/output",
		},
	}
}

// parseKpartxOutput parses the output of kpartx to extract device path
func parseKpartxOutput(output string) (string, error) {
	// Example output: "add map loop0p1 (253:0): 0 194560 linear 7:0 2048"
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		if strings.HasPrefix(line, "add map ") {
			parts := strings.Fields(line)
			if len(parts) >= 3 {
				// Get the device name (e.g., loop0p1)
				deviceName := parts[2]
				return "/dev/mapper/" + deviceName, nil
			}
		}
	}

	return "", fmt.Errorf("could not find device path in kpartx output: %s", output)
}

// convertCIDRToNetmask converts a CIDR suffix to a netmask
func convertCIDRToNetmask(cidr string) string {
	// Remove the leading "/"
	cidr = strings.TrimPrefix(cidr, "/")

	// Convert to integer
	prefix, err := fmt.Sscanf(cidr, "%d", new(int))
	if err != nil || prefix < 0 || prefix > 32 {
		// Default to /24 if invalid
		prefix = 24
	}

	// Calculate netmask
	mask := ^((1 << (32 - prefix)) - 1)

	// Convert to dotted decimal
	return fmt.Sprintf("%d.%d.%d.%d",
		(mask>>24)&0xFF,
		(mask>>16)&0xFF,
		(mask>>8)&0xFF,
		mask&0xFF,
	)
}
