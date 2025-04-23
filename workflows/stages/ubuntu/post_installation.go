// Package ubuntu provides stages for Ubuntu image operations
package ubuntu

import (
	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/turingpi/workflows/actions/common"
	ubuntuActions "github.com/davidroman0O/turingpi/workflows/actions/ubuntu"
)

// CreatePostInstallationStage creates a stage for post-installation tasks
func CreatePostInstallationStage() *gostage.Stage {
	stage := gostage.NewStageWithTags(
		"ubuntu-post-installation",
		"Ubuntu Post-Installation",
		"Configures the Ubuntu system after deployment",
		[]string{"ubuntu", "post-install", "config"},
	)

	// Add actions in sequence
	stage.AddAction(common.NewWaitAction(30)) // Wait for system to fully boot
	stage.AddAction(ubuntuActions.NewPasswordChangeAction())

	return stage
}
