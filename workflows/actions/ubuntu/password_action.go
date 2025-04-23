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

// PasswordChangeAction changes the default Ubuntu user password
type PasswordChangeAction struct {
	actions.PlatformActionBase
}

// NewPasswordChangeAction creates a new action to change the default password
func NewPasswordChangeAction() *PasswordChangeAction {
	return &PasswordChangeAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"ubuntu-password-change",
			"Changes the default Ubuntu user password",
		),
	}
}

// ExecuteNative implements execution on native platforms
func (a *PasswordChangeAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// ExecuteDocker implements execution via Docker
func (a *PasswordChangeAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// executeImpl is the shared implementation
func (a *PasswordChangeAction) executeImpl(ctx *gostage.ActionContext, toolsProvider tools.ToolProvider) error {
	// Get required parameters from the store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return fmt.Errorf("failed to get node ID: %w", err)
	}

	// Get the node IP address - required for SSH
	ipCIDR, err := store.Get[string](ctx.Store(), "IPCIDR")
	if err != nil {
		return fmt.Errorf("failed to get node IP address: %w", err)
	}

	// Strip CIDR notation if present
	ipAddress := ipCIDR
	if idx := strings.Index(ipCIDR, "/"); idx != -1 {
		ipAddress = ipCIDR[:idx]
	}

	// Get the new password from the store
	newPassword, err := store.GetOrDefault[string](ctx.Store(), "NewPassword", "turingpi123!")
	if err != nil {
		return fmt.Errorf("failed to get new password: %w", err)
	}

	// Default Ubuntu username
	username := "ubuntu"

	// Get BMC tool for node interactions
	bmcTool := toolsProvider.GetBMCTool()
	if bmcTool == nil {
		return fmt.Errorf("BMC tool not available")
	}

	ctx.Logger.Info("Starting password change for user '%s' on node %d (%s)", username, nodeID, ipAddress)

	// Check if boot has completed (UART monitor should have set this)
	bootCompleted, err := store.GetOrDefault[bool](ctx.Store(), "BootCompleted", false)
	if err != nil {
		ctx.Logger.Warn("Failed to check boot status: %v", err)
		// Continue anyway as we're explicitly waiting before this action
	}

	if !bootCompleted {
		ctx.Logger.Warn("Boot completion not confirmed by UART monitor")
		// Continue anyway - the explicit wait action before this should cover it
	}

	// Direct SSH command to change password
	// This is simpler than expect scripts and aligns with your codebase style
	ctx.Logger.Info("Executing password change via SSH")

	// Try a few times in case the node is still starting up
	var success bool
	var lastOutput, lastError string

	// Maximum number of retries
	maxRetries := 3
	timeout := 10 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		ctx.Logger.Info("Password change attempt %d/%d", attempt, maxRetries)

		// Create the SSH command
		sshCmd := fmt.Sprintf(
			"sshpass -p 'ubuntu' ssh -o StrictHostKeyChecking=no -o ConnectTimeout=10 %s@%s 'echo -e \"ubuntu\\n%s\\n%s\" | sudo passwd %s'",
			username, ipAddress, newPassword, newPassword, username,
		)

		// Execute the command via BMC
		ctx.Logger.Debug("Running command: %s", sshCmd)
		stdout, stderr, err := bmcTool.ExecuteCommand(context.Background(), sshCmd)

		if err != nil {
			lastError = err.Error()
			lastOutput = stdout + "\n" + stderr
			ctx.Logger.Warn("Password change attempt %d failed: %v", attempt, err)
			ctx.Logger.Debug("Command output: %s", lastOutput)
			time.Sleep(timeout) // Wait before retry
			continue
		}

		// Check for success indicators in output
		if strings.Contains(stdout, "passwd: password updated successfully") {
			ctx.Logger.Info("Password successfully changed for user '%s'", username)
			success = true
			break
		} else {
			lastOutput = stdout + "\n" + stderr
			ctx.Logger.Warn("Password change attempt %d did not report success", attempt)
			ctx.Logger.Debug("Command output: %s", lastOutput)
			time.Sleep(timeout) // Wait before retry
		}
	}

	if !success {
		ctx.Logger.Error("All password change attempts failed")
		ctx.Logger.Debug("Last output: %s", lastOutput)
		if lastError != "" {
			ctx.Logger.Debug("Last error: %s", lastError)
		}
		return fmt.Errorf("failed to change password after multiple attempts")
	}

	// Store success in context
	if err := ctx.Store().Put("PasswordChanged", true); err != nil {
		return fmt.Errorf("failed to store password change status: %w", err)
	}

	return nil
}
