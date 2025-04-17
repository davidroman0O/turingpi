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

	"github.com/davidroman0O/turingpi/pkg/tpi/docker"
	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// DockerConfig holds the Docker execution configuration used for platform-independent image operations
var DockerConfig *platform.DockerExecutionConfig

// DockerContainerID stores the ID of the persistent container we create for operations
var DockerContainerID string

// dockerAdapter for performing operations - will be initialized for non-Linux platforms
var dockerAdapter *docker.DockerAdapter

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
func InitDockerConfig(sourceDir, tempDir, outputDir string) error {
	// We'll use the Docker adapter to manage Docker resources
	var err error

	fmt.Printf("InitDockerConfig called with:\n")
	fmt.Printf("  sourceDir: %s\n", sourceDir)
	fmt.Printf("  tempDir: %s\n", tempDir)
	fmt.Printf("  outputDir: %s\n", outputDir)

	// Clean up any existing adapter first to avoid resource leaks
	if dockerAdapter != nil {
		fmt.Println("Cleaning up existing Docker adapter before creating a new one")
		dockerAdapter.Cleanup()
		dockerAdapter = nil
		DockerConfig = nil
		DockerContainerID = ""
	}

	// Create a temporary config first to set the image name
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)

	// Set the image to turingpi-prepare which triggers special handling
	config.DockerImage = "turingpi-prepare"

	// Ensure we use a unique container name to avoid conflicts
	config.UseUniqueContainerName = true

	fmt.Printf("Docker configuration prepared:\n")
	fmt.Printf("  Image: %s\n", config.DockerImage)
	fmt.Printf("  Container Name: %s\n", config.ContainerName)
	fmt.Printf("  Source Dir: %s\n", config.SourceDir)
	fmt.Printf("  Temp Dir: %s\n", config.TempDir)
	fmt.Printf("  Output Dir: %s\n", config.OutputDir)
	fmt.Printf("  Additional Mounts: %d\n", len(config.AdditionalMounts))

	// Create the adapter with our custom config - with retries
	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		fmt.Printf("Attempting to create Docker adapter (attempt %d/%d)...\n", retry+1, maxRetries)
		dockerAdapter, err = docker.NewAdapterWithConfig(config)
		if err == nil {
			break
		}

		if retry < maxRetries-1 {
			waitTime := time.Duration(retry+1) * time.Second
			fmt.Printf("Docker connection attempt %d failed: %v. Retrying in %v...\n",
				retry+1, err, waitTime)
			time.Sleep(waitTime)
		}
	}

	if err != nil {
		// Clear any partially initialized adapter
		if dockerAdapter != nil {
			dockerAdapter.Cleanup()
		}
		dockerAdapter = nil
		DockerConfig = nil
		DockerContainerID = ""
		return fmt.Errorf("critical error: failed to initialize Docker after %d attempts: %w",
			maxRetries, err)
	}

	// Keep the DockerConfig for backward compatibility
	DockerConfig = dockerAdapter.Container.Config
	DockerContainerID = dockerAdapter.GetContainerID()

	fmt.Printf("Docker adapter initialized successfully.\n")
	fmt.Printf("  Container ID: %s\n", DockerContainerID)
	fmt.Printf("  Container Name: %s\n", dockerAdapter.GetContainerName())

	return nil
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
	if !platform.IsLinux() {
		// Docker is required for non-Linux platforms, ensure it's properly initialized
		if DockerConfig == nil || dockerAdapter == nil {
			return "", fmt.Errorf("critical error: Docker configuration is not initialized, but required for non-Linux platforms")
		}

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

	// NOTE: We're no longer cleaning up the Docker container here
	// to allow for subsequent operations. The caller should handle cleanup
	// when all operations are complete.
	//
	// Previously:
	// if !platform.IsLinux() && dockerAdapter != nil {
	//    dockerAdapter.Cleanup()
	// }

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

// MapPartitions uses kpartx to map disk partitions
func MapPartitions(imgPathAbs string) (string, error) {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() {
		if dockerAdapter == nil {
			return "", fmt.Errorf("critical error: Docker adapter is not initialized, but required for partition mapping on non-Linux platforms")
		}

		fmt.Println("Using Docker for partition mapping...")

		// Check if the image file exists in the container
		// First, copy the image to the Docker container if it's not already there
		fmt.Printf("DEBUG: Checking image path: %s\n", imgPathAbs)

		// The image should be in the /tmp directory inside the container
		// Get the basename of the image file
		imgBaseName := filepath.Base(imgPathAbs)
		containerImgPath := fmt.Sprintf("/tmp/%s", imgBaseName)

		// Check if the image exists inside the container
		checkCmd := fmt.Sprintf("ls -la %s", containerImgPath)
		checkOutput, err := dockerAdapter.ExecuteCommand(checkCmd)
		if err != nil {
			fmt.Printf("DEBUG: Image not found in container at %s\n", containerImgPath)
			fmt.Printf("DEBUG: Will copy image to container\n")

			// Copy image to container
			copyCmd := exec.Command("docker", "cp",
				imgPathAbs,
				fmt.Sprintf("%s:%s", dockerAdapter.GetContainerName(), containerImgPath))

			copyOutput, err := copyCmd.CombinedOutput()
			if err != nil {
				return "", fmt.Errorf("failed to copy image to container: %w, output: %s", err, string(copyOutput))
			}

			fmt.Printf("DEBUG: Copied image to container: %s -> %s\n", imgPathAbs, containerImgPath)
		} else {
			fmt.Printf("DEBUG: Image already exists in container: %s\n", checkOutput)
		}

		// Execute kpartx in Docker
		dockerCmd := fmt.Sprintf("kpartx -av %s", containerImgPath)
		fmt.Printf("DEBUG: Running in container: %s\n", dockerCmd)

		output, err := dockerAdapter.ExecuteCommand(dockerCmd)
		if err != nil {
			return "", fmt.Errorf("Docker partition mapping failed: %w", err)
		}

		fmt.Printf("DEBUG: kpartx output: %s\n", output)

		// Parse output using the same helper as native Linux approach for consistency
		rootDevice, err := parseKpartxOutput(output)
		if err != nil {
			return "", fmt.Errorf("failed to parse Docker kpartx output: %w", err)
		}

		rootDevPath := fmt.Sprintf("/dev/mapper/%s", rootDevice)
		fmt.Printf("Docker mapped root partition: %s\n", rootDevPath)

		// Check if the device was actually created
		checkDevCmd := fmt.Sprintf("ls -la %s", rootDevPath)
		checkDevOutput, err := dockerAdapter.ExecuteCommand(checkDevCmd)
		if err != nil {
			return "", fmt.Errorf("mapped device not found in container: %s", rootDevPath)
		}
		fmt.Printf("DEBUG: Device exists: %s\n", checkDevOutput)

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
	if !platform.IsLinux() && dockerAdapter != nil {
		fmt.Println("Using Docker for partition cleanup...")

		// Execute kpartx cleanup in Docker
		dockerCmd := fmt.Sprintf("kpartx -d /tmp/%s", filepath.Base(imgPathAbs))

		_, err := dockerAdapter.ExecuteCommand(dockerCmd)
		if err != nil {
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
	if !platform.IsLinux() && dockerAdapter != nil {
		fmt.Println("Using Docker for filesystem mounting...")

		// Extract the partition name from the full path
		partName := filepath.Base(partitionDevice)

		// Make sure the mount directory exists in the container
		prepareCmd := "mkdir -p /mnt"
		_, err := dockerAdapter.ExecuteCommand(prepareCmd)
		if err != nil {
			return fmt.Errorf("Docker failed to create mount directory: %w", err)
		}

		// Check if /dev/mapper exists and has the device
		checkMapperCmd := "ls -la /dev/mapper/"
		mapperOutput, err := dockerAdapter.ExecuteCommand(checkMapperCmd)
		fmt.Printf("DEBUG: Mapper directory contents: \n%s\n", mapperOutput)

		// Execute mount in Docker - use the full path to the device
		dockerCmd := fmt.Sprintf("mount %s /mnt", partitionDevice)
		fmt.Printf("DEBUG: Running in container: %s\n", dockerCmd)

		output, err := dockerAdapter.ExecuteCommand(dockerCmd)
		if err != nil {
			return fmt.Errorf("Docker filesystem mounting failed: %w\nOutput: %s", err, output)
		}

		// Verify the mount worked
		verifyCmd := "mount | grep /mnt"
		verifyOutput, err := dockerAdapter.ExecuteCommand(verifyCmd)
		if err != nil {
			return fmt.Errorf("mount verification failed: %w", err)
		}
		fmt.Printf("DEBUG: Mount verification: %s\n", verifyOutput)

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
	if !platform.IsLinux() && dockerAdapter != nil {
		fmt.Println("Using Docker for filesystem unmounting...")

		// Execute umount in Docker
		dockerCmd := "umount /mnt"

		_, err := dockerAdapter.ExecuteCommand(dockerCmd)
		if err != nil {
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
	if !platform.IsLinux() && dockerAdapter != nil {
		fmt.Println("Using Docker for network configuration...")

		// Set hostname
		hostnameCmd := fmt.Sprintf("echo '%s' > /mnt/etc/hostname", hostname)
		_, err := dockerAdapter.ExecuteCommand(hostnameCmd)
		if err != nil {
			return fmt.Errorf("Docker hostname config failed: %w", err)
		}

		// Update /etc/hosts
		hostsContent := fmt.Sprintf("127.0.0.1\tlocalhost\n127.0.1.1\t%s\n\n", hostname)
		hostsCmd := fmt.Sprintf("echo '%s' > /mnt/etc/hosts", hostsContent)
		_, err = dockerAdapter.ExecuteCommand(hostsCmd)
		if err != nil {
			return fmt.Errorf("Docker hosts file config failed: %w", err)
		}

		// Check if image uses Netplan
		checkNetplanCmd := "[ -d /mnt/etc/netplan ] && echo 'netplan' || echo 'interfaces'"
		netplanCheckOutput, err := dockerAdapter.ExecuteCommand(checkNetplanCmd)
		if err != nil {
			return fmt.Errorf("Docker netplan check failed: %w", err)
		}

		usesNetplan := strings.TrimSpace(netplanCheckOutput) == "netplan"

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
			_, err = dockerAdapter.ExecuteCommand(netplanCmd)
			if err != nil {
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
			_, err = dockerAdapter.ExecuteCommand(interfacesCmd)
			if err != nil {
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
	if !platform.IsLinux() && dockerAdapter != nil {
		fmt.Println("Using Docker for image compression...")
		// Docker command to compress: xz -zck6 input.img > output.img.xz
		// Note: In the turingpi-prepare Docker container:
		// - Source directory is mounted at /images
		// - Temp directory is mounted at /tmp
		// - Output directory is mounted at /prepared-images
		dockerCmd := fmt.Sprintf("xz -zck6 %s > %s",
			filepath.Join("/tmp", filepath.Base(modifiedImgPath)),
			filepath.Join("/prepared-images", filepath.Base(finalXZPath)))

		_, err := dockerAdapter.ExecuteCommand(dockerCmd)
		if err != nil {
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
	if !platform.IsLinux() && dockerAdapter != nil {
		fmt.Println("Using Docker for file operations...")

		// Just execute each operation directly since the filesystem is already mounted at /mnt
		// No need to mount again - we can reuse the existing mount
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
					dockerAdapter.Container.Config.ContainerName,
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
					dockerAdapter.Container.Config.ContainerName,
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
			output, err := dockerAdapter.ExecuteCommand(dockerCmd)
			if err != nil {
				return fmt.Errorf("Docker operation %d (%s) failed: %w\nOutput: %s", i+1, op.Type(), err, output)
			}

			fmt.Printf("Operation completed successfully: %s\n", output)
		}

		// No need to unmount or cleanup - caller will handle that

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

// DecompressImageXZ decompresses an XZ-compressed disk image
func DecompressImageXZ(sourceImgXZAbs, tmpDir string) (string, error) {
	// Create output filename by replacing .xz extension
	outputImgPath := filepath.Join(tmpDir, filepath.Base(strings.TrimSuffix(sourceImgXZAbs, ".xz")))

	// Check if we need to use Docker for platform-independence
	if !platform.IsLinux() {
		if dockerAdapter == nil {
			return "", fmt.Errorf("critical error: Docker adapter is not initialized, but required for image decompression on non-Linux platforms")
		}

		fmt.Println("Using Docker for decompression...")

		// Add a lot more debug information
		fmt.Printf("DEBUG: Source file absolute path: %s\n", sourceImgXZAbs)
		fmt.Printf("DEBUG: Source file directory: %s\n", filepath.Dir(sourceImgXZAbs))
		fmt.Printf("DEBUG: Source file base name: %s\n", filepath.Base(sourceImgXZAbs))
		fmt.Printf("DEBUG: Output file path: %s\n", outputImgPath)
		fmt.Printf("DEBUG: Output file base name: %s\n", filepath.Base(outputImgPath))
		fmt.Printf("DEBUG: Container ID: %s\n", dockerAdapter.GetContainerID())
		fmt.Printf("DEBUG: Container Name: %s\n", dockerAdapter.GetContainerName())
		fmt.Printf("DEBUG: Docker Config Source Dir: %s\n", dockerAdapter.Container.Config.SourceDir)
		fmt.Printf("DEBUG: Docker Config Temp Dir: %s\n", dockerAdapter.Container.Config.TempDir)
		fmt.Printf("DEBUG: Docker Config Output Dir: %s\n", dockerAdapter.Container.Config.OutputDir)

		// Check disk space in Docker
		diskSpaceOutput, err := dockerAdapter.ExecuteCommand("df -h")
		fmt.Printf("DEBUG: Docker disk space:\n%s\n", diskSpaceOutput)

		// First ensure that the /workspace directory exists and is writable in the container
		prepareCmd := "mkdir -p /workspace && chmod 777 /workspace && ls -la / | grep workspace"
		workspaceOutput, err := dockerAdapter.ExecuteCommand(prepareCmd)
		if err != nil {
			return "", fmt.Errorf("Failed to prepare /workspace directory: %w", err)
		}
		fmt.Printf("DEBUG: Workspace directory: %s\n", workspaceOutput)

		// Due to the disk space constraints, let's try to decompress directly on the host
		// rather than in the Docker container
		fmt.Printf("DEBUG: Attempting host-based decompression due to disk space concerns\n")

		// First, copy the compressed file to the host temporary directory if it's not already there
		hostSourcePath := sourceImgXZAbs
		if !strings.HasPrefix(sourceImgXZAbs, tmpDir) {
			hostTempSourcePath := filepath.Join(tmpDir, filepath.Base(sourceImgXZAbs))
			if _, err := os.Stat(hostTempSourcePath); os.IsNotExist(err) {
				fmt.Printf("DEBUG: Copying source file to temporary directory: %s -> %s\n", sourceImgXZAbs, hostTempSourcePath)
				data, err := os.ReadFile(sourceImgXZAbs)
				if err != nil {
					return "", fmt.Errorf("Failed to read source file: %w", err)
				}

				err = os.WriteFile(hostTempSourcePath, data, 0644)
				if err != nil {
					return "", fmt.Errorf("Failed to write source file to temp dir: %w", err)
				}
			}
			hostSourcePath = hostTempSourcePath
		}

		// Try decompression on the host
		fmt.Printf("DEBUG: Attempting decompression on host: %s -> %s\n", hostSourcePath, outputImgPath)

		// Create command to decompress on host
		hostCmd := exec.Command("xz", "--decompress", "--keep", "--stdout", hostSourcePath)
		outFile, err := os.Create(outputImgPath)
		if err != nil {
			return "", fmt.Errorf("Failed to create output file: %w", err)
		}
		defer outFile.Close()

		hostCmd.Stdout = outFile
		var stderr bytes.Buffer
		hostCmd.Stderr = &stderr

		err = hostCmd.Run()
		if err != nil {
			fmt.Printf("DEBUG: Host decompression failed: %v - %s\n", err, stderr.String())
			fmt.Printf("DEBUG: Falling back to Docker-based decompression\n")

			// Since we're in a disk space constrained environment,
			// Try to decompress and process in smaller chunks directly to save space
			fallingBackCmd := fmt.Sprintf("echo 'Using space-efficient approach' && xz --decompress --keep --stdout %s > %s && ls -lah %s",
				filepath.Join("/tmp", filepath.Base(sourceImgXZAbs)),
				outputImgPath,
				outputImgPath)

			output, err := dockerAdapter.ExecuteCommand(fallingBackCmd)
			if err != nil {
				fmt.Printf("DEBUG: Docker decompression failed too: %s\n", output)
				return "", fmt.Errorf("Both host and Docker decompression failed: %w", err)
			}

			fmt.Printf("DEBUG: Docker decompression output: %s\n", output)

			// Check if the file was created in the container
			checkCmd := fmt.Sprintf("ls -la %s", outputImgPath)
			checkOutput, err := dockerAdapter.ExecuteCommand(checkCmd)
			if err != nil {
				fmt.Printf("DEBUG: File does not exist in container: %s\n", checkOutput)
				return "", fmt.Errorf("Decompressed file not found in container")
			}

			fmt.Printf("DEBUG: File exists in container: %s\n", checkOutput)

			// Success - the file is decompressed in the container
			// We need to keep it there rather than copy it back to conserve disk space
			// Return a special path indicating this file is in the container
			return "DOCKER:" + outputImgPath, nil
		}

		// Host decompression succeeded
		fmt.Printf("DEBUG: Host decompression succeeded\n")

		// Verify the file was created
		if _, err := os.Stat(outputImgPath); os.IsNotExist(err) {
			return "", fmt.Errorf("Decompressed image not found at %s after host decompression", outputImgPath)
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

// PrepareImageSimple is a simplified version that follows the prep.go approach
// It decompresses, modifies and recompresses the image using Docker only where needed
func PrepareImageSimple(opts PrepareImageOptions) (string, error) {
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

	// Path to the uncompressed image
	sourceImgXZAbs, err := filepath.Abs(opts.SourceImgXZ)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path: %w", err)
	}
	imgPath := filepath.Join(tempWorkDir, filepath.Base(strings.TrimSuffix(sourceImgXZAbs, ".xz")))

	// Calculate full CIDR address
	ipCIDR := opts.IPAddress + opts.IPCIDRSuffix

	// Check if we're running on Linux
	if platform.IsLinux() {
		// --- Native Linux approach ---
		fmt.Println("Running on Linux, using native tools")

		// 1. Decompress image
		fmt.Println("Decompressing image...")
		cmd := exec.Command("xz", "--decompress", "--keep", "--stdout", sourceImgXZAbs)
		outFile, err := os.Create(imgPath)
		if err != nil {
			return "", fmt.Errorf("failed to create output file: %w", err)
		}

		cmd.Stdout = outFile
		var stderr bytes.Buffer
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			outFile.Close()
			return "", fmt.Errorf("decompression failed: %w - %s", err, stderr.String())
		}
		outFile.Close()
		fmt.Printf("Decompressed to: %s\n", imgPath)

		// 2. Map partitions
		fmt.Println("Mapping partitions...")
		rootPartition, err := MapPartitions(imgPath)
		if err != nil {
			return "", fmt.Errorf("failed to map partitions: %w", err)
		}
		defer CleanupPartitions(imgPath) // Ensure cleanup

		// 3. Mount filesystem
		mountDir := filepath.Join(tempWorkDir, "mnt")
		if err := os.MkdirAll(mountDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create mount dir: %w", err)
		}

		if err := MountFilesystem(rootPartition, mountDir); err != nil {
			return "", fmt.Errorf("failed to mount filesystem: %w", err)
		}
		defer UnmountFilesystem(mountDir) // Ensure unmount

		// 4. Apply network configuration
		fmt.Println("Applying network configuration...")
		if err := ApplyNetworkConfig(mountDir, opts.Hostname, ipCIDR, opts.Gateway, opts.DNSServers); err != nil {
			return "", fmt.Errorf("failed to apply network config: %w", err)
		}

		// 5. Recompress
		fmt.Println("Recompressing image...")
		if err := RecompressImageXZ(imgPath, outputPath); err != nil {
			return "", fmt.Errorf("failed to recompress image: %w", err)
		}

	} else {
		// --- Docker approach ---
		if dockerAdapter == nil {
			return "", fmt.Errorf("Docker adapter not initialized but required for non-Linux platforms")
		}

		fmt.Println("Running on non-Linux platform, using Docker")

		// Create a simple shell script to perform the operations
		scriptPath := filepath.Join(tempWorkDir, "prepare_image.sh")

		// Format DNS servers for the script
		dnsServerList := strings.Join(opts.DNSServers, ",")

		// Create the shell script content
		scriptContent := fmt.Sprintf(`#!/bin/bash
set -e

SOURCE_XZ=/images/%s
DEST_IMG=/workspace/image.img
MOUNT_DIR=/workspace/mnt
OUTPUT_XZ=/prepared-images/%s

# 1. Decompress image
echo "==> Decompressing image..."
xz -dc $SOURCE_XZ > $DEST_IMG

# 2. Map partitions
echo "==> Mapping partitions..."
KPARTX_OUTPUT=$(kpartx -av $DEST_IMG)
echo "$KPARTX_OUTPUT"
ROOT_PART_NAME=$(echo "$KPARTX_OUTPUT" | awk 'NR==2 {print $3}')
ROOT_PART=/dev/mapper/$ROOT_PART_NAME

# 3. Mount filesystem
echo "==> Mounting filesystem..."
mkdir -p $MOUNT_DIR
mount $ROOT_PART $MOUNT_DIR

# 4. Apply network configuration
echo "==> Configuring system..."
# 4.1 Set hostname
echo "%s" > $MOUNT_DIR/etc/hostname

# 4.2 Apply netplan config
mkdir -p $MOUNT_DIR/etc/netplan
cat > $MOUNT_DIR/etc/netplan/01-turing-static.yaml << EOF
network:
  version: 2
  ethernets:
    eth0:
      dhcp4: no
      addresses:
        - %s
      gateway4: %s
      nameservers:
        addresses: [%s]
EOF

# 5. Unmount and cleanup
echo "==> Cleaning up..."
umount $MOUNT_DIR
kpartx -d $DEST_IMG

# 6. Recompress
echo "==> Recompressing image..."
xz -zc $DEST_IMG > $OUTPUT_XZ

echo "==> Image preparation complete!"
`,
			filepath.Base(sourceImgXZAbs),
			filepath.Base(outputPath),
			opts.Hostname,
			ipCIDR,
			opts.Gateway,
			dnsServerList)

		// Write the script to the temp directory
		if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
			return "", fmt.Errorf("failed to create preparation script: %w", err)
		}

		// Copy the script to the container
		scriptName := filepath.Base(scriptPath)
		containerScriptPath := filepath.Join("/workspace", scriptName)

		copyCmd := exec.Command("docker", "cp", scriptPath,
			fmt.Sprintf("%s:%s", dockerAdapter.GetContainerName(), containerScriptPath))
		if output, err := copyCmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("failed to copy script to container: %w, output: %s", err, string(output))
		}

		// Execute the script in the container
		fmt.Println("Executing image preparation script in Docker...")
		scriptCmd := fmt.Sprintf("bash %s", containerScriptPath)
		_, err = dockerAdapter.ExecuteCommand(scriptCmd)
		if err != nil {
			return "", fmt.Errorf("failed to execute preparation script in Docker: %w", err)
		}

		// The output file should now be in the prepared-images directory
		fmt.Printf("Docker preparation completed successfully. Output: %s\n", outputPath)
	}

	// Final verification
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return "", fmt.Errorf("output file not found at %s after preparation", outputPath)
	}

	fmt.Printf("Image preparation completed successfully: %s\n", outputPath)
	return outputPath, nil
}

// DockerAdapter returns the current Docker adapter instance
// Can be used for cleanup after all operations are complete
// This is safe to call even if the adapter is nil
func DockerAdapter() *docker.DockerAdapter {
	return dockerAdapter
}
