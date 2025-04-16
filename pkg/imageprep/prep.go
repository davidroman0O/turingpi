package imageprep

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	// "syscall" // Using /proc/mounts instead of syscall for mount check
	"time"
	// Need strconv for parsing kpartx output
)

// PrepareImageOptions holds the configuration for image preparation.
type PrepareImageOptions struct {
	SourceImgXZ    string
	NodeNum        int // Retained for default hostname logic if needed
	NodeIPCIDR     string
	NodeHostname   string
	NodeGateway    string
	NodeDNSServers string // Comma-separated
	CacheDir       string
}

// PrepareImage modifies an OS image with specific node configurations.
// It requires sudo privileges for mount/kpartx operations.
func PrepareImage(opts PrepareImageOptions) (string, error) {
	fmt.Println("--- Starting Image Preparation ---")

	// --- Validation and Setup ---
	if _, err := exec.LookPath("sudo"); err != nil {
		return "", fmt.Errorf("sudo command not found in PATH, which is required for image preparation")
	}
	if _, err := exec.LookPath("kpartx"); err != nil {
		return "", fmt.Errorf("kpartx command not found in PATH, please install it (e.g., apt install kpartx)")
	}
	if _, err := exec.LookPath("xz"); err != nil {
		return "", fmt.Errorf("xz command not found in PATH, please install it (e.g., apt install xz-utils)")
	}
	if opts.SourceImgXZ == "" || opts.NodeIPCIDR == "" || opts.NodeHostname == "" || opts.NodeGateway == "" || opts.NodeDNSServers == "" || opts.CacheDir == "" {
		return "", fmt.Errorf("missing required options for PrepareImage")
	}
	if !strings.Contains(opts.NodeIPCIDR, "/") {
		return "", fmt.Errorf("invalid NodeIPCIDR format, must include CIDR suffix (e.g., 192.168.1.101/24)")
	}

	sourceImgXZAbs, err := filepath.Abs(opts.SourceImgXZ)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for source image: %w", err)
	}
	if _, err := os.Stat(sourceImgXZAbs); os.IsNotExist(err) {
		return "", fmt.Errorf("source image '%s' not found", sourceImgXZAbs)
	}

	cacheDirAbs, err := filepath.Abs(opts.CacheDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for cache directory: %w", err)
	}
	if err := os.MkdirAll(cacheDirAbs, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory '%s': %w", cacheDirAbs, err)
	}

	// Construct expected output filename
	// Ensure hostname is safe for filenames
	safeHostname := strings.ReplaceAll(opts.NodeHostname, "/", "_") // Basic safety
	safeHostname = strings.ReplaceAll(safeHostname, "\\", "_")      // Correctly escaped backslash check
	outputFilename := fmt.Sprintf("%s.img.xz", safeHostname)
	preparedImagePath := filepath.Join(cacheDirAbs, outputFilename)

	// --- Cache Check ---
	if _, err := os.Stat(preparedImagePath); err == nil {
		fmt.Printf("Prepared image already exists in cache: %s\n", preparedImagePath)
		fmt.Println("Skipping preparation.")
		fmt.Println("--- Image Preparation Finished (Cached) ---")
		return preparedImagePath, nil
	} else if !os.IsNotExist(err) {
		// Error checking the cache file (permissions?)
		return "", fmt.Errorf("failed to check cache file '%s': %w", preparedImagePath, err)
	}

	fmt.Printf("Prepared image not found in cache. Proceeding with preparation...\n")

	// --- Temporary Directory ---
	tmpDir, err := os.MkdirTemp("", "rk1img-prep-")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	fmt.Printf("Created temporary directory: %s\n", tmpDir)

	// Define paths early for defer
	imgBaseName := strings.TrimSuffix(filepath.Base(sourceImgXZAbs), ".xz")
	imgPath := filepath.Join(tmpDir, imgBaseName)
	mountDir := filepath.Join(tmpDir, "mnt")

	// Defer cleanup logic
	defer func(dirToClean string, imgPathToClean string, mountPath string) {
		fmt.Printf("--- Starting deferred cleanup for %s ---\n", dirToClean)

		// Check if mounted before trying to unmount
		if isMounted(mountPath) {
			fmt.Printf("Attempting unmount: %s\n", mountPath)
			umountCmd := exec.Command("sudo", "umount", "-f", mountPath) // Add -f for force, might help in cleanup
			if output, err := umountCmd.CombinedOutput(); err != nil {
				fmt.Printf("Warning: Failed to unmount %s: %v\nOutput: %s\n", mountPath, err, string(output))
			} else {
				fmt.Println("Unmount successful.")
			}
		} else {
			fmt.Printf("Mount path %s not mounted, skipping unmount.\n", mountPath)
		}

		// Check if image path exists before running kpartx -d
		if _, statErr := os.Stat(imgPathToClean); statErr == nil {
			kpartxDCmd := exec.Command("sudo", "kpartx", "-d", imgPathToClean)
			fmt.Printf("Attempting kpartx cleanup: %s\n", kpartxDCmd.String())
			if output, err := kpartxDCmd.CombinedOutput(); err != nil {
				// Check if it failed because no mappings were found (exit code 1 often means this for kpartx -d)
				if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
					fmt.Printf("Warning: Failed to run kpartx -d %s: %v\nOutput: %s\n", imgPathToClean, err, string(output))
				} else {
					fmt.Println("kpartx -d indicated no mappings found (normal if already cleaned or failed before mapping).")
				}
			} else {
				fmt.Println("kpartx cleanup successful.")
			}
		} else {
			fmt.Printf("Skipping kpartx cleanup as image path %s doesn't exist.\n", imgPathToClean)
		}

		// Remove the whole temp directory
		fmt.Printf("Removing temporary directory %s...\n", dirToClean)
		if err := os.RemoveAll(dirToClean); err != nil {
			fmt.Printf("Warning: Failed to remove temporary directory %s: %v\n", dirToClean, err)
		} else {
			fmt.Println("Temporary directory removed.")
		}
		fmt.Println("--- Finished deferred cleanup ---")
	}(tmpDir, imgPath, mountDir) // Pass calculated paths to defer

	if err := os.MkdirAll(mountDir, 0755); err != nil {
		// Cleanup will run via defer
		return "", fmt.Errorf("failed to create mount directory '%s': %w", mountDir, err)
	}

	fmt.Printf("Uncompressed Image Path: %s\n", imgPath)
	fmt.Printf("Mount Path: %s\n", mountDir)

	// --- Decompression ---
	fmt.Println("==> Decompressing image...")
	// Use xz -dkc to decompress to stdout and redirect to file
	unxzCmd := exec.Command("xz", "-dkc", sourceImgXZAbs)
	imgFile, err := os.Create(imgPath)
	if err != nil {
		return "", fmt.Errorf("failed to create output image file '%s': %w", imgPath, err)
	}
	unxzCmd.Stdout = imgFile
	unxzCmd.Stderr = os.Stderr // Show errors directly

	if err := unxzCmd.Start(); err != nil {
		imgFile.Close()
		return "", fmt.Errorf("failed to start decompression: %w", err)
	}
	if err := unxzCmd.Wait(); err != nil {
		imgFile.Close()
		// Attempt to remove partial file on failure
		os.Remove(imgPath)
		return "", fmt.Errorf("decompression failed: %w", err)
	}
	imgFile.Close()
	fmt.Println("Decompression complete.")

	// --- Partition Mapping ---
	fmt.Println("==> Mapping partitions using kpartx...")
	// Use sudo for kpartx
	kpartxCmd := exec.Command("sudo", "kpartx", "-av", imgPath)
	fmt.Printf("Running: %s\n", kpartxCmd.String())
	kpartxOutput, err := kpartxCmd.CombinedOutput()
	fmt.Printf("kpartx output:\n%s\n", string(kpartxOutput))
	if err != nil {
		// kpartx can return non-zero if devices already exist, check output
		// Allow exit code 0 (success) or 1 (often means device exists or no new mappings)
		exitCode := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		if exitCode > 1 {
			// Only treat exit codes > 1 as definite failure here
			return "", fmt.Errorf("kpartx failed unexpectedly for '%s': %w\nOutput: %s", imgPath, err, string(kpartxOutput))
		}
		fmt.Println("kpartx returned non-zero (or zero), potentially indicating mappings were added or already exist. Continuing.")
	}

	// Parse kpartx output to find the root partition
	rootPartMapper, err := parseKpartxOutput(string(kpartxOutput))
	if err != nil {
		return "", fmt.Errorf("failed to find root partition from kpartx output: %w", err)
	}
	rootPartition := filepath.Join("/dev/mapper", rootPartMapper)
	fmt.Printf("Selected root partition mapper: %s\n", rootPartition)

	// Wait a moment for device nodes to potentially appear
	fmt.Println("Waiting for device node to appear...")
	if err := waitForDevice(rootPartition, 5*time.Second); err != nil {
		return "", fmt.Errorf("device mapper node '%s' did not appear: %w", rootPartition, err)
	}
	fmt.Println("Device node exists.")

	// --- Mounting ---
	fmt.Println("==> Mounting root filesystem...")
	mountCmd := exec.Command("sudo", "mount", rootPartition, mountDir)
	fmt.Printf("Running: %s\n", mountCmd.String())
	if output, err := mountCmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to mount '%s' on '%s': %w\nOutput: %s", rootPartition, mountDir, err, string(output))
	}
	fmt.Println("Mount successful.")

	// --- Modifying Files ---
	fmt.Println("==> Modifying configuration files...")

	// 1. Hostname
	hostnameFile := filepath.Join(mountDir, "etc", "hostname")
	fmt.Printf("Setting hostname in %s\n", hostnameFile)
	if _, statErr := os.Stat(hostnameFile); statErr == nil {
		if err := writeToFileAsRoot(hostnameFile, []byte(opts.NodeHostname+"\n"), 0644); err != nil {
			return "", fmt.Errorf("failed to write hostname file: %w", err)
		}
	} else if os.IsNotExist(statErr) {
		fmt.Printf("Warning: Hostname file %s not found. Skipping update.\n", hostnameFile)
	} else {
		return "", fmt.Errorf("failed to stat hostname file %s: %w", hostnameFile, statErr)
	}

	// 2. Netplan (Assuming Ubuntu/Debian structure)
	netplanDir := filepath.Join(mountDir, "etc", "netplan")
	netplanFile := filepath.Join(netplanDir, "01-turing-static.yaml") // Use a distinct name
	fmt.Printf("Configuring static IP via Netplan in %s\n", netplanFile)

	if _, statErr := os.Stat(netplanDir); statErr == nil {
		dnsYaml := "[" + strings.Join(strings.Split(opts.NodeDNSServers, ","), ", ") + "]" // Format for YAML
		netplanContent := fmt.Sprintf(
			`network:
  version: 2
  ethernets:
    eth0:
      dhcp4: no
      addresses:
        - %s
      gateway4: %s
      nameservers:
        addresses: %s
`, opts.NodeIPCIDR, opts.NodeGateway, dnsYaml)

		if err := writeToFileAsRoot(netplanFile, []byte(netplanContent), 0644); err != nil {
			return "", fmt.Errorf("failed to write netplan config: %w", err)
		}

		// Optional: Remove other default configs if they exist
		files, _ := filepath.Glob(filepath.Join(netplanDir, "*.yaml"))
		for _, f := range files {
			baseName := filepath.Base(f)
			// Be careful not to delete our own file or READMEs
			if baseName != filepath.Base(netplanFile) && !strings.EqualFold(baseName, "readme.md") && !strings.EqualFold(baseName, "readme.txt") {
				fmt.Printf("Removing potentially conflicting Netplan file: %s\n", f)
				absRmPath, _ := filepath.Abs(f) // Use absolute path for rm target
				rmCmd := exec.Command("sudo", "rm", "-f", absRmPath)
				fmt.Printf("Running: %s\n", rmCmd.String())
				if output, err := rmCmd.CombinedOutput(); err != nil {
					// Log warning, but don't fail the whole process for this
					fmt.Printf("Warning: Failed to remove old netplan file %s: %v\nOutput: %s\n", absRmPath, err, string(output))
				}
			}
		}

	} else if os.IsNotExist(statErr) {
		fmt.Printf("Warning: Netplan directory %s not found. Skipping network configuration.\n", netplanDir)
	} else {
		return "", fmt.Errorf("failed to stat netplan directory %s: %w", netplanDir, statErr)
	}

	fmt.Println("File modifications complete.")

	// --- Sync filesystem before unmount ---
	fmt.Println("==> Syncing filesystem...")
	syncCmd := exec.Command("sync") // sync usually doesn't need sudo
	if err := syncCmd.Run(); err != nil {
		fmt.Printf("Warning: sync command failed: %v\n", err) // Non-critical usually
	} else {
		fmt.Println("Sync successful.")
	}

	// Unmount and kpartx cleanup will be handled by the defer function

	// --- Recompression ---
	fmt.Printf("==> Recompressing image to %s...\n", preparedImagePath)
	// Use xz -zck6 to compress from file to stdout, redirect to output file (Level 6 is default, less memory intensive than 9)
	// Use nice to lower priority if needed: exec.Command("nice", "-n", "10", "xz", ...)
	xzCmd := exec.Command("xz", "-zck6", imgPath) // Use default compression (level 6), keep original
	outFile, err := os.Create(preparedImagePath)
	if err != nil {
		return "", fmt.Errorf("failed to create output compressed file '%s': %w", preparedImagePath, err)
	}

	xzCmd.Stdout = outFile
	xzCmd.Stderr = os.Stderr // Show compression errors

	fmt.Printf("Starting compression command: %s\n", xzCmd.String())
	startTime := time.Now()

	if err := xzCmd.Start(); err != nil {
		outFile.Close()
		return "", fmt.Errorf("failed to start compression: %w", err)
	}
	if err := xzCmd.Wait(); err != nil {
		outFile.Close()
		// Attempt to remove partially created file on failure
		fmt.Printf("Compression failed, removing partial file: %s\n", preparedImagePath)
		os.Remove(preparedImagePath)
		return "", fmt.Errorf("compression failed: %w", err)
	}
	outFile.Close() // Close explicitly after Wait() succeeds

	duration := time.Since(startTime)
	fmt.Printf("Recompression complete (took %s).\n", duration)

	fmt.Println("--- Image Preparation Finished Successfully ---")
	return preparedImagePath, nil
}

// parseKpartxOutput attempts to find the most likely root partition mapper name.
func parseKpartxOutput(output string) (string, error) {
	var p1Mapper, pOtherMapper string
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		// Example lines:
		// add map loop0p1 (254:0): 0 524288 linear /dev/loop0 2048
		// add map loop0p2 (254:1): 0 15097856 linear /dev/loop0 526336
		// loop0p1 : 0 524288 linear /dev/loop0 2048
		parts := strings.Fields(lowerLine)
		mapperPart := ""

		for _, p := range parts {
			// Find the part that looks like loopXpY
			if strings.HasPrefix(p, "loop") && strings.Contains(p, "p") {
				mapperPart = p
				break
			}
		}

		if mapperPart == "" {
			continue // Skip lines without a clear mapper part
		}

		// Clean up potential surrounding characters like parentheses
		mapperPart = strings.Trim(mapperPart, "():")

		if strings.HasSuffix(mapperPart, "p1") {
			p1Mapper = mapperPart
		} else if strings.Contains(mapperPart, "p") {
			// It's a partition other than p1
			if pOtherMapper == "" { // Take the first non-p1 partition found
				pOtherMapper = mapperPart
			}
		}
	}

	if pOtherMapper != "" {
		fmt.Printf("parseKpartxOutput: Found non-p1 partition: %s\n", pOtherMapper)
		return pOtherMapper, nil
	}
	if p1Mapper != "" {
		fmt.Printf("parseKpartxOutput: Found only p1 partition: %s\n", p1Mapper)
		return p1Mapper, nil
	}

	return "", fmt.Errorf("no suitable partition (e.g., loopXpY) found in kpartx output:\n%s", output)
}

// waitForDevice polls for the existence of a device node file.
func waitForDevice(devicePath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(200 * time.Millisecond) // Check frequently
	defer ticker.Stop()

	for {
		select {
		case <-time.After(time.Until(deadline)):
			// Check one last time after timeout
			if _, err := os.Stat(devicePath); err == nil {
				return nil
			}
			return fmt.Errorf("timed out after %v waiting for %s", timeout, devicePath)
		case <-ticker.C:
			if _, err := os.Stat(devicePath); err == nil {
				return nil // Device found
			}
			// Continue loop if not found yet
		}
	}
}

// writeToFileAsRoot writes content to a file using sudo tee.
func writeToFileAsRoot(filePath string, content []byte, perm os.FileMode) error {
	absPath, err := filepath.Abs(filePath) // Ensure absolute path for sudo
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %w", filePath, err)
	}

	cmd := exec.Command("sudo", "tee", absPath)
	cmd.Stdin = strings.NewReader(string(content))
	fmt.Printf("Running: %s\n", cmd.String())
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("sudo tee failed for %s: %w\nOutput: %s", absPath, err, string(output))
	}

	// Set permissions afterwards using chmod
	chmodCmd := exec.Command("sudo", "chmod", fmt.Sprintf("%o", perm), absPath)
	fmt.Printf("Running: %s\n", chmodCmd.String())
	if output, err := chmodCmd.CombinedOutput(); err != nil {
		// Log as warning, tee likely set workable permissions based on umask
		fmt.Printf("Warning: sudo chmod failed for %s: %v\nOutput: %s\n", absPath, err, string(output))
	}
	return nil
}

// isMounted checks if a directory is a mount point using /proc/mounts.
func isMounted(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		fmt.Printf("Warning: Could not get absolute path for %s in isMounted: %v\n", path, err)
		return false
	}

	mountsFile := "/proc/mounts"
	if _, err := os.Stat(mountsFile); os.IsNotExist(err) {
		fmt.Println("Warning: /proc/mounts not found, cannot reliably check mount status.")
		return false
	}

	content, err := os.ReadFile(mountsFile)
	if err != nil {
		fmt.Printf("Warning: Could not read %s: %v\n", mountsFile, err)
		return false
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			// Field 2 is the mount point. Need to handle potential escaped sequences.
			// Go's strconv.Unquote can handle standard C-style escapes if needed,
			// but for typical paths, direct comparison is usually fine.
			// Example: /mnt/my\040folder might appear for "/mnt/my folder"
			// Let's assume simple paths for now. A more robust solution
			// would involve parsing escapes if necessary.
			mountPoint := fields[1]
			if mountPoint == absPath {
				return true
			}
		}
	}
	return false
}
