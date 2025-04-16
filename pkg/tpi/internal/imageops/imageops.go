package imageops

/*
TODO: Simplified but maintain Docker-based approach for cross-platform

This file contains operations for image preparation (mounting, partitioning,
filesystem manipulation). The Docker-based approach is maintained for
cross-platform compatibility.

Benefits of Docker-based approach:
- Encapsulates all Linux-specific tools and operations in a container
- Works consistently across platforms (Linux, macOS, Windows)
- Significantly reduces Go code complexity
- Improves maintainability
*/

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// DockerConfig holds the Docker execution configuration used for platform-independent image operations
var DockerConfig *platform.DockerExecutionConfig

// PrepareImageOptions contains all parameters needed to prepare an image
type PrepareImageOptions struct {
	SourceImgXZ  string   // Path to the source compressed image
	NodeNum      int      // Node number (used for default hostname if needed)
	IPAddress    string   // IP address without CIDR
	IPCIDRSuffix string   // CIDR suffix (e.g., "/24")
	Hostname     string   // Hostname to set
	Gateway      string   // Gateway IP address
	DNSServers   []string // List of DNS server IPs
	OutputDir    string   // Directory to store output image
	TempDir      string   // Directory for temporary processing
}

// InitDockerConfig initializes the Docker configuration for cross-platform operations
func InitDockerConfig(sourceDir, tempDir, outputDir string) {
	DockerConfig = platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)
}

// PrepareImage decompresses a disk image, modifies it with network settings, and recompresses it
func PrepareImage(opts PrepareImageOptions) (string, error) {
	// Validate inputs
	if opts.SourceImgXZ == "" {
		return "", fmt.Errorf("source image path is required")
	}
	if opts.IPAddress == "" {
		return "", fmt.Errorf("IP address is required")
	}
	if opts.Gateway == "" {
		return "", fmt.Errorf("gateway is required")
	}
	if len(opts.DNSServers) == 0 {
		return "", fmt.Errorf("at least one DNS server is required")
	}

	// Set default CIDR suffix if not provided
	if opts.IPCIDRSuffix == "" {
		opts.IPCIDRSuffix = "/24"
	}

	// Set default hostname if not provided
	if opts.Hostname == "" {
		opts.Hostname = fmt.Sprintf("node%d", opts.NodeNum)
	}

	// Set default output directory if not provided
	if opts.OutputDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		opts.OutputDir = filepath.Join(homeDir, ".cache", "turingpi", "images")
	}

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Set default temp directory if not provided
	if opts.TempDir == "" {
		opts.TempDir = os.TempDir()
	}

	// Output filename based on hostname
	outputFilename := fmt.Sprintf("%s.img.xz", opts.Hostname)
	outputPath := filepath.Join(opts.OutputDir, outputFilename)

	// If output file already exists, return it (caching)
	if _, err := os.Stat(outputPath); err == nil {
		fmt.Printf("Image already exists: %s\n", outputPath)
		return outputPath, nil
	}

	// Create a temp working directory
	tempWorkDir, err := os.MkdirTemp(opts.TempDir, "turingpi-image-*")
	if err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempWorkDir) // Clean up at the end

	// 1. Decompress the image
	fmt.Println("Decompressing source image...")
	sourceImgXZAbs, err := filepath.Abs(opts.SourceImgXZ)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}

	decompressedImgPath, err := DecompressImageXZ(sourceImgXZAbs, tempWorkDir)
	if err != nil {
		return "", fmt.Errorf("failed to decompress source image: %w", err)
	}

	// Calculate full CIDR address
	ipCIDR := opts.IPAddress + opts.IPCIDRSuffix

	// Check if we need to use Docker for platform-independence
	if !platform.IsLinux() && DockerConfig != nil {
		fmt.Println("Using Docker for image modification (step by step)...")

		// 2. Map partitions in Docker
		fmt.Println("Mapping partitions in Docker...")
		rootPartitionDev, err := MapPartitions(decompressedImgPath)
		if err != nil {
			return "", fmt.Errorf("failed to map partitions in Docker: %w", err)
		}

		// 3. Mount filesystem in Docker
		fmt.Println("Mounting filesystem in Docker...")
		mountDir := filepath.Join(tempWorkDir, "mnt")
		if err := os.MkdirAll(mountDir, 0755); err != nil {
			// Cleanup before returning
			CleanupPartitions(decompressedImgPath)
			return "", fmt.Errorf("failed to create mount directory: %w", err)
		}

		if err := MountFilesystem(rootPartitionDev, mountDir); err != nil {
			// Cleanup before returning
			CleanupPartitions(decompressedImgPath)
			return "", fmt.Errorf("failed to mount filesystem in Docker: %w", err)
		}

		// 4. Apply network configuration in Docker
		fmt.Println("Applying network configuration in Docker...")
		if err := ApplyNetworkConfig(mountDir, opts.Hostname, ipCIDR, opts.Gateway, opts.DNSServers); err != nil {
			// Cleanup before returning
			UnmountFilesystem(mountDir)
			CleanupPartitions(decompressedImgPath)
			return "", fmt.Errorf("failed to apply network configuration in Docker: %w", err)
		}

		// 5. Unmount filesystem in Docker
		fmt.Println("Unmounting filesystem in Docker...")
		if err := UnmountFilesystem(mountDir); err != nil {
			// Try to cleanup partitions even if unmount failed
			CleanupPartitions(decompressedImgPath)
			return "", fmt.Errorf("failed to unmount filesystem in Docker: %w", err)
		}

		// 6. Cleanup partition mapping in Docker
		fmt.Println("Cleaning up partition mapping in Docker...")
		if err := CleanupPartitions(decompressedImgPath); err != nil {
			return "", fmt.Errorf("failed to cleanup partitions in Docker: %w", err)
		}
	} else {
		// Native Linux approach - we'll use the local tools directly

		// 2. Map partitions
		fmt.Println("Mapping partitions...")
		rootPartitionDev, err := MapPartitions(decompressedImgPath)
		if err != nil {
			return "", fmt.Errorf("failed to map partitions: %w", err)
		}
		// Ensure partitions are unmapped at the end
		defer CleanupPartitions(decompressedImgPath)

		// 3. Mount the root filesystem
		mountDir := filepath.Join(tempWorkDir, "mnt")
		if err := os.MkdirAll(mountDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create mount directory: %w", err)
		}

		fmt.Printf("Mounting root partition: %s -> %s\n", rootPartitionDev, mountDir)
		if err := MountFilesystem(rootPartitionDev, mountDir); err != nil {
			return "", fmt.Errorf("failed to mount filesystem: %w", err)
		}
		// Ensure filesystem is unmounted at the end
		defer UnmountFilesystem(mountDir)

		// 4. Apply network configuration
		fmt.Println("Applying network configuration...")
		err = ApplyNetworkConfig(mountDir, opts.Hostname, ipCIDR, opts.Gateway, opts.DNSServers)
		if err != nil {
			return "", fmt.Errorf("failed to apply network configuration: %w", err)
		}

		// 5. Unmount filesystem
		fmt.Println("Unmounting filesystem...")
		if err := UnmountFilesystem(mountDir); err != nil {
			return "", fmt.Errorf("failed to unmount filesystem: %w", err)
		}

		// 6. Cleanup partition mapping
		fmt.Println("Cleaning up partition mapping...")
		if err := CleanupPartitions(decompressedImgPath); err != nil {
			return "", fmt.Errorf("failed to cleanup partitions: %w", err)
		}
	}

	// 7. Compress the modified image
	fmt.Println("Compressing the modified image...")
	finalXZPath := filepath.Join(opts.OutputDir, outputFilename)
	if err := RecompressImageXZ(decompressedImgPath, finalXZPath); err != nil {
		return "", fmt.Errorf("failed to recompress modified image: %w", err)
	}

	fmt.Printf("Successfully prepared image: %s\n", finalXZPath)
	return finalXZPath, nil
}

// runCommand runs a command and returns its output
func runCommand(cmd *exec.Cmd) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// DecompressImageXZ decompresses an XZ-compressed disk image
func DecompressImageXZ(sourceImgXZAbs, tmpDir string) (string, error) {
	// Create output filename by replacing .xz extension
	outputImgPath := filepath.Join(tmpDir, filepath.Base(strings.TrimSuffix(sourceImgXZAbs, ".xz")))

	// Check if we need to use Docker for platform-independence
	if !platform.IsLinux() && DockerConfig != nil {
		fmt.Println("Using Docker for decompression...")
		// Docker command to decompress: xz -dc source.img.xz > output.img
		dockerCmd := fmt.Sprintf("xz -dc %s > %s",
			filepath.Join("/images", filepath.Base(sourceImgXZAbs)),
			filepath.Join("/tmp", filepath.Base(outputImgPath)))

		if err := platform.ExecuteLinuxCommand(DockerConfig, "/tmp", dockerCmd); err != nil {
			return "", fmt.Errorf("Docker decompression failed: %w", err)
		}

		fmt.Printf("Decompression successful: %s\n", outputImgPath)
		return outputImgPath, nil
	}

	// Native Linux approach
	cmd := exec.Command("xz", "--decompress", "--keep", "--stdout", sourceImgXZAbs)
	outFile, err := os.Create(outputImgPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	fmt.Printf("Decompressing: %s -> %s\n", sourceImgXZAbs, outputImgPath)
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("decompression failed: %w - %s", err, stderr.String())
	}

	// Verify the file was created
	if _, err := os.Stat(outputImgPath); os.IsNotExist(err) {
		return "", fmt.Errorf("decompressed image not found at %s", outputImgPath)
	}

	return outputImgPath, nil
}

// MapPartitions uses kpartx to map disk partitions
func MapPartitions(imgPathAbs string) (string, error) {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && DockerConfig != nil {
		fmt.Println("Using Docker for partition mapping...")

		// Execute kpartx in Docker - just run kpartx and get its output
		dockerCmd := fmt.Sprintf("kpartx -av /tmp/%s", filepath.Base(imgPathAbs))

		output, err := platform.ExecuteLinuxCommandWithOutput(DockerConfig, filepath.Dir(imgPathAbs), dockerCmd)
		if err != nil {
			return "", fmt.Errorf("Docker partition mapping failed: %w", err)
		}

		// Parse output using the same helper as native Linux approach for consistency
		rootDevice, err := parseKpartxOutput(string(output))
		if err != nil {
			return "", fmt.Errorf("failed to parse Docker kpartx output: %w", err)
		}

		rootDevPath := fmt.Sprintf("/dev/mapper/%s", rootDevice)
		fmt.Printf("Docker mapped root partition: %s\n", rootDevPath)

		// No need to wait for device in Docker - it should be immediately available in the container
		return rootDevPath, nil
	}

	// Native Linux approach
	cmd := exec.Command("kpartx", "-av", imgPathAbs)
	output, err := runCommand(cmd)
	if err != nil {
		return "", fmt.Errorf("kpartx failed: %w", err)
	}

	// Parse output to get device name
	rootDevice, err := parseKpartxOutput(string(output))
	if err != nil {
		return "", fmt.Errorf("failed to parse kpartx output: %w", err)
	}

	// Wait for device to become available
	rootDevPath := fmt.Sprintf("/dev/mapper/%s", rootDevice)
	if err := waitForDevice(rootDevPath, 5*time.Second); err != nil {
		return "", fmt.Errorf("device not available: %w", err)
	}

	return rootDevPath, nil
}

// parseKpartxOutput parses kpartx output to extract root partition device path
func parseKpartxOutput(output string) (string, error) {
	// Example output:
	// add map loop1p1 (253:1): 0 524288 linear 7:1 8192
	// add map loop1p2 (253:2): 0 32768000 linear 7:1 532480
	//
	// For simplicity, we assume the first line is boot and second is root
	// This could be improved to detect partitions by examining sizes or file systems

	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return "", fmt.Errorf("unexpected kpartx output format (less than 2 lines)")
	}

	// Check second line for root partition
	rootLine := lines[1]
	parts := strings.Fields(rootLine)
	if len(parts) < 3 || !strings.HasPrefix(parts[0], "add") || !strings.HasPrefix(parts[2], "loop") {
		return "", fmt.Errorf("unexpected kpartx output format: %s", rootLine)
	}

	return parts[2], nil
}

// waitForDevice waits for a device to become available
func waitForDevice(devicePath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(devicePath); err == nil {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for device %s", devicePath)
}

// CleanupPartitions unmaps partitions
func CleanupPartitions(imgPathAbs string) error {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && DockerConfig != nil {
		fmt.Println("Using Docker for partition cleanup...")

		// Execute kpartx cleanup in Docker
		dockerCmd := fmt.Sprintf("kpartx -d /tmp/%s", filepath.Base(imgPathAbs))

		if err := platform.ExecuteLinuxCommand(DockerConfig, filepath.Dir(imgPathAbs), dockerCmd); err != nil {
			return fmt.Errorf("Docker partition cleanup failed: %w", err)
		}

		fmt.Println("Docker partition cleanup completed")
		return nil
	}

	// Native Linux approach
	cmd := exec.Command("kpartx", "-d", imgPathAbs)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to unmap partitions: %w", err)
	}
	return nil
}

// MountFilesystem mounts a filesystem
func MountFilesystem(partitionDevice, mountDir string) error {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && DockerConfig != nil {
		fmt.Println("Using Docker for filesystem mounting...")

		// Extract the partition name from the full path
		partName := filepath.Base(partitionDevice)

		// Execute mount in Docker
		dockerCmd := fmt.Sprintf("mkdir -p /mnt && mount /dev/mapper/%s /mnt", partName)

		if err := platform.ExecuteLinuxCommand(DockerConfig, filepath.Dir(mountDir), dockerCmd); err != nil {
			return fmt.Errorf("Docker filesystem mounting failed: %w", err)
		}

		fmt.Printf("Docker mounted %s to /mnt\n", partName)
		return nil
	}

	// Native Linux approach
	cmd := exec.Command("mount", partitionDevice, mountDir)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("mount failed: %w", err)
	}

	// Verify it's actually mounted
	if !isMounted(mountDir) {
		return fmt.Errorf("filesystem not mounted at %s", mountDir)
	}

	return nil
}

// isMounted checks if a path is a mountpoint
func isMounted(path string) bool {
	// Only for Linux - for non-Linux platforms, this is handled in the Docker container
	cmd := exec.Command("mountpoint", "-q", path)
	return cmd.Run() == nil
}

// UnmountFilesystem unmounts a filesystem
func UnmountFilesystem(mountDir string) error {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && DockerConfig != nil {
		fmt.Println("Using Docker for filesystem unmounting...")

		// Execute umount in Docker
		dockerCmd := "umount /mnt"

		if err := platform.ExecuteLinuxCommand(DockerConfig, filepath.Dir(mountDir), dockerCmd); err != nil {
			return fmt.Errorf("Docker filesystem unmounting failed: %w", err)
		}

		fmt.Println("Docker unmounted filesystem")
		return nil
	}

	// Native Linux approach
	// Check if actually mounted first
	if !isMounted(mountDir) {
		return nil // Already unmounted
	}

	cmd := exec.Command("umount", mountDir)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("umount failed: %w", err)
	}

	return nil
}

// writeToFileAsRoot writes content to a file as root
func writeToFileAsRoot(filePath string, content []byte, perm fs.FileMode) error {
	// Only for Linux - for non-Linux platforms, this is handled in the Docker container
	// Create temp file
	tempFile, err := os.CreateTemp(filepath.Dir(filePath), "tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	// Write content to temp file
	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tempFile.Close()

	// Set permissions on temp file
	if err := os.Chmod(tempPath, perm); err != nil {
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	// Move temp file to target path (requires root)
	cmd := exec.Command("sudo", "mv", tempPath, filePath)
	_, err = runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	return nil
}

// ApplyNetworkConfig applies network config to the mounted filesystem
func ApplyNetworkConfig(mountDir string, hostname string, ipCIDR string, gateway string, dnsServers []string) error {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && DockerConfig != nil {
		fmt.Println("Using Docker for network configuration...")

		// Set hostname
		hostnameCmd := fmt.Sprintf("echo '%s' > /mnt/etc/hostname", hostname)
		if err := platform.ExecuteLinuxCommand(DockerConfig, filepath.Dir(mountDir), hostnameCmd); err != nil {
			return fmt.Errorf("Docker hostname config failed: %w", err)
		}

		// Update /etc/hosts
		hostsContent := fmt.Sprintf("127.0.0.1\tlocalhost\n127.0.1.1\t%s\n\n", hostname)
		hostsCmd := fmt.Sprintf("echo '%s' > /mnt/etc/hosts", hostsContent)
		if err := platform.ExecuteLinuxCommand(DockerConfig, filepath.Dir(mountDir), hostsCmd); err != nil {
			return fmt.Errorf("Docker hosts file config failed: %w", err)
		}

		// Check if image uses Netplan
		checkNetplanCmd := "[ -d /mnt/etc/netplan ] && echo 'netplan' || echo 'interfaces'"
		netplanCheckOutput, err := platform.ExecuteLinuxCommandWithOutput(DockerConfig, filepath.Dir(mountDir), checkNetplanCmd)
		if err != nil {
			return fmt.Errorf("Docker netplan check failed: %w", err)
		}

		usesNetplan := strings.TrimSpace(string(netplanCheckOutput)) == "netplan"

		if usesNetplan {
			// Apply netplan config
			fmt.Println("Applying Netplan configuration in Docker...")

			dnsAddrs := strings.Join(dnsServers, ", ")
			netplanYaml := fmt.Sprintf(`# Generated by Turing Pi CLI
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

			netplanCmd := fmt.Sprintf("mkdir -p /mnt/etc/netplan && echo '%s' > /mnt/etc/netplan/01-netcfg.yaml", netplanYaml)
			if err := platform.ExecuteLinuxCommand(DockerConfig, filepath.Dir(mountDir), netplanCmd); err != nil {
				return fmt.Errorf("Docker netplan config failed: %w", err)
			}
		} else {
			// Apply interfaces config
			fmt.Println("Applying traditional network interfaces configuration in Docker...")

			// Extract IP and network bits
			parts := strings.Split(ipCIDR, "/")
			if len(parts) != 2 {
				return fmt.Errorf("invalid IP CIDR format: %s", ipCIDR)
			}
			ipAddr := parts[0]
			networkBits := parts[1]

			// Calculate netmask
			var netmask string
			switch networkBits {
			case "24":
				netmask = "255.255.255.0"
			case "16":
				netmask = "255.255.0.0"
			case "8":
				netmask = "255.0.0.0"
			default:
				return fmt.Errorf("unsupported network bits: %s", networkBits)
			}

			dnsLine := "dns-nameservers " + strings.Join(dnsServers, " ")
			interfacesContent := fmt.Sprintf(`# Generated by Turing Pi CLI
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

			interfacesCmd := fmt.Sprintf("mkdir -p /mnt/etc/network && echo '%s' > /mnt/etc/network/interfaces", interfacesContent)
			if err := platform.ExecuteLinuxCommand(DockerConfig, filepath.Dir(mountDir), interfacesCmd); err != nil {
				return fmt.Errorf("Docker interfaces config failed: %w", err)
			}
		}

		fmt.Println("Docker network configuration completed")
		return nil
	}

	// Native Linux approach
	// Set hostname
	hostnameFile := filepath.Join(mountDir, "etc/hostname")
	if err := writeToFileAsRoot(hostnameFile, []byte(hostname+"\n"), 0644); err != nil {
		return fmt.Errorf("failed to write hostname file: %w", err)
	}

	// Update /etc/hosts file
	hostsFile := filepath.Join(mountDir, "etc/hosts")
	hostsContent := fmt.Sprintf("127.0.0.1\tlocalhost\n127.0.1.1\t%s\n\n", hostname)
	if err := writeToFileAsRoot(hostsFile, []byte(hostsContent), 0644); err != nil {
		return fmt.Errorf("failed to update hosts file: %w", err)
	}

	// Check if image uses Netplan (Ubuntu/newer Debian) or traditional interfaces
	netplanDir := filepath.Join(mountDir, "etc/netplan")
	usesNetplan := false
	if _, err := os.Stat(netplanDir); err == nil {
		usesNetplan = true
	}

	if usesNetplan {
		// Image uses Netplan
		return applyNetplanConfig(mountDir, ipCIDR, gateway, dnsServers)
	} else {
		// Fall back to traditional network interfaces config (Debian)
		return applyInterfacesConfig(mountDir, ipCIDR, gateway, dnsServers)
	}
}

// applyNetplanConfig creates Netplan configuration files
func applyNetplanConfig(mountDir string, ipCIDR string, gateway string, dnsServers []string) error {
	netplanDir := filepath.Join(mountDir, "etc/netplan")

	// Create Netplan directory if it doesn't exist
	if err := os.MkdirAll(netplanDir, 0755); err != nil {
		return fmt.Errorf("failed to create netplan directory: %w", err)
	}

	// Get the list of current netplan files
	files, err := os.ReadDir(netplanDir)
	if err != nil {
		return fmt.Errorf("failed to read netplan directory: %w", err)
	}

	// Build netplan config
	dnsAddrs := strings.Join(dnsServers, ", ")
	netplanYaml := fmt.Sprintf(`# Generated by Turing Pi CLI
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

	// Either use an existing file or create a new one
	var configPath string
	if len(files) > 0 {
		configPath = filepath.Join(netplanDir, files[0].Name())
	} else {
		configPath = filepath.Join(netplanDir, "01-netcfg.yaml")
	}

	// Write netplan config
	if err := writeToFileAsRoot(configPath, []byte(netplanYaml), 0644); err != nil {
		return fmt.Errorf("failed to write netplan config: %w", err)
	}

	return nil
}

// applyInterfacesConfig creates traditional network interfaces configuration
func applyInterfacesConfig(mountDir string, ipCIDR string, gateway string, dnsServers []string) error {
	interfacesFile := filepath.Join(mountDir, "etc/network/interfaces")

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
	case "24":
		netmask = "255.255.255.0"
	case "16":
		netmask = "255.255.0.0"
	case "8":
		netmask = "255.0.0.0"
	default:
		return fmt.Errorf("unsupported network bits: %s", networkBits)
	}

	// Build interfaces config
	dnsLine := "dns-nameservers " + strings.Join(dnsServers, " ")
	interfacesContent := fmt.Sprintf(`# Generated by Turing Pi CLI
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

	// Write interfaces config
	if err := writeToFileAsRoot(interfacesFile, []byte(interfacesContent), 0644); err != nil {
		return fmt.Errorf("failed to write interfaces file: %w", err)
	}

	return nil
}

// RecompressImageXZ compresses a disk image with XZ
func RecompressImageXZ(modifiedImgPath, finalXZPath string) error {
	// Check if we need to use Docker for platform-independence
	if !platform.IsLinux() && DockerConfig != nil {
		fmt.Println("Using Docker for image compression...")
		// Docker command to compress: xz -zck6 input.img > output.img.xz
		dockerCmd := fmt.Sprintf("xz -zck6 %s > %s",
			filepath.Join("/tmp", filepath.Base(modifiedImgPath)),
			filepath.Join("/output", filepath.Base(finalXZPath)))

		if err := platform.ExecuteLinuxCommand(DockerConfig, "/tmp", dockerCmd); err != nil {
			return fmt.Errorf("Docker compression failed: %w", err)
		}

		fmt.Printf("Docker compression completed: %s\n", finalXZPath)
		return nil
	}

	// Native Linux approach
	// Create directory if it doesn't exist
	outputDir := filepath.Dir(finalXZPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Use xz to compress, with -6 compression level
	fmt.Printf("Compressing: %s -> %s (level 6)\n", modifiedImgPath, finalXZPath)
	cmd := exec.Command("xz", "-zck6", "--stdout", modifiedImgPath)

	// Direct output to file
	outFile, err := os.Create(finalXZPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("compression failed: %w - %s", err, stderr.String())
	}

	// Verify the compressed file exists
	if _, err := os.Stat(finalXZPath); os.IsNotExist(err) {
		return fmt.Errorf("compressed image not found at %s", finalXZPath)
	}

	return nil
}

// =============================================================================
// File Operations Interface and Implementations
// =============================================================================

// FileOperation defines the interface for file operations within the image
type FileOperation interface {
	Type() string
	Execute(mountDir string) error
}

// WriteOperation implements FileOperation for writing data to a file
type WriteOperation struct {
	RelativePath string
	Data         []byte
	Perm         os.FileMode
}

func (op WriteOperation) Type() string { return "write" }
func (op WriteOperation) Execute(mountDir string) error {
	filePath := filepath.Join(mountDir, op.RelativePath)
	fmt.Printf("Writing %d bytes to %s (Mode: %o)\n", len(op.Data), filePath, op.Perm)
	return WriteToImageFile(mountDir, op.RelativePath, op.Data, op.Perm)
}

// CopyLocalOperation implements FileOperation for copying files
type CopyLocalOperation struct {
	LocalSourcePath  string
	RelativeDestPath string
}

func (op CopyLocalOperation) Type() string { return "copyLocal" }
func (op CopyLocalOperation) Execute(mountDir string) error {
	return CopyFileToImage(mountDir, op.LocalSourcePath, op.RelativeDestPath)
}

// MkdirOperation implements FileOperation for creating directories
type MkdirOperation struct {
	RelativePath string
	Perm         os.FileMode
}

func (op MkdirOperation) Type() string { return "mkdir" }
func (op MkdirOperation) Execute(mountDir string) error {
	return MkdirInImage(mountDir, op.RelativePath, op.Perm)
}

// ChmodOperation implements FileOperation for changing permissions
type ChmodOperation struct {
	RelativePath string
	Perm         os.FileMode
}

func (op ChmodOperation) Type() string { return "chmod" }
func (op ChmodOperation) Execute(mountDir string) error {
	return ChmodInImage(mountDir, op.RelativePath, op.Perm)
}

// ExecuteFileOperationsParams contains parameters for ExecuteFileOperations
type ExecuteFileOperationsParams struct {
	MountDir   string
	Operations []FileOperation
}

// ExecuteFileOperations executes a batch of file operations
func ExecuteFileOperations(params ExecuteFileOperationsParams) error {
	if len(params.Operations) == 0 {
		fmt.Println("No file operations to execute.")
		return nil
	}

	fmt.Printf("Executing %d file operations...\n", len(params.Operations))

	// Check if we need to use Docker for platform-independence
	if !platform.IsLinux() && DockerConfig != nil {
		fmt.Println("Using Docker for file operations...")

		// For Docker, we execute each operation individually with its own command
		// First make sure the image is mounted
		mountCmd := "mkdir -p /mnt && " +
			"IMG_PATH=/tmp/" + filepath.Base(params.MountDir) + "/../img && " +
			"KPARTX_OUTPUT=$(kpartx -av $IMG_PATH) && " +
			"ROOT_PART_NAME=$(echo \"$KPARTX_OUTPUT\" | awk 'NR==2 {print $3}') && " +
			"ROOT_PART=/dev/mapper/$ROOT_PART_NAME && " +
			"mount $ROOT_PART /mnt"

		if err := platform.ExecuteLinuxCommand(DockerConfig, filepath.Dir(params.MountDir), mountCmd); err != nil {
			return fmt.Errorf("Docker initial mount failed: %w", err)
		}

		// Execute each operation individually
		for i, op := range params.Operations {
			fmt.Printf("Operation %d/%d: %s\n", i+1, len(params.Operations), op.Type())

			var dockerCmd string

			switch o := op.(type) {
			case WriteOperation:
				// Write file operation
				// Create temp file with content
				tempFile, err := os.CreateTemp("", "docker-file-op-*.tmp")
				if err != nil {
					return fmt.Errorf("failed to create temp file: %w", err)
				}
				tempPath := tempFile.Name()
				defer os.Remove(tempPath)

				if _, err := tempFile.Write(o.Data); err != nil {
					tempFile.Close()
					return fmt.Errorf("failed to write temp file: %w", err)
				}
				tempFile.Close()

				// Copy file to Docker container
				tempFileName := filepath.Base(tempPath)
				copyToDockerCmd := fmt.Sprintf("docker cp %s %s:/tmp/%s",
					tempPath,
					DockerConfig.ContainerName,
					tempFileName)

				copyCmd := exec.Command("bash", "-c", copyToDockerCmd)
				if _, err := runCommand(copyCmd); err != nil {
					return fmt.Errorf("failed to copy file to Docker: %w", err)
				}

				// Move file to target in Docker
				dockerCmd = fmt.Sprintf("mkdir -p $(dirname /mnt/%s) && cp /tmp/%s /mnt/%s && chmod %o /mnt/%s",
					o.RelativePath, tempFileName, o.RelativePath, o.Perm, o.RelativePath)

			case MkdirOperation:
				// Mkdir operation
				dockerCmd = fmt.Sprintf("mkdir -p /mnt/%s && chmod %o /mnt/%s",
					o.RelativePath, o.Perm, o.RelativePath)

			case ChmodOperation:
				// Chmod operation
				dockerCmd = fmt.Sprintf("chmod %o /mnt/%s", o.Perm, o.RelativePath)

			case CopyLocalOperation:
				// CopyLocalOperation - need to copy file to container first
				content, err := os.ReadFile(o.LocalSourcePath)
				if err != nil {
					return fmt.Errorf("failed to read local file %s: %w", o.LocalSourcePath, err)
				}

				tempFile, err := os.CreateTemp("", "docker-copy-op-*.tmp")
				if err != nil {
					return fmt.Errorf("failed to create temp file: %w", err)
				}
				tempPath := tempFile.Name()
				defer os.Remove(tempPath)

				if _, err := tempFile.Write(content); err != nil {
					tempFile.Close()
					return fmt.Errorf("failed to write temp file: %w", err)
				}
				tempFile.Close()

				// Copy file to Docker container
				tempFileName := filepath.Base(tempPath)
				copyToDockerCmd := fmt.Sprintf("docker cp %s %s:/tmp/%s",
					tempPath,
					DockerConfig.ContainerName,
					tempFileName)

				copyCmd := exec.Command("bash", "-c", copyToDockerCmd)
				if _, err := runCommand(copyCmd); err != nil {
					return fmt.Errorf("failed to copy file to Docker: %w", err)
				}

				// Move file to target in Docker
				dockerCmd = fmt.Sprintf("mkdir -p $(dirname /mnt/%s) && cp /tmp/%s /mnt/%s && chmod 0644 /mnt/%s",
					o.RelativeDestPath, tempFileName, o.RelativeDestPath, o.RelativeDestPath)
			}

			// Execute the operation in Docker
			if err := platform.ExecuteLinuxCommand(DockerConfig, filepath.Dir(params.MountDir), dockerCmd); err != nil {
				// Ensure cleanup before returning error
				cleanupCmd := "umount /mnt && kpartx -d $IMG_PATH"
				_ = platform.ExecuteLinuxCommand(DockerConfig, filepath.Dir(params.MountDir), cleanupCmd)
				return fmt.Errorf("Docker operation %d (%s) failed: %w", i+1, op.Type(), err)
			}
		}

		// Clean up - unmount and remove partition mapping
		cleanupCmd := "umount /mnt && kpartx -d $IMG_PATH"
		if err := platform.ExecuteLinuxCommand(DockerConfig, filepath.Dir(params.MountDir), cleanupCmd); err != nil {
			return fmt.Errorf("Docker cleanup failed: %w", err)
		}

		fmt.Println("Docker file operations completed successfully")
		return nil
	}

	// Direct execution for Linux
	for i, op := range params.Operations {
		fmt.Printf("Operation %d/%d: %s\n", i+1, len(params.Operations), op.Type())
		if err := op.Execute(params.MountDir); err != nil {
			return fmt.Errorf("operation %d failed: %w", i+1, err)
		}
	}

	return nil
}

// WriteToImageFile writes content to a file within the mounted image
func WriteToImageFile(mountDir, relativePath string, content []byte, perm os.FileMode) error {
	filePath := filepath.Join(mountDir, relativePath)
	fmt.Printf("Writing to file: %s\n", filePath)

	// Create parent directories if they don't exist
	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	return writeToFileAsRoot(filePath, content, perm)
}

// CopyFileToImage copies a local file into the mounted image
func CopyFileToImage(mountDir, localSourcePath, relativeDestPath string) error {
	// Read the local file
	content, err := os.ReadFile(localSourcePath)
	if err != nil {
		return fmt.Errorf("failed to read local file %s: %w", localSourcePath, err)
	}

	// Write to the destination in the image
	destPath := filepath.Join(mountDir, relativeDestPath)
	destDir := filepath.Dir(destPath)

	// Create destination directory if it doesn't exist
	cmd := exec.Command("sudo", "mkdir", "-p", destDir)
	if _, err := runCommand(cmd); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	return writeToFileAsRoot(destPath, content, 0644)
}

// MkdirInImage creates a directory within the mounted image
func MkdirInImage(mountDir, relativePath string, perm os.FileMode) error {
	dirPath := filepath.Join(mountDir, relativePath)
	cmd := exec.Command("sudo", "mkdir", "-p", dirPath)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	// Set permissions
	chmodCmd := exec.Command("sudo", "chmod", fmt.Sprintf("%o", perm), dirPath)
	_, err = runCommand(chmodCmd)
	if err != nil {
		return fmt.Errorf("failed to set directory permissions: %w", err)
	}

	return nil
}

// ChmodInImage changes permissions of a file/directory within the mounted image
func ChmodInImage(mountDir, relativePath string, perm os.FileMode) error {
	filePath := filepath.Join(mountDir, relativePath)
	cmd := exec.Command("sudo", "chmod", fmt.Sprintf("%o", perm), filePath)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	return nil
}
