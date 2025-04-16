package tpi

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

// executionStatus represents the state of a phase execution.
type executionStatus string

const (
	StatusPending   executionStatus = "pending"
	StatusRunning   executionStatus = "running"
	StatusFailed    executionStatus = "failed"
	StatusCompleted executionStatus = "completed"
)

// phaseState holds the state for a single phase of the workflow for a specific node.
type phaseState struct {
	Status          executionStatus `json:"status"`
	Timestamp       time.Time       `json:"timestamp"`
	InputHash       string          `json:"input_hash,omitempty"`        // SHA256 hash of inputs to detect changes
	OutputImagePath string          `json:"output_image_path,omitempty"` // Specific output for phase 1
	Error           string          `json:"error,omitempty"`             // Store error message on failure
}

// nodeState holds the state for all phases for a specific node.
type nodeState struct {
	ImageCustomization phaseState `json:"image_customization"`
	OSInstallation     phaseState `json:"os_installation"`
	PostInstallation   phaseState `json:"post_installation"`
}

// clusterState represents the overall state for all configured nodes.
// The keys are the NodeID enum (which is an int).
type clusterState map[NodeID]nodeState

// stateManager handles loading, saving, and locking the state file.
type stateManager struct {
	filePath string
	mu       sync.RWMutex // Internal mutex for concurrent access to the map in memory
	state    clusterState
	fileLock *flock.Flock // File lock to prevent concurrent writes by different processes
}

// newStateManager creates a new state manager instance and performs initial load.
func newStateManager(filePath string) (*stateManager, error) {
	mgr := &stateManager{
		filePath: filePath,
		state:    make(clusterState),
		// Use a lock file based on the state file path
		fileLock: flock.New(filePath + ".lock"),
	}

	if err := mgr.load(); err != nil {
		// If the file doesn't exist, it's not an error, just means empty state.
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to initially load state from %s: %w", filePath, err)
		}
		log.Printf("State file %s not found, starting with empty state.", filePath)
		// Ensure state is definitely empty if file didn't exist
		mgr.state = make(clusterState)
	}
	return mgr, nil
}

// load reads the state file into memory. It acquires an exclusive file lock.
func (sm *stateManager) load() error {
	// Acquire exclusive file lock (wait indefinitely) for reading/potential initial write
	// Using TryLock might be too aggressive if another process holds the lock briefly.
	if err := sm.fileLock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire file lock for %s: %w", sm.filePath, err)
	}
	defer func() {
		if err := sm.fileLock.Unlock(); err != nil {
			log.Printf("Error releasing file lock for %s after load: %v", sm.filePath, err)
		}
	}()

	// RLock the internal map mutex for reading the file content
	// Although we hold the file lock, other goroutines in *this* process might try to access the map
	// We upgrade to Write lock later if needed
	sm.mu.RLock()
	data, err := os.ReadFile(sm.filePath)
	sm.mu.RUnlock()

	if err != nil {
		// If the file simply doesn't exist, return that specific error
		if os.IsNotExist(err) {
			// Need write lock to ensure the map is empty
			sm.mu.Lock()
			sm.state = make(clusterState)
			sm.mu.Unlock()
			return err // Propagate os.IsNotExist, indicating we start fresh
		}
		return fmt.Errorf("failed to read state file %s: %w", sm.filePath, err)
	}

	// If file is empty, treat as empty state
	if len(data) == 0 {
		sm.mu.Lock()
		sm.state = make(clusterState)
		sm.mu.Unlock()
		return nil
	}

	var loadedState clusterState
	if err := json.Unmarshal(data, &loadedState); err != nil {
		// Handle potential corruption
		log.Printf("Warning: Failed to unmarshal state file %s: %v. Treating as empty state.", sm.filePath, err)
		sm.mu.Lock()
		sm.state = make(clusterState)
		sm.mu.Unlock()
		// Optionally save the empty state immediately to overwrite corrupted file?
		// if saveErr := sm.saveInternal(); saveErr != nil {
		//    log.Printf("Failed to save initial empty state after unmarshal error: %v", saveErr)
		// }
		return nil // Treat as empty state for now, don't return the unmarshal error directly
	}

	// Load successful, need write lock to update the map
	sm.mu.Lock()
	sm.state = loadedState
	sm.mu.Unlock()

	log.Printf("Loaded state for %d nodes from %s", len(sm.state), sm.filePath)
	return nil
}

// save writes the current in-memory state to the file. It acquires an exclusive file lock.
func (sm *stateManager) save() error {
	// Acquire exclusive file lock (wait indefinitely) for writing
	if err := sm.fileLock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire file lock for saving %s: %w", sm.filePath, err)
	}
	defer func() {
		if err := sm.fileLock.Unlock(); err != nil {
			log.Printf("Error releasing file lock for %s after save: %v", sm.filePath, err)
		}
	}()

	// Lock map for reading during marshalling
	sm.mu.RLock()
	data, err := json.MarshalIndent(sm.state, "", "  ")
	sm.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Perform the actual file writing
	if err := os.WriteFile(sm.filePath, data, 0640); err != nil {
		return fmt.Errorf("failed to write state file %s: %w", sm.filePath, err)
	}
	log.Printf("Saved state for %d nodes to %s", len(sm.state), sm.filePath)
	return nil
}

// --- Getters and Setters for State --- //

// GetNodeState retrieves the state for a specific node, initializing if not present.
// It handles internal locking.
func (sm *stateManager) GetNodeState(nodeID NodeID) nodeState {
	sm.mu.RLock()
	state, exists := sm.state[nodeID]
	sm.mu.RUnlock()

	if !exists {
		// Initialize with pending status if node state doesn't exist
		state = nodeState{
			ImageCustomization: phaseState{Status: StatusPending, Timestamp: time.Now()},
			OSInstallation:     phaseState{Status: StatusPending, Timestamp: time.Now()},
			PostInstallation:   phaseState{Status: StatusPending, Timestamp: time.Now()},
		}
		// Need write lock to add it to the map
		sm.mu.Lock()
		sm.state[nodeID] = state
		sm.mu.Unlock()
		// Save the newly initialized state immediately? Or wait for next explicit save?
		// Let's save it to ensure it persists if the program exits before next phase.
		if err := sm.save(); err != nil {
			log.Printf("Warning: failed to save state after initializing node %d: %v", nodeID, err)
		}
	}
	return state
}

// UpdatePhaseState updates the state for a specific phase of a specific node and saves the state file.
// It handles internal locking.
func (sm *stateManager) UpdatePhaseState(nodeID NodeID, phaseName string, status executionStatus, inputHash string, outputImagePath string, phaseErr error) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Ensure node state exists (it should if getNodeState was called previously)
	nState, exists := sm.state[nodeID]
	if !exists {
		// This is unexpected if getNodeState was used, but handle defensively
		log.Printf("Warning: Node %d state not found during updatePhaseState for phase %s. Initializing.", nodeID, phaseName)
		nState = nodeState{
			ImageCustomization: phaseState{Status: StatusPending},
			OSInstallation:     phaseState{Status: StatusPending},
			PostInstallation:   phaseState{Status: StatusPending},
		}
	}

	newState := phaseState{
		Status:          status,
		Timestamp:       time.Now(),
		InputHash:       inputHash,
		OutputImagePath: outputImagePath, // Will be empty unless phase 1 completes
	}
	if phaseErr != nil {
		newState.Status = StatusFailed // Ensure status is Failed if error provided
		newState.Error = phaseErr.Error()
	}

	found := true
	switch phaseName {
	case "ImageCustomization":
		nState.ImageCustomization = newState
	case "OSInstallation":
		nState.OSInstallation = newState
	case "PostInstallation":
		nState.PostInstallation = newState
	default:
		found = false
		log.Printf("Warning: Attempted to update unknown phase '%s' for node %d", phaseName, nodeID)
	}

	if found {
		sm.state[nodeID] = nState
		// Release lock before saving to avoid deadlock if save calls load internally (it doesn't currently)
		sm.mu.Unlock()
		err := sm.saveInternalRequiresLock() // Save the updated state
		sm.mu.Lock()                         // Re-acquire lock before defer unlocks
		if err != nil {
			return fmt.Errorf("failed to save state after updating phase %s for node %d: %w", phaseName, nodeID, err)
		}
	}

	return nil
}

// saveInternalRequiresLock performs the actual file writing, assuming the *internal map lock* is held.
// It acquires the file lock itself.
func (sm *stateManager) saveInternalRequiresLock() error {
	// Acquire exclusive file lock for writing
	if err := sm.fileLock.Lock(); err != nil {
		return fmt.Errorf("failed to acquire file lock for saving %s: %w", sm.filePath, err)
	}
	defer func() {
		if err := sm.fileLock.Unlock(); err != nil {
			log.Printf("Error releasing file lock for %s after save: %v", sm.filePath, err)
		}
	}()

	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	if err := os.WriteFile(sm.filePath, data, 0640); err != nil {
		return fmt.Errorf("failed to write state file %s: %w", sm.filePath, err)
	}
	log.Printf("Saved state for %d nodes to %s", len(sm.state), sm.filePath)
	return nil
}

// TODO: Add functions for hashing inputs.
// func calculateInputHash(inputs ...interface{}) (string, error) { ... }
