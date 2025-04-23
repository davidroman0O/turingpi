// Package ubuntu provides actions for Ubuntu image preparation and deployment
package ubuntu

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/keys"
	"github.com/davidroman0O/turingpi/tools"
	"github.com/davidroman0O/turingpi/workflows/actions"
)

// UARTMonitorAction monitors the UART console output during boot
type UARTMonitorAction struct {
	actions.PlatformActionBase
}

// NewUARTMonitorAction creates a new action to monitor UART output
func NewUARTMonitorAction() *UARTMonitorAction {
	return &UARTMonitorAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"ubuntu-uart-monitor",
			"Monitors the UART console output during Ubuntu boot",
		),
	}
}

// ExecuteNative implements execution on native platforms
func (a *UARTMonitorAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// ExecuteDocker implements execution via Docker
func (a *UARTMonitorAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// executeImpl is the shared implementation
func (a *UARTMonitorAction) executeImpl(ctx *gostage.ActionContext, toolsProvider tools.ToolProvider) error {
	// Get node ID from store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return fmt.Errorf("failed to get node ID: %w", err)
	}

	// Get BMC tool
	bmcTool := toolsProvider.GetBMCTool()
	if bmcTool == nil {
		return fmt.Errorf("BMC tool not available")
	}

	// Check if we need to monitor (e.g., if flash was completed)
	flashCompleted, err := store.GetOrDefault[bool](ctx.Store(), "FlashCompleted", false)
	if err != nil {
		return fmt.Errorf("failed to check flash completion status: %w", err)
	}

	if !flashCompleted {
		ctx.Logger.Info("Flash not completed, skipping UART monitoring")
		return nil
	}

	ctx.Logger.Info("Starting UART monitoring for node %d", nodeID)

	// Create a timeout context for the boot monitoring
	monitorCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Track boot progress indicators
	var (
		systemdStarted     bool
		networkInitialized bool
		loginPromptFound   bool
		bootCompleted      bool
	)

	// Store boot status updates
	if err := ctx.Store().Put("BootStatus", "starting"); err != nil {
		return fmt.Errorf("failed to store boot status: %w", err)
	}

	// Monitor UART output with polling
	pollInterval := 5 * time.Second
	startTime := time.Now()

	for {
		select {
		case <-monitorCtx.Done():
			// Timeout reached
			ctx.Logger.Warn("Boot monitoring timed out after %v", time.Since(startTime))
			return fmt.Errorf("boot monitoring timed out")

		default:
			// Poll UART output using ExecuteCommand
			uartCmd := fmt.Sprintf("tpi uart --node %d get", nodeID)
			output, stderr, err := bmcTool.ExecuteCommand(context.Background(), uartCmd)
			if err != nil {
				ctx.Logger.Warn("Error getting UART output: %v (stderr: %s)", err, stderr)
				time.Sleep(pollInterval)
				continue
			}

			// Save the full output for debugging
			if err := ctx.Store().Put("LastUARTOutput", output); err != nil {
				ctx.Logger.Warn("Failed to store UART output: %v", err)
			}

			// Check for boot progress indicators
			if !systemdStarted && strings.Contains(output, "systemd[1]") {
				systemdStarted = true
				ctx.Logger.Info("systemd started on node %d", nodeID)
				if err := ctx.Store().Put("BootStatus", "systemd_started"); err != nil {
					ctx.Logger.Warn("Failed to store boot status: %v", err)
				}
			}

			if !networkInitialized && strings.Contains(output, "NetworkManager[") {
				networkInitialized = true
				ctx.Logger.Info("Network initialized on node %d", nodeID)
				if err := ctx.Store().Put("BootStatus", "network_initialized"); err != nil {
					ctx.Logger.Warn("Failed to store boot status: %v", err)
				}
			}

			if !loginPromptFound && (strings.Contains(output, "login:") || strings.Contains(output, "Ubuntu 20.04")) {
				loginPromptFound = true
				ctx.Logger.Info("Login prompt found on node %d", nodeID)
				if err := ctx.Store().Put("BootStatus", "login_prompt"); err != nil {
					ctx.Logger.Warn("Failed to store boot status: %v", err)
				}
			}

			// Check if boot is complete
			if systemdStarted && networkInitialized && loginPromptFound {
				bootCompleted = true
				ctx.Logger.Info("Boot completed successfully on node %d", nodeID)
				if err := ctx.Store().Put("BootStatus", "completed"); err != nil {
					ctx.Logger.Warn("Failed to store boot status: %v", err)
				}
				break
			}

			// Wait before polling again
			time.Sleep(pollInterval)
		}

		if bootCompleted {
			break
		}
	}

	ctx.Logger.Info("UART monitoring completed for node %d", nodeID)
	ctx.Logger.Info("Boot time: %v", time.Since(startTime))

	// Store boot completion in context
	if err := ctx.Store().Put("BootCompleted", true); err != nil {
		return fmt.Errorf("failed to store boot completion status: %w", err)
	}

	return nil
}
