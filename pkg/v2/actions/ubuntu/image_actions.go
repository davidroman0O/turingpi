// Package ubuntu provides Ubuntu-specific actions for TuringPi
package ubuntu

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidroman0O/gostate"
	"github.com/davidroman0O/gostate/store"
	"github.com/davidroman0O/turingpi/pkg/v2/actions"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
)

// CheckBaseImageAction verifies and downloads the base Ubuntu image if needed
type CheckBaseImageAction struct {
	actions.PlatformActionBase
	osVersion string
}

// NewCheckBaseImageAction creates a new CheckBaseImageAction
func NewCheckBaseImageAction(osVersion string) *CheckBaseImageAction {
	return &CheckBaseImageAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"CheckBaseImage",
			fmt.Sprintf("Verify and download Ubuntu %s base image if needed", osVersion),
		),
		osVersion: osVersion,
	}
}

// ExecuteNative implements the native Linux execution path
func (a *CheckBaseImageAction) ExecuteNative(ctx *gostate.ActionContext, tools tools.ToolProvider) error {
	// Get required tools
	cacheTool := tools.GetCacheTool()
	if cacheTool == nil {
		return fmt.Errorf("cache tool is required but not available")
	}

	fsTool := tools.GetFSTool()
	if fsTool == nil {
		return fmt.Errorf("filesystem tool is required but not available")
	}

	// Get cache dir from context
	cacheDir, err := store.GetOrDefault(ctx.Store, "cacheDir", "/var/cache/turingpi")
	if err != nil {
		return fmt.Errorf("failed to get cache directory: %w", err)
	}

	ctx.Logger.Info("Checking for Ubuntu %s base image", a.osVersion)

	// Construct image name and paths
	imageName := fmt.Sprintf("ubuntu-%s-server-arm64.img.xz", a.osVersion)
	cacheKey := fmt.Sprintf("ubuntu/%s", imageName)
	localPath := filepath.Join(cacheDir, "images", "ubuntu", imageName)

	// Check if the image exists in the cache
	exists, err := cacheTool.Exists(context.Background(), cacheKey)
	if err != nil {
		return fmt.Errorf("failed to check cache: %w", err)
	}

	// If image exists, ensure local path exists
	if exists {
		if fsTool.FileExists(localPath) {
			ctx.Logger.Info("Using cached image: %s", localPath)
			// Store the image path in the workflow store for downstream actions
			if err := ctx.Store.Put("baseImagePath", localPath); err != nil {
				return fmt.Errorf("failed to store image path: %w", err)
			}
			return nil
		}
	}

	// Need to download the image
	ctx.Logger.Info("Downloading Ubuntu %s image...", a.osVersion)

	// Ensure the directory exists
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create image directory: %w", err)
	}

	// TODO: Implement the actual download logic
	// This would typically use an HTTP client to download from Ubuntu's website
	// For now, just simulate it by returning an error that the image is missing

	return fmt.Errorf("Ubuntu %s base image not found and automatic download not implemented", a.osVersion)
}

// ExecuteDocker implements the Docker-based execution path for non-Linux platforms
func (a *CheckBaseImageAction) ExecuteDocker(ctx *gostate.ActionContext, tools tools.ToolProvider) error {
	// For image checking/downloading, we can use the same logic as the native implementation
	// because we're just verifying and potentially downloading a file, not doing OS-specific operations
	return a.ExecuteNative(ctx, tools)
}

// DecompressImageAction decompresses the base Ubuntu XZ image
type DecompressImageAction struct {
	actions.PlatformActionBase
}

// NewDecompressImageAction creates a new DecompressImageAction
func NewDecompressImageAction() *DecompressImageAction {
	return &DecompressImageAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"DecompressImage",
			"Decompress the Ubuntu base image for customization",
		),
	}
}

// ExecuteNative implements the native Linux execution path
func (a *DecompressImageAction) ExecuteNative(ctx *gostate.ActionContext, tools tools.ToolProvider) error {
	// Get required tools
	imageTool := tools.GetImageTool()
	if imageTool == nil {
		return fmt.Errorf("image tool is required but not available")
	}

	// Get the base image path from the context
	baseImagePath, err := store.Get[string](ctx.Store, "baseImagePath")
	if err != nil {
		return fmt.Errorf("base image path not found in context: %w", err)
	}

	// Get the temporary directory for decompression
	tempDir, err := store.GetOrDefault(ctx.Store, "tempDir", os.TempDir())
	if err != nil {
		return fmt.Errorf("failed to get temp directory: %w", err)
	}

	ctx.Logger.Info("Decompressing image: %s", baseImagePath)

	// Construct the output path for the decompressed image
	outputPath := filepath.Join(tempDir, fmt.Sprintf("%s.img", filepath.Base(baseImagePath[:len(baseImagePath)-3])))

	// Decompress the image
	decompressedPath, err := imageTool.DecompressImageXZ(context.Background(), baseImagePath, outputPath)
	if err != nil {
		return fmt.Errorf("failed to decompress image: %w", err)
	}

	// Store the decompressed image path in the context
	if err := ctx.Store.Put("decompressedImagePath", decompressedPath); err != nil {
		return fmt.Errorf("failed to store decompressed image path: %w", err)
	}

	ctx.Logger.Info("Image decompressed to: %s", decompressedPath)
	return nil
}

// ExecuteDocker implements the Docker-based execution path for non-Linux platforms
func (a *DecompressImageAction) ExecuteDocker(ctx *gostate.ActionContext, tools tools.ToolProvider) error {
	// Get required tools
	containerTool := tools.GetContainerTool()
	if containerTool == nil {
		return fmt.Errorf("container tool is required but not available")
	}

	// Get the base image path from the context
	baseImagePath, err := store.Get[string](ctx.Store, "baseImagePath")
	if err != nil {
		return fmt.Errorf("base image path not found in context: %w", err)
	}

	// Get the temporary directory for decompression
	tempDir, err := store.GetOrDefault(ctx.Store, "tempDir", os.TempDir())
	if err != nil {
		return fmt.Errorf("failed to get temp directory: %w", err)
	}

	ctx.Logger.Info("Decompressing image in Docker container: %s", baseImagePath)

	// Create a container for the decompression
	config := createImageToolContainerConfig(baseImagePath, tempDir)
	container, err := containerTool.CreateContainer(context.Background(), config)
	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	// Ensure cleanup
	defer func() {
		ctx.Logger.Info("Cleaning up container")
		_ = containerTool.RemoveContainer(context.Background(), container.ID())
	}()

	// Construct the container-relative paths
	containerXZPath := filepath.Join("/images", filepath.Base(baseImagePath))
	containerOutPath := containerXZPath[:len(containerXZPath)-3] // Remove .xz extension

	// Run the decompression command
	cmd := []string{"unxz", "-k", "-f", containerXZPath}
	output, err := containerTool.RunCommand(context.Background(), container.ID(), cmd)
	if err != nil {
		return fmt.Errorf("decompression failed in container: %w, output: %s", err, output)
	}

	// Verify the output file exists in the container
	verifyCmd := []string{"ls", "-la", containerOutPath}
	output, err = containerTool.RunCommand(context.Background(), container.ID(), verifyCmd)
	if err != nil {
		return fmt.Errorf("decompressed file not found in container: %w", err)
	}

	// The actual file will be in the host's tempDir due to the volume mounting
	hostOutPath := filepath.Join(tempDir, filepath.Base(containerOutPath))

	// Store the decompressed image path in the context
	if err := ctx.Store.Put("decompressedImagePath", hostOutPath); err != nil {
		return fmt.Errorf("failed to store decompressed image path: %w", err)
	}

	ctx.Logger.Info("Image decompressed to: %s", hostOutPath)
	return nil
}

// Helper function to create a container configuration for image operations
func createImageToolContainerConfig(imagePath, mountDir string) container.ContainerConfig {
	return container.ContainerConfig{
		Image:      "ubuntu:latest",
		Command:    []string{"sleep", "infinity"}, // Keep container running
		WorkDir:    "/workspace",
		Privileged: true, // Needed for disk operations
		Capabilities: []string{
			"SYS_ADMIN",
			"MKNOD",
		},
		Mounts: map[string]string{
			filepath.Dir(imagePath): "/images", // Mount the image directory
			mountDir:                "/output", // Mount the output directory
		},
	}
}
