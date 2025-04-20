package actions

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/davidroman0O/gostate"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
)

// SamplePlatformAction demonstrates a platform-aware action implementation
type SamplePlatformAction struct {
	TuringPiAction
	// Action-specific fields go here
	ImagePath string
}

// NewSamplePlatformAction creates a new SamplePlatformAction
func NewSamplePlatformAction(imagePath string) *SamplePlatformAction {
	action := &SamplePlatformAction{
		TuringPiAction: NewTuringPiAction("SamplePlatformAction", "Sample platform-aware action"),
		ImagePath:      imagePath,
	}
	return action
}

// ExecuteNative implements the native Linux execution path
func (a *SamplePlatformAction) ExecuteNative(ctx *gostate.ActionContext) error {
	// Get required tools
	fsTool, err := a.GetFSTool()
	if err != nil {
		return fmt.Errorf("failed to get filesystem tool: %w", err)
	}

	imageTool, err := a.GetImageTool()
	if err != nil {
		return fmt.Errorf("failed to get image tool: %w", err)
	}

	// Perform native Linux operations
	ctx.Logger.Info("Executing on Linux platform")

	// Example: Check if image file exists
	if !fsTool.FileExists(a.ImagePath) {
		return fmt.Errorf("image file not found: %s", a.ImagePath)
	}

	// Example: Map partitions directly on Linux
	devicePath, err := imageTool.MapPartitions(context.Background(), a.ImagePath)
	if err != nil {
		return fmt.Errorf("failed to map partitions: %w", err)
	}

	// Store the result for downstream actions
	if err := ctx.Store.Put("devicePath", devicePath); err != nil {
		return fmt.Errorf("failed to store device path: %w", err)
	}

	return nil
}

// ExecuteDocker implements the Docker-based execution path for non-Linux platforms
func (a *SamplePlatformAction) ExecuteDocker(ctx *gostate.ActionContext) error {
	// Get required tools
	containerTool, err := a.GetContainerTool()
	if err != nil {
		return fmt.Errorf("failed to get container tool: %w", err)
	}

	// Prepare container environment
	ctx.Logger.Info("Executing via Docker container")

	// Example: Create a container to work with the image
	containerConfig := createContainerConfig(a.ImagePath)
	container, err := containerTool.CreateContainer(context.Background(), containerConfig)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Store container ID for cleanup by downstream actions
	if err := ctx.Store.Put("containerId", container.ID()); err != nil {
		return fmt.Errorf("failed to store container ID: %w", err)
	}

	// Example: Run kpartx command inside container
	cmd := []string{"kpartx", "-av", filepath.Base(a.ImagePath)}
	output, err := containerTool.RunCommand(context.Background(), container.ID(), cmd)
	if err != nil {
		return fmt.Errorf("failed to map partitions in container: %w", err)
	}

	// Parse output to get device path and store it
	devicePath := parseKpartxOutput(output)
	if err := ctx.Store.Put("containerDevicePath", devicePath); err != nil {
		return fmt.Errorf("failed to store container device path: %w", err)
	}

	return nil
}

// Helper functions for the sample action

// createContainerConfig creates a container configuration for the operation
func createContainerConfig(imagePath string) container.ContainerConfig {
	// Create configuration for working with disk images
	return container.ContainerConfig{
		Image:      "ubuntu:latest",
		Name:       "turingpi-image-processor",
		Command:    []string{"sleep", "infinity"},
		WorkDir:    "/workspace",
		Privileged: true,
		Capabilities: []string{
			"SYS_ADMIN",
			"MKNOD",
		},
		Mounts: map[string]string{
			filepath.Dir(imagePath): "/images",
		},
	}
}

// parseKpartxOutput parses the output of kpartx to extract device path
func parseKpartxOutput(output string) string {
	// This is a placeholder - implement actual parsing logic
	return "/dev/mapper/sample"
}
