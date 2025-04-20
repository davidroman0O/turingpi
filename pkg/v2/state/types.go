package state

import "time"

// NodeID represents a compute node identifier
type NodeID int

// NodeState represents the last known state of a node
type NodeState struct {
	NodeID NodeID `json:"nodeID"`

	// Last known network configuration
	IPAddress string `json:"ipAddress"`
	Hostname  string `json:"hostname"`

	// Last operation result
	LastOperation     string    `json:"lastOperation"`
	LastOperationTime time.Time `json:"lastOperationTime"`
	LastError         string    `json:"lastError,omitempty"`

	// Custom properties
	Properties map[string]interface{} `json:"properties,omitempty"`
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
	UpdateNodeProperties(nodeID NodeID, properties map[string]interface{}) error

	// SaveState persists the current state
	SaveState() error
}

// StateManager is an alias for Manager to maintain backward compatibility
type StateManager = Manager
