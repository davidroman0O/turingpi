// Package ubuntu provides stages for Ubuntu image operations
package ubuntu

import (
	"github.com/davidroman0O/gostage"
	ubuntuActions "github.com/davidroman0O/turingpi/pkg/v2/workflows/actions/ubuntu"
)

// CreateImagePreparationStage creates a stage for preparing an Ubuntu image
func CreateImagePreparationStage() *gostage.Stage {
	stage := gostage.NewStageWithTags(
		"ubuntu-image-preparation",
		"Ubuntu Image Preparation",
		"Prepares an Ubuntu image with network configuration and customizations",
		[]string{"ubuntu", "image", "prepare"},
	)

	// Add actions in sequence
	stage.AddAction(ubuntuActions.NewImagePrepareAction())
	stage.AddAction(ubuntuActions.NewImageFinalizeAction())
	// stage.AddAction(ubuntuActions.NewImageUploadAction())

	return stage
}
