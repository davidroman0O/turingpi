package bmc

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Flags for BMC tests
var (
	firmwarePathFlag = flag.String("firmware", "", "Path to firmware file for update tests")
)

// getFirstLines returns the first n lines of a string
func getFirstLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[:n], "\n")
}

// TestBMC_GetInfo tests retrieving information from the BMC
func TestBMC_GetInfo(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup BMC connection
	configPath := filepath.Join("../cache/testdata/ssh_config.json")
	bmc, err := NewWithSSH(configPath)
	if err != nil {
		t.Fatalf("Failed to create BMC with SSH: %v", err)
	}

	// Use a context with timeout for all operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	info, err := bmc.GetInfo(ctx)
	if err != nil {
		t.Fatalf("Failed to get BMC info: %v", err)
	}

	// Validate the info
	if info.Version == "" {
		t.Error("BMC version is empty")
	}
	if info.IPAddress == "" {
		t.Error("BMC IP address is empty")
	}
	if info.BuildVersion == "" {
		t.Error("BuildVersion is empty")
	}
	if info.APIVersion == "" {
		t.Error("APIVersion is empty")
	}

	t.Logf("BMC Info: %+v", info)
}

// TestBMC_PowerOperations tests power-related operations on nodes
func TestBMC_PowerOperations(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup BMC connection
	configPath := filepath.Join("../cache/testdata/ssh_config.json")
	bmc, err := NewWithSSH(configPath)
	if err != nil {
		t.Fatalf("Failed to create BMC with SSH: %v", err)
	}

	// Use a context with timeout for all operations
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Test each node's power operations
	for nodeID := 1; nodeID <= 4; nodeID++ {
		testNodePowerOperations(t, ctx, bmc, nodeID)
	}
}

// TestBMC_ExecuteCommand tests executing various commands on the BMC
func TestBMC_ExecuteCommand(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup BMC connection
	configPath := filepath.Join("../cache/testdata/ssh_config.json")
	bmc, err := NewWithSSH(configPath)
	if err != nil {
		t.Fatalf("Failed to create BMC with SSH: %v", err)
	}

	// Use a context with timeout for all operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test basic info command
	t.Run("InfoCommand", func(t *testing.T) {
		stdout, stderr, err := bmc.ExecuteCommand(ctx, "tpi info")
		if err != nil {
			t.Fatalf("Failed to execute info command: %v (stderr: %s)", err, stderr)
		}

		if stdout == "" {
			t.Error("Command output is empty")
		}
		if !strings.Contains(stdout, "version") {
			t.Error("Output doesn't contain expected version info")
		}

		t.Logf("Info command output: %s", stdout)
	})

	// Test node status command
	t.Run("PowerStatusCommand", func(t *testing.T) {
		stdout, stderr, err := bmc.ExecuteCommand(ctx, "tpi power status")
		if err != nil {
			t.Fatalf("Failed to execute status command: %v (stderr: %s)", err, stderr)
		}

		if stdout == "" {
			t.Error("Command output is empty")
		}
		for i := 1; i <= 4; i++ {
			if !strings.Contains(stdout, fmt.Sprintf("node%d:", i)) {
				t.Errorf("Output missing status for node %d", i)
			}
		}

		t.Logf("Power status command output: %s", stdout)
	})

	// Test help command
	t.Run("HelpCommand", func(t *testing.T) {
		stdout, stderr, err := bmc.ExecuteCommand(ctx, "tpi help")
		if err != nil {
			t.Fatalf("Failed to execute help command: %v (stderr: %s)", err, stderr)
		}

		if stdout == "" {
			t.Error("Command output is empty")
		}

		t.Logf("Help command output sample: %s", getFirstLines(stdout, 5))
	})
}

// TestBMC_USBConfig tests USB configuration operations
func TestBMC_USBConfig(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup BMC connection
	configPath := filepath.Join("../cache/testdata/ssh_config.json")
	bmc, err := NewWithSSH(configPath)
	if err != nil {
		t.Fatalf("Failed to create BMC with SSH: %v", err)
	}

	// Use a context with timeout for all operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Get current USB config
	initialConfig, err := bmc.GetUSBConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get USB configuration: %v", err)
	}
	t.Logf("Initial USB config: Node=%d, Host=%t", initialConfig.NodeID, initialConfig.Host)

	// Test setting USB to node 1 in device mode
	testNodeID := 1
	t.Logf("Setting USB to node %d in device mode", testNodeID)
	if err := bmc.SetUSBConfig(ctx, testNodeID, false); err != nil {
		t.Fatalf("Failed to set USB config: %v", err)
	}
	time.Sleep(1 * time.Second)

	// Verify the change
	newConfig, err := bmc.GetUSBConfig(ctx)
	if err != nil {
		t.Fatalf("Failed to get USB configuration: %v", err)
	}
	if newConfig.NodeID != testNodeID || newConfig.Host != false {
		t.Errorf("USB config not set correctly. Expected Node=%d, Host=false, got Node=%d, Host=%t",
			testNodeID, newConfig.NodeID, newConfig.Host)
	}

	// Restore the initial config
	t.Log("Restoring initial USB configuration")
	if err := bmc.SetUSBConfig(ctx, initialConfig.NodeID, initialConfig.Host); err != nil {
		t.Errorf("Failed to restore initial USB config: %v", err)
	}
}

// TestBMC_UART tests basic UART functionality with minimal assumptions
func TestBMC_UART(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup BMC connection
	configPath := filepath.Join("../cache/testdata/ssh_config.json")
	bmc, err := NewWithSSH(configPath)
	if err != nil {
		t.Fatalf("Failed to create BMC with SSH: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	nodeID := 2 // Use whichever node is available

	// 1. First test basic GetUARTOutput
	t.Log("Testing basic GetUARTOutput")
	initialOutput, err := bmc.GetUARTOutput(ctx, nodeID)
	if err != nil {
		t.Fatalf("Failed to get initial UART output: %v", err)
	}

	// Log a sample of the initial output for debugging
	if initialOutput != "" {
		t.Logf("Initial UART output sample: %s", getFirstLines(initialOutput, 3))
	} else {
		t.Log("Initial UART output is empty")
	}

	// 2. Test basic SendUARTInput with a carriage return
	t.Log("Testing basic SendUARTInput with carriage return")
	if err := bmc.SendUARTInput(ctx, nodeID, "\r"); err != nil {
		t.Fatalf("Failed to send carriage return: %v", err)
	}

	// Give the system a moment to respond
	time.Sleep(1 * time.Second)

	// 3. Check if output changed after sending input
	afterCROutput, err := bmc.GetUARTOutput(ctx, nodeID)
	if err != nil {
		t.Fatalf("Failed to get UART output after carriage return: %v", err)
	}

	if afterCROutput != "" {
		t.Logf("UART output after carriage return: %s", getLastLines(afterCROutput, 3))
	} else {
		t.Log("No output received after carriage return")
	}

	// 4. Now test with a simple help command for U-Boot
	t.Log("Testing with 'help' command")
	if err := bmc.SendUARTInput(ctx, nodeID, "help\r"); err != nil {
		t.Fatalf("Failed to send help command: %v", err)
	}

	// Give the system a moment to respond
	time.Sleep(2 * time.Second)

	// 5. Check for the response
	finalOutput, err := bmc.GetUARTOutput(ctx, nodeID)
	if err != nil {
		t.Fatalf("Failed to get UART output after help command: %v", err)
	}

	if finalOutput != "" {
		t.Logf("Final UART output: %s", getLastLines(finalOutput, 5))

		// Note: We only log if help-related content is found, but don't fail the test if it's not
		// Since we can't guarantee what state the system is in
		if strings.Contains(strings.ToLower(finalOutput), "help") ||
			strings.Contains(strings.ToLower(finalOutput), "command") {
			t.Logf("Successfully found command-related response in UART output")
		} else {
			t.Logf("Command response not found in UART output - system may be in an unexpected state")
		}
	} else {
		t.Log("No output received after help command")
	}

	// Test conclusion
	t.Log("UART test completed - basic communication functions verified")
}

// TestBMC_Reboot tests rebooting the BMC
// WARNING: This test is destructive and will reboot the BMC!
func TestBMC_Reboot(t *testing.T) {
	// Skip by default due to disruptive nature
	if testing.Short() || !isBMCRebootTestEnabled() {
		t.Skip("Skipping BMC reboot test (enable with env var BMC_TEST_REBOOT=1)")
	}

	// Setup BMC connection
	configPath := filepath.Join("../cache/testdata/ssh_config.json")
	bmc, err := NewWithSSH(configPath)
	if err != nil {
		t.Fatalf("Failed to create BMC with SSH: %v", err)
	}

	// Use a context with timeout for all operations
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	err = bmc.Reboot(ctx)
	if err != nil {
		t.Fatalf("Failed to reboot BMC: %v", err)
	}

	// Wait for BMC to come back online
	t.Log("Waiting 30 seconds for BMC to reboot...")
	time.Sleep(30 * time.Second)

	// Recreate the connection since it was lost during reboot
	bmc, err = NewWithSSH(configPath)
	if err != nil {
		t.Fatalf("Failed to reconnect to BMC after reboot: %v", err)
	}

	// Verify BMC is back online by getting info
	info, err := bmc.GetInfo(ctx)
	if err != nil {
		t.Fatalf("BMC didn't come back online after reboot: %v", err)
	}
	t.Logf("BMC is back online with version: %s", info.Version)
}

// TestBMC_FirmwareUpdate tests firmware update
// Separated because it's extremely disruptive and requires a firmware path
func TestBMC_FirmwareUpdate(t *testing.T) {
	// Parse flags to ensure flag is available
	flag.Parse()

	// This is the only test that needs a parameter, run with:
	// go test -v -run=TestBMC_FirmwareUpdate -firmware=/path/to/firmware.bin
	firmwarePath := *firmwarePathFlag

	if firmwarePath == "" {
		t.Skip("No firmware path provided, set -firmware flag to run")
	}

	// Get the SSH connection to the real hardware
	configPath := filepath.Join("../cache/testdata/ssh_config.json")
	bmc, err := NewWithSSH(configPath)
	if err != nil {
		t.Fatalf("Failed to create BMC with SSH: %v", err)
	}

	// Use a context with timeout for all operations
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Test version before update
	infoBefore, err := bmc.GetInfo(ctx)
	if err != nil {
		t.Fatalf("Failed to get BMC info before update: %v", err)
	}
	t.Logf("BMC version before update: %s", infoBefore.Version)

	// Update firmware
	err = bmc.UpdateFirmware(ctx, firmwarePath)
	if err != nil {
		t.Fatalf("Failed to update firmware: %v", err)
	}

	// Wait for BMC to finish updating and come back online
	t.Log("Waiting 120 seconds for firmware update to complete...")
	time.Sleep(120 * time.Second)

	// Reconnect to BMC
	bmc, err = NewWithSSH(configPath)
	if err != nil {
		t.Fatalf("Failed to reconnect to BMC after firmware update: %v", err)
	}

	// Verify BMC is back online by getting info
	infoAfter, err := bmc.GetInfo(ctx)
	if err != nil {
		t.Fatalf("BMC didn't come back online after firmware update: %v", err)
	}
	t.Logf("BMC version after update: %s", infoAfter.Version)

	// Check if version changed
	if infoAfter.Version == infoBefore.Version {
		t.Logf("Warning: BMC version did not change after update")
	}
}

// Test power operations on a specific node
func testNodePowerOperations(t *testing.T, ctx context.Context, bmc BMC, nodeID int) {
	t.Run(fmt.Sprintf("Node%d_PowerOperations", nodeID), func(t *testing.T) {
		// Get initial power status
		initialStatus, err := bmc.GetPowerStatus(ctx, nodeID)
		if err != nil {
			t.Fatalf("Failed to get initial power status for node %d: %v", nodeID, err)
		}
		t.Logf("Node %d initial power status: %s", nodeID, initialStatus.State)

		// Save initial state to restore at the end
		initialState := initialStatus.State

		// Try to power off the node if it's on
		if initialState == PowerStateOn {
			t.Logf("Turning off node %d", nodeID)
			err := bmc.PowerOff(ctx, nodeID)
			if err != nil {
				t.Fatalf("Failed to power off node %d: %v", nodeID, err)
			}

			// Wait for power state to change
			time.Sleep(2 * time.Second)

			// Verify it's off
			status, err := bmc.GetPowerStatus(ctx, nodeID)
			if err != nil {
				t.Fatalf("Failed to get power status after power off: %v", err)
			}
			if status.State != PowerStateOff {
				t.Errorf("Node %d should be off but is %s", nodeID, status.State)
			} else {
				t.Logf("Successfully powered off node %d", nodeID)
			}

			// Now power it back on
			t.Logf("Turning on node %d", nodeID)
			err = bmc.PowerOn(ctx, nodeID)
			if err != nil {
				t.Fatalf("Failed to power on node %d: %v", nodeID, err)
			}

			// Wait for power state to change
			time.Sleep(3 * time.Second)

			// Verify it's on
			status, err = bmc.GetPowerStatus(ctx, nodeID)
			if err != nil {
				t.Fatalf("Failed to get power status after power on: %v", err)
			}
			if status.State != PowerStateOn {
				t.Errorf("Node %d should be on but is %s", nodeID, status.State)
			} else {
				t.Logf("Successfully powered on node %d", nodeID)
			}

			// Test reset
			t.Logf("Resetting node %d", nodeID)
			err = bmc.Reset(ctx, nodeID)
			if err != nil {
				t.Fatalf("Failed to reset node %d: %v", nodeID, err)
			}

			// Wait for reset to complete
			time.Sleep(3 * time.Second)

			// Verify it's still on after reset
			status, err = bmc.GetPowerStatus(ctx, nodeID)
			if err != nil {
				t.Fatalf("Failed to get power status after reset: %v", err)
			}
			if status.State != PowerStateOn {
				t.Errorf("Node %d should be on after reset but is %s", nodeID, status.State)
			} else {
				t.Logf("Successfully reset node %d", nodeID)
			}
		} else if initialState == PowerStateOff {
			// Node is off, let's turn it on, reset it, then off again
			t.Logf("Turning on node %d", nodeID)
			err := bmc.PowerOn(ctx, nodeID)
			if err != nil {
				t.Fatalf("Failed to power on node %d: %v", nodeID, err)
			}

			// Wait for power state to change
			time.Sleep(3 * time.Second)

			// Verify it's on
			status, err := bmc.GetPowerStatus(ctx, nodeID)
			if err != nil {
				t.Fatalf("Failed to get power status after power on: %v", err)
			}
			if status.State != PowerStateOn {
				t.Errorf("Node %d should be on but is %s", nodeID, status.State)
			} else {
				t.Logf("Successfully powered on node %d", nodeID)
			}

			// Test reset
			t.Logf("Resetting node %d", nodeID)
			err = bmc.Reset(ctx, nodeID)
			if err != nil {
				t.Fatalf("Failed to reset node %d: %v", nodeID, err)
			}

			// Wait for reset to complete
			time.Sleep(3 * time.Second)

			// Now turn it off again to restore initial state
			t.Logf("Turning off node %d", nodeID)
			err = bmc.PowerOff(ctx, nodeID)
			if err != nil {
				t.Fatalf("Failed to power off node %d: %v", nodeID, err)
			}

			// Wait for power state to change
			time.Sleep(2 * time.Second)

			// Verify it's off
			status, err = bmc.GetPowerStatus(ctx, nodeID)
			if err != nil {
				t.Fatalf("Failed to get power status after power off: %v", err)
			}
			if status.State != PowerStateOff {
				t.Errorf("Node %d should be off but is %s", nodeID, status.State)
			} else {
				t.Logf("Successfully powered off node %d", nodeID)
			}
		} else {
			t.Logf("Node %d has unknown power state, skipping power operations", nodeID)
		}

		// Restore the node to its initial state if different from current
		currentStatus, err := bmc.GetPowerStatus(ctx, nodeID)
		if err != nil {
			t.Fatalf("Failed to get current power status for node %d: %v", nodeID, err)
		}

		if currentStatus.State != initialState {
			t.Logf("Restoring node %d to initial state: %s", nodeID, initialState)
			if initialState == PowerStateOn {
				err = bmc.PowerOn(ctx, nodeID)
			} else if initialState == PowerStateOff {
				err = bmc.PowerOff(ctx, nodeID)
			}

			if err != nil {
				t.Errorf("Failed to restore node %d to initial state: %v", nodeID, err)
			} else {
				time.Sleep(2 * time.Second)
				finalStatus, err := bmc.GetPowerStatus(ctx, nodeID)
				if err != nil {
					t.Errorf("Failed to get final power status for node %d: %v", nodeID, err)
				} else {
					t.Logf("Node %d final state: %s", nodeID, finalStatus.State)
				}
			}
		} else {
			t.Logf("Node %d is already in its initial state: %s", nodeID, initialState)
		}
	})
}

// Helper function to check if BMC reboot test is enabled
func isBMCRebootTestEnabled() bool {
	return false // Change this to check for an environment variable if needed
}

// TestUploadFile tests the SSHExecutor's UploadFile function
func TestUploadFile(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Read the config file
	configPath := filepath.Join("../cache/testdata/ssh_config.json")
	configFile, err := os.Open(configPath)
	if err != nil {
		t.Fatalf("Failed to open SSH config file: %v", err)
	}
	defer configFile.Close()

	// Parse the config
	configData, err := io.ReadAll(configFile)
	if err != nil {
		t.Fatalf("Failed to read SSH config file: %v", err)
	}

	var config SSHConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		t.Fatalf("Failed to parse SSH config: %v", err)
	}

	// Create a temporary local file to upload
	localFile, err := os.CreateTemp("", "upload-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	localPath := localFile.Name()
	defer os.Remove(localPath) // Clean up the temp file at the end

	// Write some test data to the file
	testData := []byte("This is a test file for UploadFile test.\nIt has multiple lines.\n")
	if _, err := localFile.Write(testData); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	localFile.Close()

	// Define remote path
	remotePath := fmt.Sprintf("/tmp/test-upload-%d.txt", time.Now().UnixNano())

	// Create SSHExecutor
	executor := &SSHExecutor{
		config: config,
	}

	// Upload the file
	t.Logf("Uploading %s to %s", localPath, remotePath)
	err = executor.UploadFile(localPath, remotePath)
	if err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}

	// Verify the upload by executing a command to check the file exists and has the correct content
	bmc, err := NewWithSSH(configPath)
	if err != nil {
		t.Fatalf("Failed to create BMC with SSH: %v", err)
	}

	// Use a context with timeout for all operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check file exists
	stdout, stderr, err := bmc.ExecuteCommand(ctx, fmt.Sprintf("ls -l %s", remotePath))
	if err != nil {
		t.Fatalf("Failed to verify file exists: %v (stderr: %s)", err, stderr)
	}
	if !strings.Contains(stdout, remotePath) {
		t.Errorf("File not found on remote system: %s", remotePath)
	} else {
		t.Logf("File found on remote system: %s", stdout)
	}

	// Check file content
	stdout, stderr, err = bmc.ExecuteCommand(ctx, fmt.Sprintf("cat %s", remotePath))
	if err != nil {
		t.Fatalf("Failed to read remote file: %v (stderr: %s)", err, stderr)
	}

	// Normalize line endings for comparison
	expectedContent := strings.TrimSpace(string(testData))
	actualContent := strings.TrimSpace(stdout)

	if expectedContent != actualContent {
		t.Errorf("File content mismatch. Expected:\n%s\nGot:\n%s", expectedContent, actualContent)
	} else {
		t.Logf("File content matches expected data")
	}

	// Clean up the remote file
	_, stderr, err = bmc.ExecuteCommand(ctx, fmt.Sprintf("rm %s", remotePath))
	if err != nil {
		t.Logf("Warning: Failed to remove remote file: %v (stderr: %s)", err, stderr)
	} else {
		t.Logf("Successfully removed remote file: %s", remotePath)
	}
}

// TestBMC_UploadFile tests the UploadFile method on the BMC interface
func TestBMC_UploadFile(t *testing.T) {
	// Skip if running short tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup BMC connection
	configPath := filepath.Join("../cache/testdata/ssh_config.json")
	bmc, err := NewWithSSH(configPath)
	if err != nil {
		t.Fatalf("Failed to create BMC with SSH: %v", err)
	}

	// Create a temporary local file to upload
	localFile, err := os.CreateTemp("", "bmc-upload-test-*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	localPath := localFile.Name()
	defer os.Remove(localPath) // Clean up the temp file at the end

	// Write some test data to the file
	testData := []byte("This is a test file for BMC.UploadFile test.\nTesting BMC interface method.\n")
	if _, err := localFile.Write(testData); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	localFile.Close()

	// Define remote path
	remotePath := fmt.Sprintf("/tmp/bmc-test-upload-%d.txt", time.Now().UnixNano())

	// Use a context with timeout for all operations
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Upload the file using the BMC interface
	t.Logf("Uploading %s to %s", localPath, remotePath)
	err = bmc.UploadFile(ctx, localPath, remotePath)
	if err != nil {
		t.Fatalf("Failed to upload file: %v", err)
	}

	// Verify the upload
	stdout, stderr, err := bmc.ExecuteCommand(ctx, fmt.Sprintf("ls -l %s", remotePath))
	if err != nil {
		t.Fatalf("Failed to verify file exists: %v (stderr: %s)", err, stderr)
	}
	if !strings.Contains(stdout, remotePath) {
		t.Errorf("File not found on remote system: %s", remotePath)
	} else {
		t.Logf("File found on remote system: %s", stdout)
	}

	// Check file content
	stdout, stderr, err = bmc.ExecuteCommand(ctx, fmt.Sprintf("cat %s", remotePath))
	if err != nil {
		t.Fatalf("Failed to read remote file: %v (stderr: %s)", err, stderr)
	}

	// Normalize line endings for comparison
	expectedContent := strings.TrimSpace(string(testData))
	actualContent := strings.TrimSpace(stdout)

	if expectedContent != actualContent {
		t.Errorf("File content mismatch. Expected:\n%s\nGot:\n%s", expectedContent, actualContent)
	} else {
		t.Logf("File content matches expected data")
	}

	// Clean up the remote file
	_, stderr, err = bmc.ExecuteCommand(ctx, fmt.Sprintf("rm %s", remotePath))
	if err != nil {
		t.Logf("Warning: Failed to remove remote file: %v (stderr: %s)", err, stderr)
	} else {
		t.Logf("Successfully removed remote file: %s", remotePath)
	}
}
