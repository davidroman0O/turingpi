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

// CompressImageAction compresses the customized Ubuntu image
type CompressImageAction struct {
	actions.PlatformActionBase
}

// NewCompressImageAction creates a new CompressImageAction
func NewCompressImageAction() *CompressImageAction {
	return &CompressImageAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"CompressImage",
			"Compress the customized Ubuntu image",
		),
	}
}

// ExecuteNative implements the native Linux execution path
func (a *CompressImageAction) ExecuteNative(ctx *gostate.ActionContext, toolProvider tools.ToolProvider) error {
	// Get required tools
	imageTool := toolProvider.GetImageTool()
	if imageTool == nil {
		return fmt.Errorf("image tool is required but not available")
	}

	fsTool := toolProvider.GetFSTool()
	if fsTool == nil {
		return fmt.Errorf("filesystem tool is required but not available")
	}

	// Get the decompressed image path from the context
	decompressedImagePath, err := store.Get[string](ctx.Store, "decompressedImagePath")
	if err != nil {
		return fmt.Errorf("decompressed image path not found in context: %w", err)
	}

	// Get output directory
	outputDir, err := store.GetOrDefault(ctx.Store, "outputDir", filepath.Dir(decompressedImagePath))
	if err != nil {
		return fmt.Errorf("failed to get output directory: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate output file path
	baseImageName := filepath.Base(decompressedImagePath)
	outputXZPath := filepath.Join(outputDir, baseImageName+".xz")

	ctx.Logger.Info("Compressing image: %s -> %s", decompressedImagePath, outputXZPath)

	// Compress the image
	if err := imageTool.CompressImageXZ(context.Background(), decompressedImagePath, outputXZPath); err != nil {
		return fmt.Errorf("failed to compress image: %w", err)
	}

	// Store the compressed image path in the context
	if err := ctx.Store.Put("finalImagePath", outputXZPath); err != nil {
		return fmt.Errorf("failed to store compressed image path: %w", err)
	}

	ctx.Logger.Info("Image compressed successfully to: %s", outputXZPath)
	return nil
}

// ExecuteDocker implements the Docker-based execution path for non-Linux platforms
func (a *CompressImageAction) ExecuteDocker(ctx *gostate.ActionContext, toolProvider tools.ToolProvider) error {
	// Get required tools
	containerTool := toolProvider.GetContainerTool()
	if containerTool == nil {
		return fmt.Errorf("container tool is required but not available")
	}

	// Get the decompressed image path from the context
	decompressedImagePath, err := store.Get[string](ctx.Store, "decompressedImagePath")
	if err != nil {
		return fmt.Errorf("decompressed image path not found in context: %w", err)
	}

	// Get output directory
	outputDir, err := store.GetOrDefault(ctx.Store, "outputDir", filepath.Dir(decompressedImagePath))
	if err != nil {
		return fmt.Errorf("failed to get output directory: %w", err)
	}

	// Ensure output directory exists
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate output file path
	baseImageName := filepath.Base(decompressedImagePath)
	outputXZPath := filepath.Join(outputDir, baseImageName+".xz")

	ctx.Logger.Info("Compressing image in Docker container: %s -> %s", decompressedImagePath, outputXZPath)

	// Create a container for the compression
	config := createContainerConfigForCompression(decompressedImagePath, outputDir)
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
	containerImgPath := filepath.Join("/images", filepath.Base(decompressedImagePath))
	containerOutputDir := "/output"
	containerOutputPath := filepath.Join(containerOutputDir, filepath.Base(outputXZPath))

	// Run the compression command
	// We use "-k" to keep the original file
	xzCmd := []string{"xz", "-z", "-k", "-9", "-f", containerImgPath}
	output, err := containerTool.RunCommand(context.Background(), container.ID(), xzCmd)
	if err != nil {
		return fmt.Errorf("compression failed in container: %w, output: %s", err, output)
	}

	// Verify the output file was created in the container
	verifyCmd := []string{"ls", "-la", containerImgPath + ".xz"}
	output, err = containerTool.RunCommand(context.Background(), container.ID(), verifyCmd)
	if err != nil {
		return fmt.Errorf("compressed file not found in container: %w", err)
	}

	// Move the compressed file to the output directory in the container
	mvCmd := []string{"mv", containerImgPath + ".xz", containerOutputPath}
	output, err = containerTool.RunCommand(context.Background(), container.ID(), mvCmd)
	if err != nil {
		return fmt.Errorf("failed to move compressed file to output directory in container: %w, output: %s", err, output)
	}

	// Store the compressed image path in the context
	if err := ctx.Store.Put("finalImagePath", outputXZPath); err != nil {
		return fmt.Errorf("failed to store compressed image path: %w", err)
	}

	ctx.Logger.Info("Image compressed successfully to: %s", outputXZPath)
	return nil
}

// Helper function to create a container configuration for compression
func createContainerConfigForCompression(imagePath, outputDir string) container.ContainerConfig {
	return container.ContainerConfig{
		Image:      "ubuntu:latest",
		Name:       "turingpi-image-compress",
		Command:    []string{"sleep", "infinity"}, // Keep container running
		WorkDir:    "/workspace",
		Privileged: true, // Not strictly needed for compression, but keeps config similar
		Mounts: map[string]string{
			filepath.Dir(imagePath): "/images", // Mount the image directory
			outputDir:               "/output", // Mount the output directory
		},
	}
}
