// Package node provides stages for TuringPi node operations
package node

import (
	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/turingpi/pkg/v2/actions/bmc"
	"github.com/davidroman0O/turingpi/pkg/v2/actions/common"
)

// CreateResetStage creates a stage for resetting a node
func CreateResetStage() *gostage.Stage {
	stage := gostage.NewStageWithTags(
		"node-reset",
		"Node Reset",
		"Performs a complete reset of a TuringPi node",
		[]string{"node", "power", "reset"},
	)

	// Add actions in sequence
	stage.AddAction(bmc.NewGetPowerStatusAction()) // Check current status
	stage.AddAction(bmc.NewPowerOffNodeAction())   // Turn off the node
	stage.AddAction(common.NewWaitAction(5))       // Wait 5 seconds
	stage.AddAction(bmc.NewPowerOnNodeAction())    // Turn on the node
	stage.AddAction(common.NewWaitAction(10))      // Wait for boot to start
	stage.AddAction(bmc.NewGetPowerStatusAction()) // Verify node is on

	return stage
}

// CreateHardResetStage creates a stage for hard resetting a node
func CreateHardResetStage() *gostage.Stage {
	stage := gostage.NewStageWithTags(
		"node-hard-reset",
		"Node Hard Reset",
		"Performs a hard reset of a TuringPi node",
		[]string{"node", "power", "reset", "hard"},
	)

	// Add actions in sequence
	stage.AddAction(bmc.NewGetPowerStatusAction()) // Check current status
	stage.AddAction(bmc.NewResetNodeAction())      // Reset the node
	stage.AddAction(common.NewWaitAction(10))      // Wait for boot to start
	stage.AddAction(bmc.NewGetPowerStatusAction()) // Verify node is on

	return stage
}
