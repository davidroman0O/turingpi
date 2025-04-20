package operations

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
)

// DockerExecutorAdapter adapts DockerAdapter to CommandExecutor interface
type DockerExecutorAdapter struct {
	adapter *container.DockerAdapter
}

// Execute implements CommandExecutor.Execute
func (d *DockerExecutorAdapter) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmdArgs := append([]string{name}, args...)
	output, err := d.adapter.ExecuteCommand(cmdArgs)
	return []byte(output), err
}

// ExecuteWithInput implements CommandExecutor.ExecuteWithInput
func (d *DockerExecutorAdapter) ExecuteWithInput(ctx context.Context, input string, name string, args ...string) ([]byte, error) {
	// Create a temporary file with the input
	tmpFile := "/tmp/input_file"
	// Escape the input to prevent command injection
	escapedInput := strings.ReplaceAll(input, "'", "'\\''")
	if _, err := d.Execute(ctx, "bash", "-c", fmt.Sprintf("echo -n '%s' > %s", escapedInput, tmpFile)); err != nil {
		return nil, fmt.Errorf("failed to create input file: %w", err)
	}

	// Execute command with input - escape the command and arguments to prevent injection
	escapedName := strings.ReplaceAll(name, "'", "'\\''")
	escapedArgs := make([]string, len(args))
	for i, arg := range args {
		escapedArgs[i] = strings.ReplaceAll(arg, "'", "'\\''")
	}

	shellCmd := fmt.Sprintf("cat %s | '%s' %s",
		tmpFile,
		escapedName,
		strings.Join(escapedArgs, "' '"))

	cmdArgs := []string{"bash", "-c", shellCmd}

	output, err := d.adapter.ExecuteCommand(cmdArgs)

	// Clean up - intentionally ignoring errors from cleanup
	_, _ = d.Execute(ctx, "rm", "-f", tmpFile)

	return []byte(output), err
}

// ExecuteInPath implements CommandExecutor.ExecuteInPath
func (d *DockerExecutorAdapter) ExecuteInPath(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	// Ensure directory exists - escape the directory path
	escapedDir := strings.ReplaceAll(dir, "'", "'\\''")
	if _, err := d.Execute(ctx, "mkdir", "-p", dir); err != nil {
		return nil, fmt.Errorf("failed to create directory: %w", err)
	}

	// Execute in directory - escape the command and arguments
	escapedName := strings.ReplaceAll(name, "'", "'\\''")
	escapedArgs := make([]string, len(args))
	for i, arg := range args {
		escapedArgs[i] = strings.ReplaceAll(arg, "'", "'\\''")
	}

	shellCmd := fmt.Sprintf("cd '%s' && '%s' %s",
		escapedDir,
		escapedName,
		strings.Join(escapedArgs, "' '"))

	cmdArgs := []string{"bash", "-c", shellCmd}

	output, err := d.adapter.ExecuteCommand(cmdArgs)
	return []byte(output), err
}

// setupExecutor creates an appropriate CommandExecutor
// - On Linux, uses native commands
// - On non-Linux, uses Docker
func setupExecutor(t *testing.T) (CommandExecutor, func(), error) {
	// On Linux, use the native executor
	if runtime.GOOS == "linux" {
		t.Log("Using native Linux executor")
		// No cleanup needed for native executor
		return &NativeExecutor{}, func() {}, nil
	}

	// On non-Linux, we must use Docker
	t.Log("Using Docker executor on non-Linux platform")

	// Create temporary directories for container mounts
	tempDir, err := os.MkdirTemp("", "turingpi-test")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create a unique container name with timestamp and test name - use shortened timestamp for readability
	testName := strings.ReplaceAll(t.Name(), "/", "-")
	testName = strings.ReplaceAll(testName, " ", "_")
	containerName := fmt.Sprintf("turingpi-test-%s-%d", testName, time.Now().Unix())

	// Enforce a maximum container name length (Docker has a limit)
	if len(containerName) > 63 {
		containerName = containerName[:63]
	}

	t.Logf("Creating test container: %s", containerName)

	// Get test ID to embed in environment variable for easier tracking
	testID := fmt.Sprintf("%s-%d", t.Name(), time.Now().UnixNano())

	// Config for Docker container
	config := &platform.DockerExecutionConfig{
		DockerImage:   "ubuntu:latest",
		ContainerName: containerName,
		TempDir:       tempDir,
		OutputDir:     tempDir,
		SourceDir:     tempDir,
	}

	// Set up variables to track if this container was already cleaned up
	var cleanedUp bool
	var cleanupMutex sync.Mutex

	// Create a more aggressive direct cleanup function that will
	// run as soon as possible when signaled
	containerCleanupFunc := func(containerID string) {
		cleanupMutex.Lock()
		defer cleanupMutex.Unlock()

		if cleanedUp {
			return
		}

		cleanedUp = true

		if containerID != "" {
			// First try direct Docker CLI cleanup as it's fastest
			removeCmd := exec.Command("docker", "rm", "-f", containerID)
			_ = removeCmd.Run() // Ignore errors

			// Wait a moment for Docker to process the removal
			time.Sleep(200 * time.Millisecond)

			// Double-check removal
			checkCmd := exec.Command("docker", "ps", "-a", "-q", "--filter", fmt.Sprintf("id=%s", containerID))
			output, _ := checkCmd.Output()
			if len(output) > 0 {
				// If container still exists, try again
				retryCmd := exec.Command("docker", "rm", "-f", containerID)
				_ = retryCmd.Run()
			}
		}

		// Clean up temp directory
		os.RemoveAll(tempDir)
	}

	// Get the registry to track containers - retry a few times if it fails
	var registry container.Registry
	var registryErr error
	for attempts := 0; attempts < 3; attempts++ {
		registry, registryErr = container.NewDockerRegistry()
		if registryErr == nil {
			break
		}
		time.Sleep(time.Second)
	}

	if registryErr != nil {
		t.Logf("Warning: Failed to get Docker registry for cleanup tracking: %v", registryErr)
	}

	// Create Docker adapter
	dockerAdapter, err := container.NewDockerAdapter(config)
	if err != nil {
		os.RemoveAll(tempDir)
		return nil, nil, fmt.Errorf("failed to create docker adapter: %w", err)
	}

	// Register a finalizer for this test to ensure container cleanup
	runtime.SetFinalizer(t, func(t *testing.T) {
		containerCleanupFunc(dockerAdapter.GetContainerID())
	})

	// Create cleanup function that ensures container is removed
	cleanup := func() {
		t.Logf("Cleaning up Docker container: %s", containerName)

		// Get container ID before adapter cleanup
		containerID := dockerAdapter.GetContainerID()

		// Use our direct cleanup
		containerCleanupFunc(containerID)

		// Then use the adapter's cleanup
		dockerAdapter.Cleanup()
	}

	// Register container with registry if available
	if registry != nil && dockerAdapter.GetContainerID() != "" {
		ctx := context.Background()
		t.Logf("Registering container %s (%s) with registry for cleanup tracking",
			dockerAdapter.GetContainerID(), containerName)

		// Create a container config for registration
		containerConfig := container.ContainerConfig{
			Name:  containerName,
			Image: "ubuntu:latest",
			Env: map[string]string{
				"TURINGPI_TEST_ID": testID,
				"TURINGPI_TEST":    "true",
			},
		}

		// Since the container is already created, we just need to register it
		_, regErr := registry.RegisterExistingContainer(ctx, dockerAdapter.GetContainerID(), containerConfig)
		if regErr != nil {
			t.Logf("Warning: Failed to register container with registry: %v", regErr)
		}
	}

	// Create our adapter to make it implement CommandExecutor
	executor := &DockerExecutorAdapter{
		adapter: dockerAdapter,
	}

	// Install necessary packages in container
	ctx := context.Background()
	_, err = executor.Execute(ctx, "apt-get", "update")
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to update apt: %w", err)
	}

	// Install required packages - findmnt is part of util-linux
	_, err = executor.Execute(ctx, "apt-get", "install", "-y", "util-linux", "rsync", "fdisk", "cloud-guest-utils")
	if err != nil {
		cleanup()
		return nil, nil, fmt.Errorf("failed to install packages: %w", err)
	}

	return executor, cleanup, nil
}

// TestIntegrationFilesystem tests FilesystemOperations with native Linux or a container
func TestIntegrationFilesystem(t *testing.T) {
	// Setup executor based on platform
	executor, cleanup, err := setupExecutor(t)
	if err != nil {
		t.Fatalf("Failed to setup executor: %v", err)
	}
	defer cleanup()

	// Create filesystem operations
	fs := NewFilesystemOperations(executor)
	ctx := context.Background()

	// Test MakeDirectory
	t.Run("MakeDirectory", func(t *testing.T) {
		err := fs.MakeDirectory("/tmp", "test-dir", 0755)
		if err != nil {
			t.Fatalf("MakeDirectory failed: %v", err)
		}

		// Verify directory exists
		if !fs.FileExists("/tmp", "test-dir") {
			t.Errorf("Directory was not created")
		}

		// Verify it's a directory
		if !fs.IsDirectory("/tmp", "test-dir") {
			t.Errorf("Created path is not a directory")
		}
	})

	// Test WriteFile
	t.Run("WriteFile", func(t *testing.T) {
		content := []byte("Hello, world!")
		err := fs.WriteFile("/tmp", "test-dir/test.txt", content, 0644)
		if err != nil {
			t.Fatalf("WriteFile failed: %v", err)
		}

		// Verify file exists
		if !fs.FileExists("/tmp", "test-dir/test.txt") {
			t.Errorf("File was not created")
		}

		// Read the file and verify content
		readContent, err := fs.ReadFile("/tmp", "test-dir/test.txt")
		if err != nil {
			t.Fatalf("ReadFile failed: %v", err)
		}

		if string(readContent) != string(content) {
			t.Errorf("File content doesn't match, got: %s, want: %s", readContent, content)
		}
	})

	// Test ChangePermissions
	t.Run("ChangePermissions", func(t *testing.T) {
		err := fs.ChangePermissions("/tmp", "test-dir/test.txt", 0600)
		if err != nil {
			t.Fatalf("ChangePermissions failed: %v", err)
		}

		// Verify permissions (indirect check via ls command)
		output, err := executor.Execute(ctx, "ls", "-l", "/tmp/test-dir/test.txt")
		if err != nil {
			t.Fatalf("Failed to check permissions: %v", err)
		}

		// Output should start with -rw------- (permissions 600)
		permissions := string(output)[0:10]
		if permissions != "-rw-------" {
			t.Errorf("Permissions not set correctly, got: %s, want: -rw-------", permissions)
		}
	})

	// Test CopyDirectory
	t.Run("CopyDirectory", func(t *testing.T) {
		// Create source directory with some files
		err := fs.MakeDirectory("/tmp", "source-dir", 0755)
		if err != nil {
			t.Fatalf("Failed to create source directory: %v", err)
		}

		err = fs.WriteFile("/tmp", "source-dir/file1.txt", []byte("File 1"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err = fs.WriteFile("/tmp", "source-dir/file2.txt", []byte("File 2"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		// Create a subdirectory
		err = fs.MakeDirectory("/tmp", "source-dir/subdir", 0755)
		if err != nil {
			t.Fatalf("Failed to create subdirectory: %v", err)
		}

		err = fs.WriteFile("/tmp", "source-dir/subdir/file3.txt", []byte("File 3"), 0644)
		if err != nil {
			t.Fatalf("Failed to create test file in subdirectory: %v", err)
		}

		// Copy the directory
		err = fs.CopyDirectory(ctx, "/tmp/source-dir", "/tmp/dest-dir")
		if err != nil {
			t.Fatalf("CopyDirectory failed: %v", err)
		}

		// Verify destination directory exists
		if !fs.FileExists("/tmp", "dest-dir") {
			t.Errorf("Destination directory was not created")
		}

		// Verify files were copied
		if !fs.FileExists("/tmp", "dest-dir/file1.txt") {
			t.Errorf("File1 was not copied")
		}

		if !fs.FileExists("/tmp", "dest-dir/file2.txt") {
			t.Errorf("File2 was not copied")
		}

		// Verify subdirectory and its contents were copied
		if !fs.IsDirectory("/tmp", "dest-dir/subdir") {
			t.Errorf("Subdirectory was not copied")
		}

		if !fs.FileExists("/tmp", "dest-dir/subdir/file3.txt") {
			t.Errorf("File in subdirectory was not copied")
		}

		// Verify file content
		content1, err := fs.ReadFile("/tmp", "dest-dir/file1.txt")
		if err != nil {
			t.Fatalf("Failed to read copied file: %v", err)
		}
		if string(content1) != "File 1" {
			t.Errorf("Copied file content doesn't match, got: %s, want: File 1", content1)
		}
	})
}

// TestIntegrationNetwork tests NetworkOperations with native Linux or a container
func TestIntegrationNetwork(t *testing.T) {
	// Setup executor based on platform
	executor, cleanup, err := setupExecutor(t)
	if err != nil {
		t.Fatalf("Failed to setup executor: %v", err)
	}
	defer cleanup()

	// Create network operations
	network := NewNetworkOperations(executor)
	ctx := context.Background()

	// Create a temporary "root" directory to simulate a mounted system
	mountDir := "/tmp/netconfig-test"
	_, err = executor.Execute(ctx, "mkdir", "-p", mountDir)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Test ApplyNetworkConfig with traditional interfaces
	t.Run("ApplyNetworkConfig_Interfaces", func(t *testing.T) {
		// Apply network configuration
		err := network.ApplyNetworkConfig(
			ctx,
			mountDir,
			"testhost",
			"192.168.1.100/24",
			"192.168.1.1",
			[]string{"8.8.8.8", "8.8.4.4"},
		)
		if err != nil {
			t.Fatalf("ApplyNetworkConfig failed: %v", err)
		}

		// Verify hostname file
		hostname, err := network.fs.ReadFile(mountDir, "etc/hostname")
		if err != nil {
			t.Fatalf("Failed to read hostname file: %v", err)
		}
		if string(hostname) != "testhost\n" {
			t.Errorf("Hostname doesn't match, got: %s, want: testhost\\n", hostname)
		}

		// Verify hosts file
		hosts, err := network.fs.ReadFile(mountDir, "etc/hosts")
		if err != nil {
			t.Fatalf("Failed to read hosts file: %v", err)
		}
		expectedHosts := "127.0.0.1\tlocalhost\n127.0.1.1\ttesthost\n\n"
		if string(hosts) != expectedHosts {
			t.Errorf("Hosts file doesn't match, got: %s, want: %s", hosts, expectedHosts)
		}

		// Verify interfaces file
		if !network.fs.FileExists(mountDir, "etc/network/interfaces") {
			t.Errorf("Interfaces file was not created")
		}

		interfaces, err := network.fs.ReadFile(mountDir, "etc/network/interfaces")
		if err != nil {
			t.Fatalf("Failed to read interfaces file: %v", err)
		}

		// Basic checks - not checking the entire content
		if len(interfaces) == 0 {
			t.Errorf("Interfaces file is empty")
		}

		// Check for some expected content
		interfacesStr := string(interfaces)
		if !strings.Contains(interfacesStr, "address 192.168.1.100") {
			t.Errorf("Interfaces file doesn't contain expected IP address")
		}
		if !strings.Contains(interfacesStr, "netmask 255.255.255.0") {
			t.Errorf("Interfaces file doesn't contain expected netmask")
		}
		if !strings.Contains(interfacesStr, "gateway 192.168.1.1") {
			t.Errorf("Interfaces file doesn't contain expected gateway")
		}
		if !strings.Contains(interfacesStr, "dns-nameservers 8.8.8.8 8.8.4.4") {
			t.Errorf("Interfaces file doesn't contain expected DNS servers")
		}
	})

	// Test ApplyNetworkConfig with netplan
	t.Run("ApplyNetworkConfig_Netplan", func(t *testing.T) {
		// Create netplan directory to trigger netplan configuration
		err := network.fs.MakeDirectory(mountDir, "etc/netplan", 0755)
		if err != nil {
			t.Fatalf("Failed to create netplan directory: %v", err)
		}

		// Apply network configuration
		err = network.ApplyNetworkConfig(
			ctx,
			mountDir,
			"netplanhost",
			"10.0.0.10/24",
			"10.0.0.1",
			[]string{"1.1.1.1", "1.0.0.1"},
		)
		if err != nil {
			t.Fatalf("ApplyNetworkConfig failed: %v", err)
		}

		// Verify netplan config file
		if !network.fs.FileExists(mountDir, "etc/netplan/01-netcfg.yaml") {
			t.Errorf("Netplan config file was not created")
		}

		netplan, err := network.fs.ReadFile(mountDir, "etc/netplan/01-netcfg.yaml")
		if err != nil {
			t.Fatalf("Failed to read netplan file: %v", err)
		}

		// Basic checks - not checking the entire content
		if len(netplan) == 0 {
			t.Errorf("Netplan file is empty")
		}

		// Check for some expected content
		netplanStr := string(netplan)
		if !strings.Contains(netplanStr, "addresses: [10.0.0.10/24]") {
			t.Errorf("Netplan file doesn't contain expected IP CIDR")
		}
		if !strings.Contains(netplanStr, "gateway4: 10.0.0.1") {
			t.Errorf("Netplan file doesn't contain expected gateway")
		}
		if !strings.Contains(netplanStr, "addresses: [1.1.1.1, 1.0.0.1]") {
			t.Errorf("Netplan file doesn't contain expected DNS servers")
		}
	})
}

// TestIntegrationImage tests ImageOperations with native Linux or a container
func TestIntegrationImage(t *testing.T) {
	// Setup executor based on platform
	executor, cleanup, err := setupExecutor(t)
	if err != nil {
		t.Fatalf("Failed to setup executor: %v", err)
	}
	defer cleanup()

	// Create image operations
	imgOps := NewImageOperations(executor)
	ctx := context.Background()

	// Test ValidateImage
	t.Run("ValidateImage", func(t *testing.T) {
		// Create a small test disk image
		_, err := executor.Execute(ctx, "dd", "if=/dev/zero", "of=/tmp/test.img", "bs=1M", "count=10")
		if err != nil {
			t.Fatalf("Failed to create test image: %v", err)
		}

		// Create a partition table
		_, err = executor.Execute(ctx, "bash", "-c", "echo -e 'o\\nn\\np\\n1\\n\\n\\nw' | fdisk /tmp/test.img")
		if err != nil {
			t.Fatalf("Failed to create partition table: %v", err)
		}

		// Test with valid image
		err = imgOps.ValidateImage(ctx, "/tmp/test.img")
		if err != nil {
			t.Errorf("ValidateImage failed with valid image: %v", err)
		}

		// Test with non-existent image
		err = imgOps.ValidateImage(ctx, "/tmp/nonexistent.img")
		if err == nil {
			t.Errorf("ValidateImage should fail with non-existent image")
		}
	})

	// Test ExtractBootFiles
	t.Run("ExtractBootFiles", func(t *testing.T) {
		// Create a mock boot directory with kernel and initrd files
		bootDir := "/tmp/boot"
		_, err := executor.Execute(ctx, "mkdir", "-p", bootDir)
		if err != nil {
			t.Fatalf("Failed to create boot directory: %v", err)
		}

		// Create mock kernel and initrd files
		_, err = executor.Execute(ctx, "bash", "-c", "echo 'kernel content' > /tmp/boot/vmlinuz-5.10.0")
		if err != nil {
			t.Fatalf("Failed to create kernel file: %v", err)
		}

		_, err = executor.Execute(ctx, "bash", "-c", "echo 'initrd content' > /tmp/boot/initrd.img-5.10.0")
		if err != nil {
			t.Fatalf("Failed to create initrd file: %v", err)
		}

		// Create output directory
		outputDir := "/tmp/output"
		_, err = executor.Execute(ctx, "mkdir", "-p", outputDir)
		if err != nil {
			t.Fatalf("Failed to create output directory: %v", err)
		}

		// Extract boot files
		kernel, initrd, err := imgOps.ExtractBootFiles(ctx, bootDir, outputDir)
		if err != nil {
			t.Fatalf("ExtractBootFiles failed: %v", err)
		}

		// Verify files were extracted
		expectedKernel := filepath.Join(outputDir, "vmlinuz-5.10.0")
		expectedInitrd := filepath.Join(outputDir, "initrd.img-5.10.0")

		if kernel != expectedKernel {
			t.Errorf("Expected kernel path %s, got %s", expectedKernel, kernel)
		}

		if initrd != expectedInitrd {
			t.Errorf("Expected initrd path %s, got %s", expectedInitrd, initrd)
		}

		// Verify file existence
		exists, err := executor.Execute(ctx, "test", "-f", kernel)
		if err != nil {
			t.Errorf("Kernel file does not exist: %v", err)
		}
		if len(exists) > 0 {
			t.Errorf("Unexpected output from test command: %s", exists)
		}

		exists, err = executor.Execute(ctx, "test", "-f", initrd)
		if err != nil {
			t.Errorf("Initrd file does not exist: %v", err)
		}
		if len(exists) > 0 {
			t.Errorf("Unexpected output from test command: %s", exists)
		}
	})

	// Test ApplyDTBOverlay
	t.Run("ApplyDTBOverlay", func(t *testing.T) {
		// Create a mock boot directory with overlays directory
		bootDir := "/tmp/boot-dtb"
		_, err := executor.Execute(ctx, "mkdir", "-p", bootDir+"/overlays")
		if err != nil {
			t.Fatalf("Failed to create boot directory with overlays: %v", err)
		}

		// Create a mock config.txt
		_, err = executor.Execute(ctx, "bash", "-c", fmt.Sprintf("echo 'initial config' > %s/config.txt", bootDir))
		if err != nil {
			t.Fatalf("Failed to create config.txt: %v", err)
		}

		// Create a mock dtb overlay file
		dtbOverlayPath := "/tmp/test-overlay.dtbo"
		_, err = executor.Execute(ctx, "bash", "-c", fmt.Sprintf("echo 'dtb overlay content' > %s", dtbOverlayPath))
		if err != nil {
			t.Fatalf("Failed to create dtb overlay file: %v", err)
		}

		// Apply DTB overlay
		err = imgOps.ApplyDTBOverlay(ctx, bootDir, dtbOverlayPath)
		if err != nil {
			t.Fatalf("ApplyDTBOverlay failed: %v", err)
		}

		// Verify overlay was copied
		overlayDest := filepath.Join(bootDir, "overlays", filepath.Base(dtbOverlayPath))
		exists, err := executor.Execute(ctx, "test", "-f", overlayDest)
		if err != nil {
			t.Errorf("Overlay file was not copied: %v", err)
		}
		if len(exists) > 0 {
			t.Errorf("Unexpected output from test command: %s", exists)
		}

		// Verify config.txt was updated
		configContent, err := executor.Execute(ctx, "cat", fmt.Sprintf("%s/config.txt", bootDir))
		if err != nil {
			t.Fatalf("Failed to read config.txt: %v", err)
		}

		overlayName := strings.TrimSuffix(filepath.Base(dtbOverlayPath), ".dtbo")
		overlayLine := fmt.Sprintf("dtoverlay=%s", overlayName)

		if !strings.Contains(string(configContent), overlayLine) {
			t.Errorf("Config.txt doesn't contain overlay line: %s", overlayLine)
		}
	})
}
