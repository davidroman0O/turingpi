// Package actions provides platform-aware actions for TuringPi workflows
package actions

import (
	"errors"
	"fmt"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/pkg/v2/cache"
	"github.com/davidroman0O/turingpi/pkg/v2/keys"
	"github.com/davidroman0O/turingpi/pkg/v2/platform"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
)

// TuringPiAction is the base action type for all TuringPi-specific actions
// It implements platform awareness (Linux or Docker/non-Linux execution)
type TuringPiAction struct {
	gostage.BaseAction
	tools        tools.ToolProvider
	toolsInitErr error
}

// PlatformAction extends base Action with platform awareness
type PlatformAction interface {
	gostage.Action

	// ExecuteNative handles execution on Linux platforms
	ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error

	// ExecuteDocker handles execution through Docker on non-Linux platforms
	ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error
}

// PlatformActionBase provides a base implementation for platform-aware actions
type PlatformActionBase struct {
	TuringPiAction
}

// NewPlatformActionBase creates a new PlatformActionBase
func NewPlatformActionBase(name, description string) PlatformActionBase {
	return PlatformActionBase{
		TuringPiAction: NewTuringPiAction(name, description),
	}
}

// NewTuringPiAction creates a new TuringPiAction
func NewTuringPiAction(name, description string) TuringPiAction {
	return TuringPiAction{
		BaseAction:   gostage.NewBaseAction(name, description),
		tools:        nil,
		toolsInitErr: nil,
	}
}

// Execute is the base implementation that provides tool access to derived actions
// and handles platform-specific execution paths
func (a *TuringPiAction) Execute(ctx *gostage.ActionContext) error {
	// Get tools provider from context
	provider, err := GetToolsFromContext(ctx)
	if err != nil {
		a.toolsInitErr = err
		return err
	}
	a.tools = provider

	// For ExecuteNative and ExecuteDocker, we need to type assert to the actual
	// implementing type, not the wrapper type like *PlatformActionBase

	// Try to reflect the receiver to get the real type
	realAction := ctx.Action
	ctx.Logger.Debug("Action is of type: %T", realAction)

	// Check if the action is a platform action
	platformAction, ok := realAction.(PlatformAction)
	if !ok {
		ctx.Logger.Debug("Action does not implement PlatformAction: %T", realAction)
		return errors.New("action does not implement PlatformAction")
	}

	// Check platform type to determine execution path
	if platform.IsLinux() {
		// Direct execution on Linux
		ctx.Logger.Debug("Using native execution path (Linux)")
		return platformAction.ExecuteNative(ctx, provider)
	} else if platform.DockerAvailable() {
		// Docker-based execution on non-Linux if Docker is available
		ctx.Logger.Debug("Using Docker execution path")
		return platformAction.ExecuteDocker(ctx, provider)
	}

	// If we get here, we can't execute the action
	return errors.New("unsupported platform: requires Linux or Docker")
}

// ExecuteNative is the default implementation for Linux platforms
// Should be overridden by derived actions
func (a *TuringPiAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	ctx.Logger.Info("Base TuringPiAction.ExecuteNative called - this is expected to be overridden by derived actions")

	// Before returning the error, let's log what we know to help debug
	ctx.Logger.Debug("Action type: %T", a)
	ctx.Logger.Debug("Tools type: %T", tools)

	if bmcTool := tools.GetBMCTool(); bmcTool != nil {
		ctx.Logger.Debug("BMC tool is available")
	} else {
		ctx.Logger.Debug("BMC tool is not available")
	}

	return errors.New("not implemented - derived action should override ExecuteNative")
}

// ExecuteDocker is the default implementation for non-Linux platforms (using Docker)
// Should be overridden by derived actions
func (a *TuringPiAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	ctx.Logger.Info("Base TuringPiAction.ExecuteDocker called - this is expected to be overridden by derived actions")

	// Before returning the error, let's log what we know to help debug
	ctx.Logger.Debug("Action type: %T", a)
	ctx.Logger.Debug("Tools type: %T", tools)

	if bmcTool := tools.GetBMCTool(); bmcTool != nil {
		ctx.Logger.Debug("BMC tool is available")
	} else {
		ctx.Logger.Debug("BMC tool is not available")
	}

	return errors.New("not implemented - derived action should override ExecuteDocker")
}

// GetBMCTool returns the BMC tool
func (a *TuringPiAction) GetBMCTool() (tools.BMCTool, error) {
	if a.toolsInitErr != nil {
		return nil, a.toolsInitErr
	}

	bmcTool := a.tools.GetBMCTool()
	if bmcTool == nil {
		return nil, errors.New("BMC tool is not available")
	}

	return bmcTool, nil
}

// GetImageTool returns the image tool
func (a *TuringPiAction) GetImageTool() (tools.OperationsTool, error) {
	if a.toolsInitErr != nil {
		return nil, a.toolsInitErr
	}

	imageTool := a.tools.GetImageTool()
	if imageTool == nil {
		return nil, errors.New("image tool is not available")
	}

	return imageTool, nil
}

// GetContainerTool returns the container tool
func (a *TuringPiAction) GetContainerTool() (tools.ContainerTool, error) {
	if a.toolsInitErr != nil {
		return nil, a.toolsInitErr
	}

	containerTool := a.tools.GetContainerTool()
	if containerTool == nil {
		return nil, errors.New("container tool is not available")
	}

	return containerTool, nil
}

// GetLocalCache returns the local filesystem cache
func (a *TuringPiAction) GetLocalCache() (*cache.FSCache, error) {
	if a.toolsInitErr != nil {
		return nil, a.toolsInitErr
	}

	localCache := a.tools.GetLocalCache()
	if localCache == nil {
		return nil, errors.New("local cache is not available")
	}

	return localCache, nil
}

// GetRemoteCache returns the remote SSH cache
func (a *TuringPiAction) GetRemoteCache() (*cache.SSHCache, error) {
	if a.toolsInitErr != nil {
		return nil, a.toolsInitErr
	}

	remoteCache := a.tools.GetRemoteCache()
	if remoteCache == nil {
		return nil, errors.New("remote cache is not available")
	}

	return remoteCache, nil
}

// StoreToolsInContext stores tools in a workflow context
func StoreToolsInContext(ctx *gostage.ActionContext, toolsProvider tools.ToolProvider) error {
	return ctx.Store().Put(keys.ToolsProvider, toolsProvider)
}

// GetToolsFromContext retrieves tools from a workflow context
func GetToolsFromContext(ctx *gostage.ActionContext) (tools.ToolProvider, error) {
	// Debug logging to see if the store exists
	ctx.Logger.Debug("Checking for tools in context store")

	// Get the provider from the store as a concrete type (which we know it is)
	concreteProvider, err := store.Get[*tools.TuringPiToolProvider](ctx.Store(), keys.ToolsProvider)
	if err != nil {
		ctx.Logger.Debug("Failed to get tools provider: %v", err)
		return nil, fmt.Errorf("failed to get tools provider from context: %w", err)
	}

	// Return the concrete provider - it also implements the interface
	return concreteProvider, nil
}

// Execute implements the Action interface for PlatformActionBase
func (a *PlatformActionBase) Execute(ctx *gostage.ActionContext) error {
	// This delegates to the base TuringPiAction's Execute method which handles platform detection
	return a.TuringPiAction.Execute(ctx)
}
