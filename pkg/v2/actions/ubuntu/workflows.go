// Package ubuntu provides Ubuntu-specific actions for TuringPi
package ubuntu

import (
	"fmt"

	"github.com/davidroman0O/gostate"
)

// Hook points for custom actions in workflows
const (
	// HookBeforeUnmount is the hook point for adding actions before unmounting the prepared image
	HookBeforeUnmount = "before-unmount"

	// HookAfterPostInstall is the hook point for adding actions after post-installation
	HookAfterPostInstall = "after-post-install"
)

// WorkflowOptions contains configuration options for Ubuntu deployment workflows
type WorkflowOptions struct {
	NodeID            int                         // ID of the node to install to
	OSVersion         string                      // Ubuntu version to install
	NodePassword      string                      // Password to set for the node's root account
	NetworkConfig     *NetworkConfig              // Network configuration for the node
	CustomActions     map[string][]gostate.Action // Custom actions to add at different hook points
	CustomStoreValues map[string]interface{}      // Custom values to add to the workflow store
}

// DefaultWorkflowOptions returns default options for workflow creation
func DefaultWorkflowOptions(nodeID int, osVersion string) *WorkflowOptions {
	return &WorkflowOptions{
		NodeID:        nodeID,
		OSVersion:     osVersion,
		NodePassword:  "turingpi", // Default password
		NetworkConfig: nil,        // Will be configured based on node ID if not provided
		CustomActions: map[string][]gostate.Action{
			HookBeforeUnmount:    {},
			HookAfterPostInstall: {},
		},
		CustomStoreValues: map[string]interface{}{},
	}
}

// CreateRK1DeploymentWorkflow creates a workflow for deploying Ubuntu to a Rockchip RK1 node
func CreateRK1DeploymentWorkflow(options *WorkflowOptions) (*gostate.Workflow, error) {
	if options == nil {
		return nil, fmt.Errorf("workflow options cannot be nil")
	}

	// Create the main workflow
	wf := gostate.NewWorkflow(
		fmt.Sprintf("rk1-ubuntu-deployment-node-%d", options.NodeID),
		fmt.Sprintf("Ubuntu %s Deployment for RK1 Node %d", options.OSVersion, options.NodeID),
		fmt.Sprintf("Complete workflow for deploying Ubuntu %s to RK1 Node %d", options.OSVersion, options.NodeID),
	)

	// Add workflow-level initial data
	wf.Store.Put("nodeID", options.NodeID)
	wf.Store.Put("osType", "ubuntu")
	wf.Store.Put("osVersion", options.OSVersion)
	wf.Store.Put("boardType", "rk1")
	wf.Store.Put("nodePassword", options.NodePassword)

	// Add custom store values
	for key, value := range options.CustomStoreValues {
		wf.Store.Put(key, value)
	}

	// If network config is provided, store it
	if options.NetworkConfig != nil {
		wf.Store.Put("networkConfig", *options.NetworkConfig)
	}

	// Stage 1: Image Preparation (includes customization and network configuration)
	imagePrepStage := buildRK1ImagePreparationStage(options)
	wf.AddStage(imagePrepStage)

	// Stage 2: OS Installation (flashing to the node)
	osInstallStage := buildRK1OSInstallationStage(options)
	wf.AddStage(osInstallStage)

	// Stage 3: Post-Installation (user setup, password, additional packages)
	postInstallStage := buildRK1PostInstallationStage(options)
	wf.AddStage(postInstallStage)

	return wf, nil
}

// buildRK1ImagePreparationStage creates the image preparation stage for RK1 boards
func buildRK1ImagePreparationStage(options *WorkflowOptions) *gostate.Stage {
	stage := gostate.NewStage(
		StageIDImagePreparation,
		fmt.Sprintf("Ubuntu %s Image Preparation for RK1", options.OSVersion),
		"Prepare and customize Ubuntu OS image for Rockchip RK1 boards",
	)

	// Add stage-specific initial data
	stage.InitialStore.Put("imageFormat", "xz")
	stage.InitialStore.Put("mountPoint", "/mnt/image")
	stage.InitialStore.Put("boardType", "rk1")

	// Core actions for image preparation
	stage.AddAction(NewCheckBaseImageAction(options.OSVersion))
	stage.AddAction(NewDecompressImageAction())

	// Configure network if needed
	stage.AddAction(NewConfigureNetworkAction(options.NodeID))

	// Add custom actions before unmounting
	for _, action := range options.CustomActions[HookBeforeUnmount] {
		stage.AddAction(action)
	}

	// Always compress the image as the final step
	stage.AddAction(NewCompressImageAction())

	return stage
}

// buildRK1OSInstallationStage creates the OS installation stage for RK1 boards
func buildRK1OSInstallationStage(options *WorkflowOptions) *gostate.Stage {
	stage := gostate.NewStage(
		StageIDOSInstallation,
		"Ubuntu OS Installation for RK1",
		"Install Ubuntu OS onto the target RK1 node",
	)

	// TODO: Implement RK1-specific installation actions
	// stage.AddAction(NewPrepareRK1NodeAction(options.NodeID))
	// stage.AddAction(NewTransferImageAction(options.NodeID))
	// stage.AddAction(NewFlashRK1ImageAction(options.NodeID))
	// stage.AddAction(NewVerifyRK1InstallAction(options.NodeID))

	return stage
}

// buildRK1PostInstallationStage creates the post-installation stage for RK1 boards
func buildRK1PostInstallationStage(options *WorkflowOptions) *gostate.Stage {
	stage := gostate.NewStage(
		StageIDPostInstallation,
		"Ubuntu Post-Installation Setup for RK1",
		"Configure the newly installed Ubuntu OS on RK1 node",
	)

	// Core post-installation actions for RK1
	// Add password configuration action
	stage.AddAction(NewSetRK1PasswordAction())

	// TODO: Add more RK1-specific setup actions
	// stage.AddAction(NewSetupRK1SystemAction())

	// Add custom actions after post-installation
	for _, action := range options.CustomActions[HookAfterPostInstall] {
		stage.AddAction(action)
	}

	return stage
}

// AddActionBeforeUnmount adds a custom action to run before unmounting the image
func (opts *WorkflowOptions) AddActionBeforeUnmount(action gostate.Action) {
	opts.CustomActions[HookBeforeUnmount] = append(opts.CustomActions[HookBeforeUnmount], action)
}

// AddActionAfterPostInstall adds a custom action to run after post-installation
func (opts *WorkflowOptions) AddActionAfterPostInstall(action gostate.Action) {
	opts.CustomActions[HookAfterPostInstall] = append(opts.CustomActions[HookAfterPostInstall], action)
}

// SetNetworkConfig sets the network configuration for the node
func (opts *WorkflowOptions) SetNetworkConfig(config NetworkConfig) {
	opts.NetworkConfig = &config
}

// SetNodePassword sets the password for the node
func (opts *WorkflowOptions) SetNodePassword(password string) {
	opts.NodePassword = password
}

// AddStoreValue adds a custom value to the workflow store
func (opts *WorkflowOptions) AddStoreValue(key string, value interface{}) {
	opts.CustomStoreValues[key] = value
}
