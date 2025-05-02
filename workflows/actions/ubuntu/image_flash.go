// Package ubuntu provides actions for Ubuntu image preparation and deployment
package ubuntu

import (
	"context"
	"fmt"
	"time"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/bmc"
	"github.com/davidroman0O/turingpi/keys"
	"github.com/davidroman0O/turingpi/tools"
	"github.com/davidroman0O/turingpi/workflows/actions"
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

	// Get the configured IP address for debugging
	ipCIDR, err := store.GetOrDefault[string](ctx.Store(), "IPCIDR", "")
	if err == nil && ipCIDR != "" {
		ctx.Logger.Info("Configured IP CIDR for this node: %s", ipCIDR)
	} else {
		ctx.Logger.Warn("No IPCIDR found in store!")
	}

	// Explicitly log all relevant store keys
	ctx.Logger.Info("Critical store values for network configuration:")
	if val, err := store.GetOrDefault[string](ctx.Store(), "IPCIDR", ""); err == nil {
		ctx.Logger.Info("  IPCIDR: %s", val)
	}
	if val, err := store.GetOrDefault[string](ctx.Store(), "Hostname", ""); err == nil {
		ctx.Logger.Info("  Hostname: %s", val)
	}
	if val, err := store.GetOrDefault[string](ctx.Store(), "Gateway", ""); err == nil {
		ctx.Logger.Info("  Gateway: %s", val)
	}

	// Create a context with timeout to prevent hanging
	flashCtx, cancel := context.WithTimeout(ctx.GoContext, 3*time.Minute)
	defer cancel()

	// Execute the flash command directly with timeout
	ctx.Logger.Info("Executing direct flash command with 3-minute timeout")
	flashCmd := fmt.Sprintf("flash_node %d %s", nodeID, remoteImagePath)

	// Start a separate goroutine to show progress during the flash operation
	progressDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ctx.Logger.Info("Flash operation still in progress... (waiting)")
			case <-progressDone:
				return
			}
		}
	}()

	// Execute the command
	stdout, stderr, err := bmcTool.ExecuteCommand(flashCtx, flashCmd)
	close(progressDone) // Signal the progress goroutine to stop

	if err != nil {
		if flashCtx.Err() == context.DeadlineExceeded {
			ctx.Logger.Error("Flash operation timed out after 3 minutes")
			return fmt.Errorf("flash operation timed out after 3 minutes")
		}
		ctx.Logger.Error("Flash operation failed: %v", err)
		ctx.Logger.Error("Stderr: %s", stderr)
		return fmt.Errorf("failed to flash node: %w", err)
	}

	ctx.Logger.Info("Flash command output: %s", stdout)
	ctx.Logger.Info("Image flashed successfully to node %d", nodeID)

	// Set node mode to normal using ExecuteCommand
	ctx.Logger.Info("Setting node %d mode to normal", nodeID)

	err = bmcTool.SetNodeMode(context.Background(), nodeID, bmc.NodeModeNormal)
	if err != nil {
		return fmt.Errorf("failed to set node mode to normal: %w ", err)
	}

	err = bmcTool.PowerOn(ctx.GoContext, 1)
	if err != nil {
		return fmt.Errorf("failed to power on node: %w", err)
	}

	ctx.Logger.Info("Node mode set to normal.")

	// Store flash completion in context
	if err := ctx.Store().Put("FlashCompleted", true); err != nil {
		return fmt.Errorf("failed to store flash completion status: %w", err)
	}

	return nil
}
