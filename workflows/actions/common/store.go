// Package common provides generic actions that can be used across different workflows
package common

import (
	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/keys"
	"github.com/davidroman0O/turingpi/workflows/actions"
)

// SetCurrentNodeAction sets the current node ID in the workflow
type SetCurrentNodeAction struct {
	actions.TuringPiAction
	nodeID int
}

// NewSetCurrentNodeAction creates a new action to set the current node
func NewSetCurrentNodeAction(nodeID int) *SetCurrentNodeAction {
	return &SetCurrentNodeAction{
		TuringPiAction: actions.NewTuringPiAction(
			"set-current-node",
			"Sets the current node for workflow operations",
		),
		nodeID: nodeID,
	}
}

// Execute implements the Action interface
func (a *SetCurrentNodeAction) Execute(ctx *gostage.ActionContext) error {
	return ctx.Store().Put(keys.CurrentNodeID, a.nodeID)
}

// AddTargetNodeAction adds a node to the list of target nodes
type AddTargetNodeAction struct {
	actions.TuringPiAction
	nodeID int
}

// NewAddTargetNodeAction creates a new action to add a target node
func NewAddTargetNodeAction(nodeID int) *AddTargetNodeAction {
	return &AddTargetNodeAction{
		TuringPiAction: actions.NewTuringPiAction(
			"add-target-node",
			"Adds a node to the list of target nodes",
		),
		nodeID: nodeID,
	}
}

// Execute implements the Action interface
func (a *AddTargetNodeAction) Execute(ctx *gostage.ActionContext) error {
	// Get current list or create new one
	targetNodes, err := store.GetOrDefault[[]int](ctx.Store(), keys.TargetNodes, []int{})
	if err != nil {
		return err
	}

	// Add the node if not already present
	found := false
	for _, id := range targetNodes {
		if id == a.nodeID {
			found = true
			break
		}
	}

	if !found {
		targetNodes = append(targetNodes, a.nodeID)
	}

	return ctx.Store().Put(keys.TargetNodes, targetNodes)
}
