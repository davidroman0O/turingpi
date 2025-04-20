package tpi

import (
	"github.com/davidroman0O/turingpi/pkg/tpi/state"
)

// tpiStateAdapter adapts state.Manager to StateManager interface
type tpiStateAdapter struct {
	manager state.Manager
}

// GetNodeState retrieves the state for a specific node
func (a *tpiStateAdapter) GetNodeState(nodeID NodeID) (*NodeState, error) {
	stateData, err := a.manager.GetNodeState(state.NodeID(nodeID))
	if err != nil {
		return nil, err
	}
	if stateData == nil {
		return nil, nil
	}
	return &NodeState{
		LastImageTime: stateData.LastImageTime,
		LastImageHash: stateData.LastImageHash,
		LastImagePath: stateData.LastImagePath,
		LastError:     stateData.LastError,
	}, nil
}

// UpdateNodeState updates the state for a specific node
func (a *tpiStateAdapter) UpdateNodeState(nodeState *NodeState) error {
	if nodeState == nil {
		return nil
	}
	stateData := &state.NodeState{
		LastImageTime: nodeState.LastImageTime,
		LastImageHash: nodeState.LastImageHash,
		LastImagePath: nodeState.LastImagePath,
		LastError:     nodeState.LastError,
	}
	return a.manager.UpdateNodeState(stateData)
}

// newStateAdapter creates a new adapter for state.Manager
func newStateAdapter(manager state.Manager) StateManager {
	return &tpiStateAdapter{manager: manager}
}

// GetStateManager returns the state management interface.
func (p *TuringPiProvider) GetStateManager() StateManager {
	return newStateAdapter(p.stateManager)
}
