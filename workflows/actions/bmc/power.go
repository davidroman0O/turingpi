// Package bmc provides actions for interacting with the Turing Pi BMC
package bmc

import (
	"context"
	"fmt"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/bmc"
	"github.com/davidroman0O/turingpi/keys"
	"github.com/davidroman0O/turingpi/tools"
	"github.com/davidroman0O/turingpi/workflows/actions"
)

// PowerOnNodeAction turns on a node
type PowerOnNodeAction struct {
	actions.PlatformActionBase
}

// NewPowerOnNodeAction creates a new action to power on a node
func NewPowerOnNodeAction() *PowerOnNodeAction {
	return &PowerOnNodeAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"power-on-node",
			"Powers on the current target node",
		),
	}
}

// ExecuteNative implements execution on native platforms
func (a *PowerOnNodeAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// ExecuteDocker implements execution via Docker
func (a *PowerOnNodeAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// executeImpl is the shared implementation
func (a *PowerOnNodeAction) executeImpl(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	// Get current node ID from store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return err
	}

	// Add additional debug logging
	ctx.Logger.Debug("Looking for BMC tool from provider type: %T", tools)

	bmcTool := tools.GetBMCTool()
	if bmcTool == nil {
		ctx.Logger.Info("BMC tool not available")
		ctx.Logger.Info("Skipping power on for node %d", nodeID)
		return nil
	}

	ctx.Logger.Debug("Found BMC tool of type: %T", bmcTool)
	ctx.Logger.Info("Powering on node %d", nodeID)
	if err := bmcTool.PowerOn(context.Background(), nodeID); err != nil {
		return err
	}

	ctx.Logger.Info("Node %d power on command sent successfully", nodeID)
	return nil
}

// PowerOffNodeAction turns off a node
type PowerOffNodeAction struct {
	actions.PlatformActionBase
}

// NewPowerOffNodeAction creates a new action to power off a node
func NewPowerOffNodeAction() *PowerOffNodeAction {
	return &PowerOffNodeAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"power-off-node",
			"Powers off the current target node",
		),
	}
}

// ExecuteNative implements execution on native platforms
func (a *PowerOffNodeAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// ExecuteDocker implements execution via Docker
func (a *PowerOffNodeAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// executeImpl is the shared implementation
func (a *PowerOffNodeAction) executeImpl(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	// Get current node ID from store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return err
	}

	bmcTool := tools.GetBMCTool()
	if bmcTool == nil {
		ctx.Logger.Info("BMC tool not available")
		ctx.Logger.Info("Skipping power off for node %d", nodeID)
		return nil
	}

	ctx.Logger.Info("Powering off node %d", nodeID)
	if err := bmcTool.PowerOff(context.Background(), nodeID); err != nil {
		return err
	}

	ctx.Logger.Info("Node %d power off command sent successfully", nodeID)
	return nil
}

// ResetNodeAction performs a hard reset on a node
type ResetNodeAction struct {
	actions.PlatformActionBase
}

// NewResetNodeAction creates a new action to reset a node
func NewResetNodeAction() *ResetNodeAction {
	return &ResetNodeAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"reset-node",
			"Resets the current target node",
		),
	}
}

// ExecuteNative implements execution on native platforms
func (a *ResetNodeAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// ExecuteDocker implements execution via Docker
func (a *ResetNodeAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// executeImpl is the shared implementation
func (a *ResetNodeAction) executeImpl(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	// Get current node ID from store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return err
	}

	bmcTool := tools.GetBMCTool()
	if bmcTool == nil {
		ctx.Logger.Info("BMC tool not available")
		ctx.Logger.Info("Skipping reset for node %d", nodeID)
		return nil
	}

	ctx.Logger.Info("Resetting node %d", nodeID)
	if err := bmcTool.Reset(context.Background(), nodeID); err != nil {
		return err
	}

	ctx.Logger.Info("Node %d reset command sent successfully", nodeID)
	return nil
}

// GetPowerStatusAction gets the power status of a node
type GetPowerStatusAction struct {
	actions.PlatformActionBase
}

// NewGetPowerStatusAction creates a new action to get power status
func NewGetPowerStatusAction() *GetPowerStatusAction {
	return &GetPowerStatusAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"get-power-status",
			"Gets the power status of the current target node",
		),
	}
}

// ExecuteNative implements execution on native platforms
func (a *GetPowerStatusAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// ExecuteDocker implements execution via Docker
func (a *GetPowerStatusAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// executeImpl is the shared implementation
func (a *GetPowerStatusAction) executeImpl(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	// Get current node ID from store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return err
	}

	fmt.Println("keys", ctx.Store().ListKeys())

	fmt.Println("tools", tools)
	t, e := actions.GetToolsFromContext(ctx)
	fmt.Println("t", t)
	fmt.Println("e", e)
	fmt.Println("bmcTool tools", tools.GetBMCTool())
	fmt.Println("bmcTool t", t.GetBMCTool())

	bmcTool := tools.GetBMCTool()
	if bmcTool == nil {
		ctx.Logger.Info("BMC tool not available")
		ctx.Logger.Info("Skipping power status check for node %d", nodeID)
		// Provide a fake status for demonstration purposes
		status := &bmc.PowerStatus{
			NodeID: nodeID,
			State:  bmc.PowerStateOn,
		}

		// Update node status in store
		statusKey := keys.FormatKey(keys.NodeStatus, nodeID)
		if err := ctx.Store().Put(statusKey, status); err != nil {
			return err
		}

		// Also update the simple power state
		powerKey := keys.FormatKey(keys.NodePower, nodeID)
		return ctx.Store().Put(powerKey, string(status.State))
	}

	ctx.Logger.Info("Getting power status for node %d", nodeID)
	status, err := bmcTool.GetPowerStatus(context.Background(), nodeID)
	if err != nil {
		return err
	}

	// Update node status in store
	statusKey := keys.FormatKey(keys.NodeStatus, nodeID)
	if err := ctx.Store().Put(statusKey, status); err != nil {
		return err
	}

	// Also update the simple power state
	powerKey := keys.FormatKey(keys.NodePower, nodeID)
	return ctx.Store().Put(powerKey, string(status.State))
}
