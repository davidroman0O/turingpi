// Package workflows provides ready-to-use workflow definitions for TuringPi operations
package workflows

import (
	"fmt"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/turingpi/pkg/v2/workflows/actions/common"
	node "github.com/davidroman0O/turingpi/pkg/v2/workflows/stages"
)

// CreateNodeResetWorkflow creates a workflow for resetting a node
func CreateNodeResetWorkflow(nodeID int, hardReset bool) *gostage.Workflow {
	workflow := gostage.NewWorkflow(
		fmt.Sprintf("node-%d-reset", nodeID),
		fmt.Sprintf("Reset Node %d", nodeID),
		fmt.Sprintf("Reset workflow for TuringPi node %d", nodeID),
	)

	// Add initialization stage to set up node ID
	initStage := gostage.NewStage(
		"init",
		"Initialization",
		"Set up workflow parameters",
	)
	initStage.AddAction(common.NewSetCurrentNodeAction(nodeID))
	workflow.AddStage(initStage)

	// Add the appropriate reset stage based on reset type
	if hardReset {
		workflow.AddStage(node.CreateHardResetStage())
	} else {
		workflow.AddStage(node.CreateResetStage())
	}

	return workflow
}

// NodeResetOptions provides configuration options for node reset
type NodeResetOptions struct {
	NodeID    int  // ID of the node to reset
	HardReset bool // Whether to use hard reset (true) or soft reset (false)
	WaitTime  int  // Custom wait time in seconds (0 for default)
}

// DefaultNodeResetOptions returns the default options for node reset
func DefaultNodeResetOptions(nodeID int) *NodeResetOptions {
	return &NodeResetOptions{
		NodeID:    nodeID,
		HardReset: false,
		WaitTime:  10,
	}
}

// CreateNodeResetWorkflowWithOptions creates a workflow for resetting a node with options
func CreateNodeResetWorkflowWithOptions(options *NodeResetOptions) *gostage.Workflow {
	workflow := gostage.NewWorkflow(
		fmt.Sprintf("node-%d-reset", options.NodeID),
		fmt.Sprintf("Reset Node %d", options.NodeID),
		fmt.Sprintf("Reset workflow for TuringPi node %d", options.NodeID),
	)

	// Add initialization stage to set up node ID
	initStage := gostage.NewStage(
		"init",
		"Initialization",
		"Set up workflow parameters",
	)
	initStage.AddAction(common.NewSetCurrentNodeAction(options.NodeID))

	// Store any custom options in the workflow
	if options.WaitTime > 0 && options.WaitTime != 10 {
		workflow.Store.Put("customWaitTime", options.WaitTime)
	}

	workflow.AddStage(initStage)

	// Add the appropriate reset stage based on reset type
	if options.HardReset {
		workflow.AddStage(node.CreateHardResetStage())
	} else {
		workflow.AddStage(node.CreateResetStage())
	}

	return workflow
}
