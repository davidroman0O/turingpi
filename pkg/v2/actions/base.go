// Package actions provides platform-aware actions for TuringPi workflows
package actions

import (
	"errors"
	"fmt"

	"github.com/davidroman0O/gostate"
	"github.com/davidroman0O/gostate/store"
	"github.com/davidroman0O/turingpi/pkg/v2/platform"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
)

// TuringPiAction is the base action type for all TuringPi-specific actions
// It implements platform awareness (Linux or Docker/non-Linux execution)
type TuringPiAction struct {
	gostate.BaseAction
	tools        tools.ToolProvider
	toolsInitErr error
}

// PlatformAction extends base Action with platform awareness
type PlatformAction interface {
	gostate.Action

	// ExecuteNative handles execution on Linux platforms
	ExecuteNative(ctx *gostate.ActionContext, tools tools.ToolProvider) error

	// ExecuteDocker handles execution through Docker on non-Linux platforms
	ExecuteDocker(ctx *gostate.ActionContext, tools tools.ToolProvider) error
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
		BaseAction: gostate.NewBaseAction(name, description),
	}
}

// Execute is the base implementation that provides tool access to derived actions
// and handles platform-specific execution paths
func (a *TuringPiAction) Execute(ctx *gostate.ActionContext) error {
	// Get the tools provider from the context
	toolsProvider, err := GetToolsFromContext(ctx)
	if err != nil {
		a.toolsInitErr = err
		return fmt.Errorf("failed to get tools provider: %w", err)
	}

	a.tools = toolsProvider

	// Execute based on platform
	if platform.IsLinux() {
		return a.ExecuteNative(ctx, toolsProvider)
	} else {
		// For non-Linux platforms, we need Docker
		containerTool := toolsProvider.GetContainerTool()
		if containerTool == nil {
			return errors.New("Docker tool is required but not available for non-Linux platform")
		}
		return a.ExecuteDocker(ctx, toolsProvider)
	}
}

// ExecuteNative is the default implementation for Linux platforms
// Should be overridden by derived actions
func (a *TuringPiAction) ExecuteNative(ctx *gostate.ActionContext, tools tools.ToolProvider) error {
	return errors.New("ExecuteNative must be implemented by derived actions")
}

// ExecuteDocker is the default implementation for non-Linux platforms (using Docker)
// Should be overridden by derived actions
func (a *TuringPiAction) ExecuteDocker(ctx *gostate.ActionContext, tools tools.ToolProvider) error {
	return errors.New("ExecuteDocker must be implemented by derived actions")
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

// GetNodeTool returns the node tool
func (a *TuringPiAction) GetNodeTool() (tools.NodeTool, error) {
	if a.toolsInitErr != nil {
		return nil, a.toolsInitErr
	}

	nodeTool := a.tools.GetNodeTool()
	if nodeTool == nil {
		return nil, errors.New("node tool is not available")
	}

	return nodeTool, nil
}

// GetImageTool returns the image tool
func (a *TuringPiAction) GetImageTool() (tools.ImageTool, error) {
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

// GetCacheTool returns the cache tool
func (a *TuringPiAction) GetCacheTool() (tools.CacheTool, error) {
	if a.toolsInitErr != nil {
		return nil, a.toolsInitErr
	}

	cacheTool := a.tools.GetCacheTool()
	if cacheTool == nil {
		return nil, errors.New("cache tool is not available")
	}

	return cacheTool, nil
}

// GetFSTool returns the filesystem tool
func (a *TuringPiAction) GetFSTool() (tools.FSTool, error) {
	if a.toolsInitErr != nil {
		return nil, a.toolsInitErr
	}

	fsTool := a.tools.GetFSTool()
	if fsTool == nil {
		return nil, errors.New("filesystem tool is not available")
	}

	return fsTool, nil
}

// StoreToolsInContext stores tools in a workflow context
func StoreToolsInContext(ctx *gostate.ActionContext, toolsProvider tools.ToolProvider) error {
	return ctx.Store.Put("$tools", toolsProvider)
}

// GetToolsFromContext retrieves tools from a workflow context
func GetToolsFromContext(ctx *gostate.ActionContext) (tools.ToolProvider, error) {
	provider, err := store.Get[tools.ToolProvider](ctx.Store, "$tools")
	if err != nil {
		return nil, fmt.Errorf("failed to get tools provider from context: %w", err)
	}

	return provider, nil
}
