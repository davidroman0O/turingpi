// Package ubuntu provides actions for Ubuntu image preparation and deployment
package ubuntu

import (
	"context"
	"fmt"
	"time"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/pkg/v2/bmc"
	"github.com/davidroman0O/turingpi/pkg/v2/keys"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
	"github.com/davidroman0O/turingpi/pkg/v2/workflows/actions"
)

// ImageFlashAction flashes the image to the node using the BMC
type ImageFlashAction struct {
	actions.PlatformActionBase
}

// NewImageFlashAction creates a new action to flash the image to the node
func NewImageFlashAction() *ImageFlashAction {
	return &ImageFlashAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"ubuntu-image-flash",
			"Flashes the Ubuntu image to the node using the BMC",
		),
	}
}

// ExecuteNative implements execution on native platforms
func (a *ImageFlashAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// ExecuteDocker implements execution via Docker
func (a *ImageFlashAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// executeImpl is the shared implementation
func (a *ImageFlashAction) executeImpl(ctx *gostage.ActionContext, toolsProvider tools.ToolProvider) error {
	// Get node ID from store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return fmt.Errorf("failed to get node ID: %w", err)
	}

	// Get the remote image path
	remoteImagePath, err := store.Get[string](ctx.Store(), "RemoteImagePath")
	if err != nil || remoteImagePath == "" {
		return fmt.Errorf("remote image path not found or empty: %w", err)
	}

	// Get BMC tool
	bmcTool := toolsProvider.GetBMCTool()
	if bmcTool == nil {
		return fmt.Errorf("BMC tool not available")
	}

	// Check node power status
	ctx.Logger.Info("Checking power status for node %d", nodeID)
	powerStatus, err := bmcTool.GetPowerStatus(context.Background(), nodeID)
	if err != nil {
		return fmt.Errorf("failed to get power status: %w", err)
	}

	// Power off the node if it's on
	if powerStatus.State == bmc.PowerStateOn {
		ctx.Logger.Info("Node %d is on, powering off before flashing", nodeID)
		if err := bmcTool.PowerOff(context.Background(), nodeID); err != nil {
			return fmt.Errorf("failed to power off node: %w", err)
		}

		// Wait for node to power off
		ctx.Logger.Info("Waiting for node %d to power off...", nodeID)
		time.Sleep(5 * time.Second)

		// Verify node is off
		powerStatus, err = bmcTool.GetPowerStatus(context.Background(), nodeID)
		if err != nil {
			return fmt.Errorf("failed to get power status after power off: %w", err)
		}

		if powerStatus.State != bmc.PowerStateOff {
			return fmt.Errorf("node %d failed to power off", nodeID)
		}
	} else {
		ctx.Logger.Info("Node %d is already off, proceeding with flash", nodeID)
	}

	// Flash the image using ExecuteCommand
	ctx.Logger.Info("Flashing image to node %d", nodeID)
	ctx.Logger.Info("Image path: %s", remoteImagePath)

	// Use the proper BMC interface method instead of directly executing the command
	err = bmcTool.FlashNode(context.Background(), nodeID, remoteImagePath)
	if err != nil {
		return fmt.Errorf("failed to flash node: %w", err)
	}

	ctx.Logger.Info("Image flashed successfully to node %d", nodeID)

	// Set node mode to normal using ExecuteCommand
	ctx.Logger.Info("Setting node %d mode to normal", nodeID)

	err = bmcTool.SetNodeMode(context.Background(), nodeID, bmc.NodeModeNormal)
	if err != nil {
		return fmt.Errorf("failed to set node mode to normal: %w ", err)
	}

	ctx.Logger.Info("Node mode set to normal.")

	// Store flash completion in context
	if err := ctx.Store().Put("FlashCompleted", true); err != nil {
		return fmt.Errorf("failed to store flash completion status: %w", err)
	}

	return nil
}
