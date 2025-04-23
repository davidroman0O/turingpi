// Package ubuntu provides stages for Ubuntu image operations
package ubuntu

import (
	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/turingpi/pkg/v2/workflows/actions/common"
	ubuntuActions "github.com/davidroman0O/turingpi/pkg/v2/workflows/actions/ubuntu"
)

// CreateImageDeploymentStage creates a stage for deploying an Ubuntu image
func CreateImageDeploymentStage() *gostage.Stage {
	stage := gostage.NewStageWithTags(
		"ubuntu-image-deployment",
		"Ubuntu Image Deployment",
		"Deploys an Ubuntu image to a node and monitors boot process",
		[]string{"ubuntu", "image", "deploy", "flash"},
	)

	// Add actions in sequence
	stage.AddAction(ubuntuActions.NewImageUploadAction()) // First upload the image to BMC
	stage.AddAction(ubuntuActions.NewImageFlashAction())  // Then flash it to the node
	stage.AddAction(common.NewWaitAction(10))             // Wait for flash to complete and node to start booting
	stage.AddAction(ubuntuActions.NewUARTMonitorAction())

	return stage
}
