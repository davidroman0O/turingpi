/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/davidroman0O/turingpi/pkg/v2/cache"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/platform"
	"github.com/davidroman0O/turingpi/pkg/v2/state"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
	"github.com/davidroman0O/turingpi/pkg/v2/workflows/actions/ubuntu"
)

// CommandExecutorMock implements the bmc.CommandExecutor interface for testing
type CommandExecutorMock struct{}

func (c *CommandExecutorMock) ExecuteCommand(command string) (string, string, error) {
	log.Printf("[CMD] Executing: %s", command)
	// Mock responses for various commands
	if command == "tpi power status" {
		return "node1: on\nnode2: off\nnode3: off\nnode4: off", "", nil
	} else if command == "tpi info" {
		return "api: v2\nversion: 1.2.3\nbuild_version: alpha\nip: 192.168.1.1\nmac: 00:11:22:33:44:55", "", nil
	}
	return "Command executed successfully", "", nil
}

func main() {
	ctx := context.Background()

	log.Println("--- Starting TuringPi RK1 Ubuntu Deployment Playground ---")

	// Set up working directory
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

	// Check platform compatibility
	if !platform.IsLinux() {
		log.Println("Detected non-Linux platform. This is a simulation run.")
		if !platform.DockerAvailable() {
			log.Println("WARNING: Docker is not available. Some functionality will be simulated.")
			log.Println("Docker is required for image operations on non-Linux platforms.")
			log.Println("Please install Docker and ensure it is running for full functionality.")
		} else {
			log.Println("Docker is available and will be used for container operations.")
		}
	}

	// Set up directories
	cacheDir := filepath.Join(workDir, ".tpi_cache")
	tempDir := filepath.Join(workDir, "tmp")
	outputDir := filepath.Join(workDir, "output")

	// Create directories
	os.MkdirAll(cacheDir, 0755)
	os.MkdirAll(tempDir, 0755)
	os.MkdirAll(outputDir, 0755)

	// Initialize the state manager
	stateFile := filepath.Join(workDir, "turingpi_state.json")
	stateManager, err := state.NewFileStateManager(stateFile)
	if err != nil {
		log.Fatalf("Failed to initialize state manager: %v", err)
	}

	// Initialize tools with a mock BMC executor
	toolConfig := &tools.TuringPiToolConfig{
		BMCExecutor: &CommandExecutorMock{},
		CacheDir:    cacheDir,
		NodeConfigs: map[int]*tools.NodeConfig{
			1: {
				Host:     "192.168.1.101",
				User:     "ubuntu",
				Password: "turingpi123",
			},
		},
	}

	toolProvider, err := tools.NewTuringPiToolProvider(toolConfig)
	if err != nil {
		log.Fatalf("Failed to initialize tool provider: %v", err)
	}

	// 1. Demonstrate BMC operations
	demonstrateBMCOperations(ctx, toolProvider)

	// 2. Demonstrate cache operations
	demonstrateCacheOperations(ctx, toolProvider, cacheDir)

	// 3. Demonstrate container operations if Docker is available
	if platform.DockerAvailable() {
		demonstrateContainerOperations(ctx, toolProvider)
	}

	// 4. Demonstrate state management
	demonstrateStateManagement(stateManager)

	// 5. Set up Ubuntu deployment workflow
	setupUbuntuWorkflow(toolProvider, tempDir, outputDir, cacheDir)

	log.Println("--- TuringPi RK1 Ubuntu Deployment Playground Completed ---")
}

func demonstrateBMCOperations(ctx context.Context, provider *tools.TuringPiToolProvider) {
	log.Println("\n=== Demonstrating BMC Operations ===")

	bmcTool := provider.GetBMCTool()

	// Get BMC info
	info, err := bmcTool.GetInfo(ctx)
	if err != nil {
		log.Printf("Failed to get BMC info: %v", err)
	} else {
		log.Printf("BMC Info: API Version: %s, Version: %s", info.APIVersion, info.Version)
		log.Printf("IP Address: %s, MAC Address: %s", info.IPAddress, info.MACAddress)
	}

	// Get power status for node 1
	status, err := bmcTool.GetPowerStatus(ctx, 1)
	if err != nil {
		log.Printf("Failed to get power status: %v", err)
	} else {
		log.Printf("Node %d power status: %s", status.NodeID, status.State)
	}

	// Power operations (simulated)
	log.Println("Simulating power operations...")

	if err := bmcTool.PowerOff(ctx, 1); err != nil {
		log.Printf("Failed to power off node 1: %v", err)
	} else {
		log.Printf("Node 1 powered off successfully")
	}

	if err := bmcTool.PowerOn(ctx, 1); err != nil {
		log.Printf("Failed to power on node 1: %v", err)
	} else {
		log.Printf("Node 1 powered on successfully")
	}
}

func demonstrateCacheOperations(ctx context.Context, provider *tools.TuringPiToolProvider, cacheDir string) {
	log.Println("\n=== Demonstrating Cache Operations ===")

	cacheTool := provider.GetCacheTool()
	if cacheTool == nil {
		log.Println("Cache tool not available")
		return
	}

	// Create test data
	testContent := []byte("This is test content for cache operations")
	testFile := filepath.Join(cacheDir, "test_file.txt")
	if err := os.WriteFile(testFile, testContent, 0644); err != nil {
		log.Printf("Failed to create test file: %v", err)
		return
	}

	// Create a cache key with tags
	cacheKey := "test-cache-item"
	tags := map[string]string{
		"type":    "test",
		"created": time.Now().Format(time.RFC3339),
	}

	// Create metadata
	metadata := cache.Metadata{
		Key:         cacheKey,
		Filename:    "test_file.txt",
		ContentType: "text/plain",
		Size:        int64(len(testContent)),
		ModTime:     time.Now(),
		Tags:        tags,
	}

	// Open file for reading
	file, err := os.Open(testFile)
	if err != nil {
		log.Printf("Failed to open test file: %v", err)
		return
	}
	defer file.Close()

	// Store in cache
	storedMeta, err := cacheTool.Put(ctx, cacheKey, metadata, file)
	if err != nil {
		log.Printf("Failed to store in cache: %v", err)
		return
	}
	log.Printf("Stored in cache with key: %s", storedMeta.Key)

	// Check if exists
	exists, err := cacheTool.Exists(ctx, cacheKey)
	if err != nil {
		log.Printf("Failed to check cache existence: %v", err)
	} else {
		log.Printf("Cache item exists: %v", exists)
	}

	// Retrieve from cache
	retrievedMeta, reader, err := cacheTool.Get(ctx, cacheKey)
	if err != nil {
		log.Printf("Failed to retrieve from cache: %v", err)
		return
	}
	defer reader.Close()

	// Read content
	content := make([]byte, retrievedMeta.Size)
	_, err = reader.Read(content)
	if err != nil {
		log.Printf("Failed to read cached content: %v", err)
		return
	}

	log.Printf("Retrieved from cache - Key: %s, Size: %d", retrievedMeta.Key, retrievedMeta.Size)
	log.Printf("Content (first 20 bytes): %s", content[:min(20, len(content))])

	// List cache items with tag filter
	items, err := cacheTool.List(ctx, map[string]string{"type": "test"})
	if err != nil {
		log.Printf("Failed to list cache items: %v", err)
	} else {
		log.Printf("Found %d cache items with tag type=test", len(items))
		for _, item := range items {
			log.Printf("  - %s (size: %d bytes)", item.Key, item.Size)
		}
	}

	// Cleanup
	os.Remove(testFile)
}

func demonstrateContainerOperations(ctx context.Context, provider *tools.TuringPiToolProvider) {
	log.Println("\n=== Demonstrating Container Operations ===")

	containerTool := provider.GetContainerTool()
	if containerTool == nil {
		log.Println("Container tool not available")
		return
	}

	// Set up a test container
	log.Println("Creating a test container...")

	config := container.ContainerConfig{
		Image:      "alpine:latest",
		Name:       "turingpi-test-container",
		Command:    []string{"sh", "-c", "sleep 60"},
		WorkDir:    "/app",
		Mounts:     map[string]string{},
		Env:        map[string]string{"TEST_VAR": "test_value"},
		Privileged: false,
	}

	testContainer, err := containerTool.CreateContainer(ctx, config)
	if err != nil {
		log.Printf("Failed to create container: %v", err)
		return
	}

	log.Printf("Container created with ID: %s", testContainer.ID())

	// Run a command in the container
	output, err := containerTool.RunCommand(ctx, testContainer.ID(), []string{"echo", "Hello from container!"})
	if err != nil {
		log.Printf("Failed to run command: %v", err)
	} else {
		log.Printf("Command output: %s", output)
	}

	// Clean up
	log.Println("Removing test container...")
	if err := containerTool.RemoveContainer(ctx, testContainer.ID()); err != nil {
		log.Printf("Failed to remove container: %v", err)
	} else {
		log.Printf("Container removed successfully")
	}
}

func demonstrateStateManagement(stateManager state.Manager) {
	log.Println("\n=== Demonstrating State Management ===")

	// Create node state
	nodeState := &state.NodeState{
		NodeID:            1,
		IPAddress:         "192.168.1.101",
		Hostname:          "turingpi-node1",
		LastOperation:     "power_on",
		LastOperationTime: time.Now(),
		Properties: map[string]interface{}{
			"board_type": "rk1",
			"os_version": "ubuntu-22.04",
			"memory":     "4GB",
		},
	}

	// Update state
	if err := stateManager.UpdateNodeState(nodeState); err != nil {
		log.Printf("Failed to update node state: %v", err)
		return
	}
	log.Printf("Node state updated for Node %d", nodeState.NodeID)

	// Get state
	retrievedState, err := stateManager.GetNodeState(state.NodeID(1))
	if err != nil {
		log.Printf("Failed to get node state: %v", err)
		return
	}

	log.Printf("Retrieved node state: Node %d, IP: %s, Hostname: %s",
		retrievedState.NodeID, retrievedState.IPAddress, retrievedState.Hostname)
	log.Printf("Last operation: %s at %s",
		retrievedState.LastOperation, retrievedState.LastOperationTime.Format(time.RFC3339))

	// Update properties
	newProperties := map[string]interface{}{
		"status":      "running",
		"update_time": time.Now().Format(time.RFC3339),
	}

	if err := stateManager.UpdateNodeProperties(state.NodeID(1), newProperties); err != nil {
		log.Printf("Failed to update node properties: %v", err)
		return
	}
	log.Printf("Node properties updated")

	// Record an operation
	if err := stateManager.RecordOperation(state.NodeID(1), "reboot", nil); err != nil {
		log.Printf("Failed to record operation: %v", err)
		return
	}
	log.Printf("Operation 'reboot' recorded")

	// List all node states
	allStates, err := stateManager.ListNodeStates()
	if err != nil {
		log.Printf("Failed to list node states: %v", err)
		return
	}

	log.Printf("Total managed nodes: %d", len(allStates))
	for _, ns := range allStates {
		log.Printf("Node %d - Last Operation: %s", ns.NodeID, ns.LastOperation)
	}

	// Save state
	if err := stateManager.SaveState(); err != nil {
		log.Printf("Failed to save state: %v", err)
	} else {
		log.Printf("State saved successfully")
	}
}

func setupUbuntuWorkflow(provider *tools.TuringPiToolProvider, tempDir, outputDir, cacheDir string) {
	log.Println("\n=== Setting Up Ubuntu Deployment Workflow ===")

	// Configure node settings
	nodeID := 1 // Node 1 is typically RK1 on TuringPi 2
	osVersion := "22.04"

	// Set up workflow options
	options := ubuntu.DefaultWorkflowOptions(nodeID, osVersion)
	options.SetNodePassword("turingpi123")

	// Set network configuration
	networkConfig := ubuntu.NetworkConfig{
		Hostname:   fmt.Sprintf("turingpi-node%d", nodeID),
		IPCIDR:     "192.168.1.101/24",
		Gateway:    "192.168.1.1",
		DNSServers: []string{"1.1.1.1", "8.8.8.8"},
	}
	options.SetNetworkConfig(networkConfig)

	// Add paths to the store
	options.AddStoreValue("cacheDir", cacheDir)
	options.AddStoreValue("tempDir", tempDir)
	options.AddStoreValue("outputDir", outputDir)

	// Set source image path (would be an actual path in real usage)
	sourceImageDir := filepath.Join(os.TempDir(), "turingpi_images")
	sourceImageName := "ubuntu-22.04.3-preinstalled-server-arm64-turing-rk1_v1.33.img.xz"
	baseImagePath := filepath.Join(sourceImageDir, sourceImageName)
	options.AddStoreValue("baseImagePath", baseImagePath)

	// Create the workflow
	log.Printf("Creating Ubuntu RK1 deployment workflow for node %d", nodeID)
	workflow, err := ubuntu.CreateRK1DeploymentWorkflow(options)
	if err != nil {
		log.Fatalf("Failed to create workflow: %v", err)
	}

	// Display workflow details
	log.Printf("Workflow created: %s", workflow.Name)
	log.Printf("Description: %s", workflow.Description)
	log.Printf("Stages: %d", len(workflow.Stages))

	for i, stage := range workflow.Stages {
		log.Printf("Stage %d: %s (%s)", i+1, stage.Name, stage.ID)
		log.Printf("  Description: %s", stage.Description)
		log.Printf("  Actions: %d", len(stage.Actions))
	}

	log.Println("Workflow setup complete. Ready for execution.")
	log.Println("Note: Actual execution requires the real TuringPi hardware or a proper container environment.")
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
