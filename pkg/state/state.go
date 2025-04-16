package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// NodeStatus represents the known status of a compute node.
type NodeStatus struct {
	NodeID     int       `json:"node_id"`
	IPAddress  string    `json:"ip_address,omitempty"`
	MACAddress string    `json:"mac_address,omitempty"`
	LastSeen   time.Time `json:"last_seen,omitempty"`
	Status     string    `json:"status,omitempty"` // e.g., "unknown", "discovered", "installing", "configured"
	OS         string    `json:"os,omitempty"`     // e.g., "agent", "ubuntu", "debian"
	Error      string    `json:"error,omitempty"`  // Record last error
}

// State represents the overall known state of all nodes.
type State struct {
	Nodes map[int]*NodeStatus `json:"nodes"`
}

const stateFileName = "state.json"

var (
	stateFilePath string
	stateMutex    sync.RWMutex
	once          sync.Once
)

// initStatePath ensures the configuration directory and state file path are set up.
func initStatePath() {
	once.Do(func() {
		configDir, err := os.UserConfigDir()
		if err != nil {
			// Fallback to home directory if config dir fails
			configDir, err = os.UserHomeDir()
			if err != nil {
				// Extremely unlikely fallback
				configDir = "."
			}
		}
		cliConfigPath := filepath.Join(configDir, "turingpi-cli")
		if err := os.MkdirAll(cliConfigPath, 0750); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Could not create config directory %s: %v\n", cliConfigPath, err)
			// Use current directory as fallback for state file path
			stateFilePath = stateFileName
		} else {
			stateFilePath = filepath.Join(cliConfigPath, stateFileName)
		}
		fmt.Printf("Using state file: %s\n", stateFilePath)
	})
}

// GetStateFilePath returns the path to the state file.
func GetStateFilePath() string {
	initStatePath()
	return stateFilePath
}

// LoadState reads the state from the JSON file.
// If the file doesn't exist, it returns an empty state.
func LoadState() (*State, error) {
	initStatePath()
	stateMutex.RLock()
	defer stateMutex.RUnlock()

	data, err := os.ReadFile(stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return new empty state
			return &State{Nodes: make(map[int]*NodeStatus)}, nil
		}
		return nil, fmt.Errorf("failed to read state file %s: %w", stateFilePath, err)
	}

	var s State
	if len(data) == 0 { // Handle empty file case
		return &State{Nodes: make(map[int]*NodeStatus)}, nil
	}

	err = json.Unmarshal(data, &s)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal state file %s: %w", stateFilePath, err)
	}

	// Ensure map is initialized if file was empty JSON object like {}
	if s.Nodes == nil {
		s.Nodes = make(map[int]*NodeStatus)
	}

	return &s, nil
}

// SaveState writes the current state to the JSON file.
func SaveState(s *State) error {
	initStatePath()
	stateMutex.Lock()
	defer stateMutex.Unlock()

	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	err = os.WriteFile(stateFilePath, data, 0640)
	if err != nil {
		return fmt.Errorf("failed to write state file %s: %w", stateFilePath, err)
	}
	return nil
}

// UpdateNodeState updates the state for a specific node and saves the file.
func UpdateNodeState(nodeID int, updateFunc func(status *NodeStatus)) error {
	s, err := LoadState() // Load current state
	if err != nil {
		return fmt.Errorf("failed to load state for update: %w", err)
	}

	nodeStatus, exists := s.Nodes[nodeID]
	if !exists {
		nodeStatus = &NodeStatus{NodeID: nodeID} // Create if not exists
		s.Nodes[nodeID] = nodeStatus
	}

	updateFunc(nodeStatus)           // Apply the update
	nodeStatus.LastSeen = time.Now() // Update timestamp

	return SaveState(s) // Save updated state
}
