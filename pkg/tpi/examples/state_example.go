// Package examples provides example code for using the tftpi components.
package examples

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/state"
)

// This example demonstrates how to use the state package with the new flexible property updates
func StateManagementExample() error {
	// Create a temporary file for testing
	tempDir, err := os.MkdirTemp("", "tftpi-example")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	stateFilePath := filepath.Join(tempDir, "state.json")

	// Create a new state manager
	manager, err := state.NewFileStateManager(stateFilePath)
	if err != nil {
		return fmt.Errorf("failed to create state manager: %w", err)
	}

	// Traditional approach: Create and update a complete node state
	nodeState := &state.NodeState{
		NodeID:            state.NodeID(1),
		BoardType:         state.RK1,
		OSType:            "ubuntu",
		OSVersion:         "22.04",
		IPAddress:         "192.168.1.101",
		Hostname:          "node1",
		LastOperation:     "Initialize",
		LastOperationTime: time.Now(),
	}

	// Update the full state
	if err := manager.UpdateNodeState(nodeState); err != nil {
		return fmt.Errorf("failed to update node state: %w", err)
	}

	// New flexible approach: Update specific properties
	properties := map[string]interface{}{
		"IPAddress": "10.0.0.101",
		"Hostname":  "node1-updated",
	}

	// Update just those properties
	if err := manager.UpdateNodeProperties(state.NodeID(1), properties); err != nil {
		return fmt.Errorf("failed to update node properties: %w", err)
	}

	// Recording an operation also updates the LastOperation fields automatically
	if err := manager.RecordOperation(state.NodeID(1), "ConfigureNetwork", nil); err != nil {
		return fmt.Errorf("failed to record operation: %w", err)
	}

	// We can also update multiple properties during different phases
	// of our workflow without creating a complete NodeState object each time

	// During image preparation
	if err := manager.UpdateNodeProperties(state.NodeID(1), map[string]interface{}{
		"LastImagePath": "/path/to/image.img",
		"LastImageHash": "abcdef123456",
		"LastImageTime": time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to update image properties: %w", err)
	}

	// During OS installation
	if err := manager.UpdateNodeProperties(state.NodeID(1), map[string]interface{}{
		"LastInstallTime": time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to update install time: %w", err)
	}

	// During post-installation configuration
	if err := manager.UpdateNodeProperties(state.NodeID(1), map[string]interface{}{
		"LastConfigTime": time.Now(),
	}); err != nil {
		return fmt.Errorf("failed to update config time: %w", err)
	}

	// Get the current state
	currentState, err := manager.GetNodeState(state.NodeID(1))
	if err != nil {
		return fmt.Errorf("failed to get node state: %w", err)
	}

	// Print the current state (just as an example)
	fmt.Printf("Node ID: %d\n", currentState.NodeID)
	fmt.Printf("IP Address: %s\n", currentState.IPAddress)
	fmt.Printf("Hostname: %s\n", currentState.Hostname)
	fmt.Printf("Last Operation: %s\n", currentState.LastOperation)

	return nil
}

// PhaseBasedStateManagement shows how to implement a phase-based approach using the flexible state system
func PhaseBasedStateManagement() error {
	// Create a temporary file for testing
	tempDir, err := os.MkdirTemp("", "tftpi-example")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	stateFilePath := filepath.Join(tempDir, "state.json")

	// Create a new state manager
	manager, err := state.NewFileStateManager(stateFilePath)
	if err != nil {
		return fmt.Errorf("failed to create state manager: %w", err)
	}

	// Define phases
	const (
		PhaseImageCustomization = "ImageCustomization"
		PhaseOSInstallation     = "OSInstallation"
		PhasePostInstallation   = "PostInstallation"
	)

	// Define statuses
	const (
		StatusPending   = "pending"
		StatusRunning   = "running"
		StatusFailed    = "failed"
		StatusCompleted = "completed"
	)

	nodeID := state.NodeID(1)

	// Initialize node with phase status as a string
	// This will be stored even though it's not defined in the NodeState struct
	if err := manager.UpdateNodeProperties(nodeID, map[string]interface{}{
		"LastOperation":     "Initialize",
		"LastOperationTime": time.Now(),
		"PhaseStatus":       StatusPending,
	}); err != nil {
		return fmt.Errorf("failed to initialize node: %w", err)
	}

	// Phase 1: Image Customization - Start
	if err := manager.UpdateNodeProperties(nodeID, map[string]interface{}{
		"LastOperation":     fmt.Sprintf("Start%s", PhaseImageCustomization),
		"LastOperationTime": time.Now(),
		"PhaseStatus":       StatusRunning,
	}); err != nil {
		return fmt.Errorf("failed to start image customization: %w", err)
	}

	// Simulate work...
	time.Sleep(10 * time.Millisecond)

	// Phase 1: Image Customization - Complete
	if err := manager.UpdateNodeProperties(nodeID, map[string]interface{}{
		"LastOperation":     fmt.Sprintf("Complete%s", PhaseImageCustomization),
		"LastOperationTime": time.Now(),
		"LastImagePath":     "/path/to/image.img",
		"LastImageHash":     "abcdef123456",
		"LastImageTime":     time.Now(),
		"PhaseStatus":       StatusCompleted,
	}); err != nil {
		return fmt.Errorf("failed to complete image customization: %w", err)
	}

	// Phase 2: OS Installation - Start
	if err := manager.UpdateNodeProperties(nodeID, map[string]interface{}{
		"LastOperation":     fmt.Sprintf("Start%s", PhaseOSInstallation),
		"LastOperationTime": time.Now(),
		"PhaseStatus":       StatusRunning,
	}); err != nil {
		return fmt.Errorf("failed to start OS installation: %w", err)
	}

	// Simulate work...
	time.Sleep(10 * time.Millisecond)

	// Phase 2: OS Installation - Complete
	if err := manager.UpdateNodeProperties(nodeID, map[string]interface{}{
		"LastOperation":     fmt.Sprintf("Complete%s", PhaseOSInstallation),
		"LastOperationTime": time.Now(),
		"LastInstallTime":   time.Now(),
		"PhaseStatus":       StatusCompleted,
	}); err != nil {
		return fmt.Errorf("failed to complete OS installation: %w", err)
	}

	// Phase 3: Post-Installation - Start
	if err := manager.UpdateNodeProperties(nodeID, map[string]interface{}{
		"LastOperation":     fmt.Sprintf("Start%s", PhasePostInstallation),
		"LastOperationTime": time.Now(),
		"PhaseStatus":       StatusRunning,
	}); err != nil {
		return fmt.Errorf("failed to start post-installation: %w", err)
	}

	// Simulate work...
	time.Sleep(10 * time.Millisecond)

	// Phase 3: Post-Installation - Complete
	if err := manager.UpdateNodeProperties(nodeID, map[string]interface{}{
		"LastOperation":     fmt.Sprintf("Complete%s", PhasePostInstallation),
		"LastOperationTime": time.Now(),
		"LastConfigTime":    time.Now(),
		"PhaseStatus":       StatusCompleted,
		"IPAddress":         "192.168.1.101", // Set during post-installation
		"Hostname":          "node1",         // Set during post-installation
	}); err != nil {
		return fmt.Errorf("failed to complete post-installation: %w", err)
	}

	// Get the final state
	currentState, err := manager.GetNodeState(nodeID)
	if err != nil {
		return fmt.Errorf("failed to get node state: %w", err)
	}

	// Print the current state
	fmt.Printf("Node ID: %d\n", currentState.NodeID)
	fmt.Printf("IP Address: %s\n", currentState.IPAddress)
	fmt.Printf("Hostname: %s\n", currentState.Hostname)
	fmt.Printf("Last Operation: %s\n", currentState.LastOperation)
	// Note: PhaseStatus is not directly accessible since it's not part of the struct
	// But we could retrieve it using a custom accessor if we added that functionality

	return nil
}
