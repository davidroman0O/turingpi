package imageops

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// PrepareImage implements ImageOpsAdapter.PrepareImage
func (a *imageOpsAdapter) PrepareImage(opts PrepareImageOptions) (string, error) {
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
		if a.dockerConfig == nil || a.dockerAdapter == nil {
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

		// 3. Mount filesystem
		fmt.Println("Mounting filesystem...")
		mountDir := filepath.Join(tempWorkDir, "mnt")
		if err := os.MkdirAll(mountDir, 0755); err != nil {
			// Cleanup before returning
			CleanupPartitions(decompressedImgPath)
			return "", fmt.Errorf("failed to create mount directory: %w", err)
		}

		if err := MountFilesystem(rootPartitionDev, mountDir); err != nil {
			// Cleanup before returning
			CleanupPartitions(decompressedImgPath)
			return "", fmt.Errorf("failed to mount filesystem: %w", err)
		}

		// 4. Apply network configuration
		fmt.Println("Applying network configuration...")
		if err := ApplyNetworkConfig(mountDir, opts.Hostname, ipCIDR, opts.Gateway, opts.DNSServers); err != nil {
			// Cleanup before returning
			UnmountFilesystem(mountDir)
			CleanupPartitions(decompressedImgPath)
			return "", fmt.Errorf("failed to apply network configuration: %w", err)
		}

		// 5. Unmount filesystem
		fmt.Println("Unmounting filesystem...")
		if err := UnmountFilesystem(mountDir); err != nil {
			// Try to cleanup partitions even if unmount failed
			CleanupPartitions(decompressedImgPath)
			return "", fmt.Errorf("failed to unmount filesystem: %w", err)
		}

		// 6. Cleanup partition mapping
		fmt.Println("Cleaning up partition mapping...")
		if err := CleanupPartitions(decompressedImgPath); err != nil {
			return "", fmt.Errorf("failed to cleanup partitions: %w", err)
		}
	}

	// 7. Recompress the modified image
	fmt.Println("Recompressing modified image...")
	if err := RecompressImageXZ(decompressedImgPath, outputPath); err != nil {
		return "", fmt.Errorf("failed to recompress modified image: %w", err)
	}

	fmt.Printf("Image preparation completed successfully: %s\n", outputPath)
	return outputPath, nil
}
