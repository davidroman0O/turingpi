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
	if state.Properties != nil {
		stateCopy.Properties = make(map[string]interface{}, len(state.Properties))
		for k, v := range state.Properties {
			stateCopy.Properties[k] = v
		}
	}
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
	if state.Properties != nil {
		stateCopy.Properties = make(map[string]interface{}, len(state.Properties))
		for k, v := range state.Properties {
			stateCopy.Properties[k] = v
		}
	}
	m.state.Nodes[state.NodeID] = &stateCopy
	m.state.LastUpdated = time.Now()

	return m.saveState()
}

// UpdateNodeProperties updates specific properties of a node state
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
			NodeID:     nodeID,
			Properties: make(map[string]interface{}),
		}
		m.state.Nodes[nodeID] = state
	} else if state.Properties == nil {
		state.Properties = make(map[string]interface{})
	}

	// Use reflection to update fields by name
	stateValue := reflect.ValueOf(state).Elem()
	stateType := stateValue.Type()

	for propName, propValue := range properties {
		// First try to update a struct field
		var field reflect.Value
		var found bool

		for i := 0; i < stateType.NumField(); i++ {
			if stateType.Field(i).Name == propName {
				field = stateValue.Field(i)
				found = true
				break
			}
		}

		if found {
			if !field.CanSet() {
				continue // Skip if field cannot be set
			}

			val := reflect.ValueOf(propValue)

			// Check if types are directly assignable
			if val.Type().AssignableTo(field.Type()) {
				field.Set(val)
				continue
			}

			// Handle type conversions
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
			case reflect.Bool:
				if val.Kind() == reflect.Bool {
					field.SetBool(val.Bool())
				}
			}
		} else {
			// If not a struct field, store in Properties map
			state.Properties[propName] = propValue
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
		// Create a copy of the state
		stateCopy := *state
		if state.Properties != nil {
			stateCopy.Properties = make(map[string]interface{}, len(state.Properties))
			for k, v := range state.Properties {
				stateCopy.Properties[k] = v
			}
		}
		states = append(states, &stateCopy)
	}

	return states, nil
}

// RecordOperation logs an operation without validation
func (m *FileStateManager) RecordOperation(nodeID NodeID, operation string, result error) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	state, exists := m.state.Nodes[nodeID]
	if !exists {
		state = &NodeState{
			NodeID: nodeID,
		}
		m.state.Nodes[nodeID] = state
	}

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

// saveState persists the state to disk (internal, must be called with lock held)
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

// loadState loads the state from disk (internal, must be called with lock held)
func (m *FileStateManager) loadState() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return fmt.Errorf("failed to read state file: %w", err)
	}

	if err := json.Unmarshal(data, &m.state); err != nil {
		return fmt.Errorf("failed to unmarshal state: %w", err)
	}

	return nil
}
