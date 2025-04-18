package state

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFileStateManager(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "tftpi-state-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	statePath := filepath.Join(tempDir, "test_state.json")

	// Create new state manager
	manager, err := NewFileStateManager(statePath)
	assert.NoError(t, err)
	assert.NotNil(t, manager)

	// Test getting state for non-existent node
	state, err := manager.GetNodeState(1)
	assert.NoError(t, err)
	assert.Nil(t, state)

	// Test updating node state
	testState := &NodeState{
		NodeID:            1,
		BoardType:         RK1,
		OSType:            "ubuntu",
		OSVersion:         "22.04",
		IPAddress:         "192.168.1.101",
		Hostname:          "node1",
		LastOperation:     "Test",
		LastOperationTime: time.Now(),
	}

	err = manager.UpdateNodeState(testState)
	assert.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(statePath)
	assert.NoError(t, err)

	// Test getting the state back
	retrievedState, err := manager.GetNodeState(1)
	assert.NoError(t, err)
	assert.NotNil(t, retrievedState)
	assert.Equal(t, NodeID(1), retrievedState.NodeID)
	assert.Equal(t, RK1, retrievedState.BoardType)
	assert.Equal(t, "ubuntu", retrievedState.OSType)
	assert.Equal(t, "192.168.1.101", retrievedState.IPAddress)

	// Test listing all states
	states, err := manager.ListNodeStates()
	assert.NoError(t, err)
	assert.Len(t, states, 1)

	// Test recording operation
	err = manager.RecordOperation(1, "PrepareImage", nil)
	assert.NoError(t, err)

	// Get updated state
	updatedState, err := manager.GetNodeState(1)
	assert.NoError(t, err)
	assert.Equal(t, "PrepareImage", updatedState.LastOperation)
	assert.Equal(t, "", updatedState.LastError)

	// Test recording operation with error
	testError := errors.New("test error")
	err = manager.RecordOperation(1, "InstallOS", testError)
	assert.NoError(t, err)

	// Get updated state
	updatedState, err = manager.GetNodeState(1)
	assert.NoError(t, err)
	assert.Equal(t, "InstallOS", updatedState.LastOperation)
	assert.Equal(t, "test error", updatedState.LastError)

	// Test recording operation for new node
	err = manager.RecordOperation(2, "PrepareImage", nil)
	assert.NoError(t, err)

	// Verify there are now 2 nodes
	states, err = manager.ListNodeStates()
	assert.NoError(t, err)
	assert.Len(t, states, 2)

	// Test UpdateNodeProperties for existing node
	err = manager.UpdateNodeProperties(1, map[string]interface{}{
		"OSType":    "debian",
		"OSVersion": "11",
		"IPAddress": "10.0.0.101",
		"Hostname":  "node1-updated",
	})
	assert.NoError(t, err)

	// Verify properties were updated
	updatedState, err = manager.GetNodeState(1)
	assert.NoError(t, err)
	assert.Equal(t, "debian", updatedState.OSType)
	assert.Equal(t, "11", updatedState.OSVersion)
	assert.Equal(t, "10.0.0.101", updatedState.IPAddress)
	assert.Equal(t, "node1-updated", updatedState.Hostname)

	// Test UpdateNodeProperties for non-existent node
	err = manager.UpdateNodeProperties(3, map[string]interface{}{
		"OSType":    "ubuntu",
		"OSVersion": "22.10",
		"BoardType": CM4,
	})
	assert.NoError(t, err)

	// Verify node was created with properties
	newState, err := manager.GetNodeState(3)
	assert.NoError(t, err)
	assert.NotNil(t, newState)
	assert.Equal(t, NodeID(3), newState.NodeID)
	assert.Equal(t, "ubuntu", newState.OSType)
	assert.Equal(t, "22.10", newState.OSVersion)
	assert.Equal(t, CM4, newState.BoardType)

	// Create a new manager with the same file path to test persistence
	newManager, err := NewFileStateManager(statePath)
	assert.NoError(t, err)

	// Verify state persisted
	states, err = newManager.ListNodeStates()
	assert.NoError(t, err)
	assert.Len(t, states, 3)

	// Test specific field values from loaded state
	node1State, err := newManager.GetNodeState(1)
	assert.NoError(t, err)
	assert.Equal(t, "InstallOS", node1State.LastOperation)
	assert.Equal(t, "test error", node1State.LastError)
	assert.Equal(t, "debian", node1State.OSType)

	node2State, err := newManager.GetNodeState(2)
	assert.NoError(t, err)
	assert.Equal(t, "PrepareImage", node2State.LastOperation)

	node3State, err := newManager.GetNodeState(3)
	assert.NoError(t, err)
	assert.Equal(t, "ubuntu", node3State.OSType)
	assert.Equal(t, "22.10", node3State.OSVersion)
}

func TestDefaultManager(t *testing.T) {
	// Create temporary directory for test
	tempDir, err := os.MkdirTemp("", "tftpi-default-state-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Initialize default manager
	err = InitDefaultManager(tempDir)
	assert.NoError(t, err)

	// Get the default manager
	manager := GetDefaultManager()
	assert.NotNil(t, manager)

	// Test basic functionality
	err = manager.RecordOperation(1, "TestOperation", nil)
	assert.NoError(t, err)

	state, err := manager.GetNodeState(1)
	assert.NoError(t, err)
	assert.NotNil(t, state)
	assert.Equal(t, "TestOperation", state.LastOperation)
}
