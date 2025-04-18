// Package state provides non-constraining state tracking for the tftpi tool.
// It records the last known state of operations on nodes without enforcing workflows.
package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"time"
)

// NodeID represents a compute node identifier
type NodeID int

// BoardType identifies the type of compute module
type BoardType string

// Predefined board types
const (
	RK1 BoardType = "rk1"
	CM4 BoardType = "cm4"
)

// NodeState represents the last known state of a node
// This is purely informational and doesn't constrain operations
type NodeState struct {
	NodeID    NodeID    `json:"nodeID"`
	BoardType BoardType `json:"boardType"`

	// Last known OS information
	OSType    string `json:"osType"`    // e.g., "ubuntu"
	OSVersion string `json:"osVersion"` // e.g., "22.04"

	// Last image preparation info
	LastImagePath string    `json:"lastImagePath"`
	LastImageHash string    `json:"lastImageHash"`
	LastImageTime time.Time `json:"lastImageTime"`

	// Last installation info
	LastInstallTime time.Time `json:"lastInstallTime"`

	// Last configuration info
	LastConfigTime time.Time `json:"lastConfigTime"`

	// Last known network configuration
	IPAddress string `json:"ipAddress"`
	Hostname  string `json:"hostname"`

	// Last operation result
	LastOperation     string    `json:"lastOperation"`
	LastOperationTime time.Time `json:"lastOperationTime"`
	LastError         string    `json:"lastError,omitempty"`
}

// SystemState holds the state for all nodes
type SystemState struct {
	Nodes       map[NodeID]*NodeState `json:"nodes"`
	LastUpdated time.Time             `json:"lastUpdated"`
}

// Manager handles persistence and querying of node states
type Manager interface {
	// GetNodeState returns the last known state of a node
	GetNodeState(nodeID NodeID) (*NodeState, error)

	// UpdateNodeState updates the state record of a node
	UpdateNodeState(state *NodeState) error

	// ListNodeStates returns all known node states
	ListNodeStates() ([]*NodeState, error)

	// RecordOperation logs an operation without validation
	RecordOperation(nodeID NodeID, operation string, result error) error

	// UpdateNodeProperties updates specific properties of a node state
	// The properties parameter is a map where keys are property names and values are the new values
	UpdateNodeProperties(nodeID NodeID, properties map[string]interface{}) error

	// SaveState persists the current state
	SaveState() error
}

// StateManager is an alias for Manager to maintain backward compatibility
type StateManager = Manager

// FileStateManager implements Manager with file-based persistence
type FileStateManager struct {
	filePath string
	state    SystemState
	mutex    sync.RWMutex
}

// NewFileStateManager creates a state manager that persists to a file
func NewFileStateManager(filePath string) (Manager, error) {
	manager := &FileStateManager{
		filePath: filePath,
		state: SystemState{
			Nodes:       make(map[NodeID]*NodeState),
			LastUpdated: time.Now(),
		},
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create state directory: %w", err)
	}

	// Try to load existing state
	if _, err := os.Stat(filePath); err == nil {
		if err := manager.loadState(); err != nil {
			return nil, fmt.Errorf("failed to load existing state: %w", err)
		}
	}

	return manager, nil
}

// GetNodeState retrieves the state for a specific node
func (m *FileStateManager) GetNodeState(nodeID NodeID) (*NodeState, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	state, exists := m.state.Nodes[nodeID]
	if !exists {
		return nil, nil // Return nil, nil if no state exists for this node
	}

	// Return a copy to prevent external modification
	stateCopy := *state
	return &stateCopy, nil
}

// UpdateNodeState updates the state for a node
func (m *FileStateManager) UpdateNodeState(state *NodeState) error {
	if state == nil {
		return fmt.Errorf("cannot update with nil state")
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Make a copy of the state
	stateCopy := *state
	m.state.Nodes[state.NodeID] = &stateCopy
	m.state.LastUpdated = time.Now()

	return m.saveState()
}

// UpdateNodeProperties updates specific properties of a node state
// This provides a more flexible way to update individual fields without replacing the entire state
func (m *FileStateManager) UpdateNodeProperties(nodeID NodeID, properties map[string]interface{}) error {
	if len(properties) == 0 {
		return nil // Nothing to update
	}

	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get or create node state
	state, exists := m.state.Nodes[nodeID]
	if !exists {
		state = &NodeState{
			NodeID: nodeID,
		}
		m.state.Nodes[nodeID] = state
	}

	// Use reflection to update fields by name
	stateValue := reflect.ValueOf(state).Elem()
	stateType := stateValue.Type()

	for propName, propValue := range properties {
		// Find field by name (case sensitive)
		var field reflect.Value
		var found bool

		for i := 0; i < stateType.NumField(); i++ {
			if stateType.Field(i).Name == propName {
				field = stateValue.Field(i)
				found = true
				break
			}
		}

		if !found {
			continue // Skip fields that don't exist
		}

		// Try to set the value, handling different types
		if !field.CanSet() {
			continue // Skip if field cannot be set
		}

		val := reflect.ValueOf(propValue)

		// Check if types are directly assignable
		if val.Type().AssignableTo(field.Type()) {
			field.Set(val)
			continue
		}

		// Handle some common type conversions
		switch field.Kind() {
		case reflect.String:
			if val.Kind() == reflect.String {
				field.SetString(val.String())
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if val.Kind() >= reflect.Int && val.Kind() <= reflect.Int64 {
				field.SetInt(val.Int())
			} else if val.Kind() >= reflect.Uint && val.Kind() <= reflect.Uint64 {
				field.SetInt(int64(val.Uint()))
			} else if val.Kind() == reflect.Float32 || val.Kind() == reflect.Float64 {
				field.SetInt(int64(val.Float()))
			}
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if val.Kind() >= reflect.Uint && val.Kind() <= reflect.Uint64 {
				field.SetUint(val.Uint())
			} else if val.Kind() >= reflect.Int && val.Kind() <= reflect.Int64 {
				field.SetUint(uint64(val.Int()))
			} else if val.Kind() == reflect.Float32 || val.Kind() == reflect.Float64 {
				field.SetUint(uint64(val.Float()))
			}
		case reflect.Bool:
			if val.Kind() == reflect.Bool {
				field.SetBool(val.Bool())
			}
		case reflect.Float32, reflect.Float64:
			if val.Kind() == reflect.Float32 || val.Kind() == reflect.Float64 {
				field.SetFloat(val.Float())
			} else if val.Kind() >= reflect.Int && val.Kind() <= reflect.Int64 {
				field.SetFloat(float64(val.Int()))
			} else if val.Kind() >= reflect.Uint && val.Kind() <= reflect.Uint64 {
				field.SetFloat(float64(val.Uint()))
			}
		}
	}

	m.state.LastUpdated = time.Now()
	return m.saveState()
}

// ListNodeStates returns all tracked node states
func (m *FileStateManager) ListNodeStates() ([]*NodeState, error) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	states := make([]*NodeState, 0, len(m.state.Nodes))
	for _, state := range m.state.Nodes {
		// Make a copy of each state
		stateCopy := *state
		states = append(states, &stateCopy)
	}

	return states, nil
}

// RecordOperation logs an operation performed on a node
func (m *FileStateManager) RecordOperation(nodeID NodeID, operation string, result error) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Get or create node state
	state, exists := m.state.Nodes[nodeID]
	if !exists {
		state = &NodeState{
			NodeID: nodeID,
		}
		m.state.Nodes[nodeID] = state
	}

	// Update operation info
	state.LastOperation = operation
	state.LastOperationTime = time.Now()

	if result != nil {
		state.LastError = result.Error()
	} else {
		state.LastError = ""
	}

	m.state.LastUpdated = time.Now()

	return m.saveState()
}

// SaveState persists the current state
func (m *FileStateManager) SaveState() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.saveState()
}

// saveState is the internal implementation of state persistence
// Caller must hold the lock
func (m *FileStateManager) saveState() error {
	data, err := json.MarshalIndent(m.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return nil
}

// loadState loads the state from the file
// Caller must hold the lock
func (m *FileStateManager) loadState() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var state SystemState
	if err := json.Unmarshal(data, &state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	// Initialize the nodes map if it's nil
	if state.Nodes == nil {
		state.Nodes = make(map[NodeID]*NodeState)
	}

	m.state = state
	return nil
}

// DefaultStateManager returns the default state manager instance
var defaultManager Manager

// InitDefaultManager initializes the default state manager
func InitDefaultManager(cacheDir string) error {
	stateFilePath := filepath.Join(cacheDir, "tftpi_state.json")
	manager, err := NewFileStateManager(stateFilePath)
	if err != nil {
		return err
	}

	defaultManager = manager
	return nil
}

// GetDefaultManager returns the default state manager
func GetDefaultManager() Manager {
	return defaultManager
}

// NodeIDFromInt converts an int to a NodeID
func NodeIDFromInt(id int) NodeID {
	return NodeID(id)
}
