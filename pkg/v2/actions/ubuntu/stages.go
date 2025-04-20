// Package ubuntu provides Ubuntu-specific actions for TuringPi
package ubuntu

import (
	"fmt"

	"github.com/davidroman0O/gostate"
)

// Stage identifiers for Ubuntu deployment
const (
	StageIDImagePreparation = "ubuntu-image-preparation"
	StageIDOSInstallation   = "ubuntu-os-installation"
	StageIDPostInstallation = "ubuntu-post-installation"
)

// CreateUbuntuDeploymentWorkflow creates a complete workflow for deploying Ubuntu to a node
func CreateUbuntuDeploymentWorkflow(nodeID int, osVersion string) (*gostate.Workflow, error) {
	// Create the main workflow
	wf := gostate.NewWorkflow(
		fmt.Sprintf("ubuntu-deployment-node-%d", nodeID),
		fmt.Sprintf("Ubuntu %s Deployment for Node %d", osVersion, nodeID),
		fmt.Sprintf("Complete workflow for deploying Ubuntu %s to Node %d", osVersion, nodeID),
	)

	// Add workflow-level initial data
	wf.Store.Put("nodeID", nodeID)
	wf.Store.Put("osType", "ubuntu")
	wf.Store.Put("osVersion", osVersion)

	// Stage 1: Image Preparation (corresponds to UbuntuImageBuilder)
	// This stage prepares the OS image with customizations like network config
	imagePrepStage := buildImagePreparationStage(nodeID, osVersion)
	wf.AddStage(imagePrepStage)

	// Stage 2: OS Installation (corresponds to UbuntuOSInstallerBuilder)
	// This stage installs the prepared image onto the node
	osInstallStage := buildOSInstallationStage(nodeID)
	wf.AddStage(osInstallStage)

	// Stage 3: Post-Installation (corresponds to UbuntuPostInstallerBuilder)
	// This stage handles setup after the OS is installed
	postInstallStage := buildPostInstallationStage(nodeID)
	wf.AddStage(postInstallStage)

	return wf, nil
}

// buildImagePreparationStage creates the stage for image preparation
func buildImagePreparationStage(nodeID int, osVersion string) *gostate.Stage {
	stage := gostate.NewStage(
		StageIDImagePreparation,
		fmt.Sprintf("Ubuntu %s Image Preparation", osVersion),
		"Prepare and customize Ubuntu OS image",
	)

	// Add stage-specific initial data
	stage.InitialStore.Put("imageFormat", "xz")
	stage.InitialStore.Put("mountPoint", "/mnt/image")

	// TODO: Add actions to the stage once they're implemented
	// stage.AddAction(NewCheckBaseImageAction(osVersion))
	// stage.AddAction(NewDecompressImageAction())
	// stage.AddAction(NewConfigureNetworkAction(nodeID))
	// stage.AddAction(NewCompressImageAction())

	return stage
}

// buildOSInstallationStage creates the stage for OS installation
func buildOSInstallationStage(nodeID int) *gostate.Stage {
	stage := gostate.NewStage(
		StageIDOSInstallation,
		"Ubuntu OS Installation",
		"Install Ubuntu OS onto the target node",
	)

	// TODO: Add actions to the stage once they're implemented
	// stage.AddAction(NewPrepareNodeAction(nodeID))
	// stage.AddAction(NewTransferImageAction(nodeID))
	// stage.AddAction(NewFlashImageAction(nodeID))
	// stage.AddAction(NewVerifyInstallAction(nodeID))

	return stage
}

// buildPostInstallationStage creates the stage for post-installation setup
func buildPostInstallationStage(nodeID int) *gostate.Stage {
	stage := gostate.NewStage(
		StageIDPostInstallation,
		"Ubuntu Post-Installation Setup",
		"Configure the newly installed Ubuntu OS",
	)

	// TODO: Add actions to the stage once they're implemented
	// stage.AddAction(NewSetupUsersAction(nodeID))
	// stage.AddAction(NewConfigureSystemAction(nodeID))
	// stage.AddAction(NewInstallPackagesAction(nodeID))
	// stage.AddAction(NewSetupServicesAction(nodeID))

	return stage
}
