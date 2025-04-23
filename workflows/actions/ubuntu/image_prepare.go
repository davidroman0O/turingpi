// Package ubuntu provides actions for Ubuntu image preparation and deployment
package ubuntu

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/keys"
	"github.com/davidroman0O/turingpi/platform"
	"github.com/davidroman0O/turingpi/tools"
	"github.com/davidroman0O/turingpi/workflows/actions"
)

// ImagePrepareAction prepares a Ubuntu image with customized network settings
type ImagePrepareAction struct {
	actions.PlatformActionBase
}

// NewImagePrepareAction creates a new action to prepare an Ubuntu image
func NewImagePrepareAction() *ImagePrepareAction {
	return &ImagePrepareAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"ubuntu-image-prepare",
			"Prepares an Ubuntu image with customized settings",
		),
	}
}

// ExecuteNative implements execution on native platforms
func (a *ImagePrepareAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	// return a.executeImpl(ctx, tools)
	return nil
}

// ExecuteDocker implements execution via Docker
func (a *ImagePrepareAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	// return a.executeImpl(ctx, tools)

	/////// TODO: with all the changes i didnt we might have just one implementation on this action

	// Get required parameters from the store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return fmt.Errorf("failed to get node ID: %w", err)
	}

	tempDir, err := store.Get[string](ctx.Store(), "workflow.tmp.dir")
	if err != nil {
		return fmt.Errorf("failed to get temp directory: %w", err)
	}

	cacheDir, err := store.Get[string](ctx.Store(), "workflow.cache.dir")
	if err != nil {
		return fmt.Errorf("failed to get cache directory: %w", err)
	}

	ctx.Logger.Info("Image preparation for node %d", nodeID)
	ctx.Logger.Info("Temp directory: %s", tempDir)
	ctx.Logger.Info("Cache directory: %s", cacheDir)

	// Get the source image path from the store
	sourceImagePath, err := store.Get[string](ctx.Store(), "SourceImagePath")
	if err != nil {
		return fmt.Errorf("failed to get source image path: %w", err)
	}

	// Check if source image exists
	if _, err := os.Stat(sourceImagePath); os.IsNotExist(err) {
		ctx.Logger.Error("Source image file does not exist: %s", sourceImagePath)
		ctx.Logger.Error("Please make sure the image file is available at the specified path.")
		ctx.Logger.Error("Note: The source image must be a valid Ubuntu image for your target board.")
		return fmt.Errorf("source file does not exist: %s", sourceImagePath)
	}

	// get path without filename
	sourceImageDir := filepath.Dir(sourceImagePath)

	ctx.Workflow.Store.Put("ubuntu.image.source.dir", sourceImageDir)

	// get the source image name
	sourceImageName := filepath.Base(sourceImagePath) // just get the filename

	ctx.Logger.Info("Source image name: %s", sourceImageName)

	// get the target image path
	targetImagePath := filepath.Join(tempDir, sourceImageName) // now we have a destination path

	ctx.Logger.Info("Copying source image to temp directory: %s -> %s", sourceImagePath, targetImagePath)

	// Copy the source image to the temp directory
	if err = copyFile(sourceImagePath, targetImagePath); err != nil {
		return fmt.Errorf("failed to copy source image: %w", err)
	}

	// Debug: Check if the file exists in the temp directory
	if _, err := os.Stat(targetImagePath); os.IsNotExist(err) {
		ctx.Logger.Error("Target image file does not exist after copy: %s", targetImagePath)
		return fmt.Errorf("target file does not exist after copy: %s", targetImagePath)
	} else {
		fileInfo, _ := os.Stat(targetImagePath)
		ctx.Logger.Info("Target image file exists: %s, size: %d bytes", targetImagePath, fileInfo.Size())
	}

	// ctx.Logger.Info("Container started successfully")

	if strings.HasSuffix(targetImagePath, ".xz") {
		// Keep the original path for copying
		// originalImagePath := targetImagePath
		// Remove the suffix for the target path
		decompressedImagePath := strings.TrimSuffix(sourceImageName, ".xz")

		// 	source := fmt.Sprintf("/workdir/%s", sourceImageName)
		// 	targetDir := "/workdir" // Use container path for output dir, not host path
		// fmt.Println("Decompressing image...", source, targetDir+"/"+targetImagePath)

		// 	// // Debug: Log mount points and paths
		// 	ctx.Logger.Info("Host temp dir: %s", absoluteTempDir)
		// 	ctx.Logger.Info("Container mount dir: /workdir")
		// 	ctx.Logger.Info("Source image in container: %s", source)
		// 	ctx.Logger.Info("Target directory in container: %s", targetDir)
		// 	ctx.Logger.Info("Target image name: %s", targetImagePath)

		containerID, err := store.Get[string](ctx.Workflow.Store, "workflow.container.id")
		if err != nil {
			return fmt.Errorf("failed to get container ID: %w", err)
		}

		ctn, err := tools.GetContainerTool().GetContainer(ctx.GoContext, containerID)
		if err != nil {
			return fmt.Errorf("failed to get container: %w", err)
		}

		output, err := ctn.Exec(ctx.GoContext, []string{"ls", "-la", "/tmp"})
		if err != nil {
			ctx.Logger.Error("Failed to list files in container: %v", err)
		} else {
			ctx.Logger.Info("Container ls output: %s", output)
		}

		// Check container contents
		ctx.Logger.Info("Listing files in container...")
		files, err := tools.GetOperationsTool().ListFilesBasic(ctx.GoContext, "/tmp")
		if err != nil {
			return fmt.Errorf("failed to list files in container: %w", err)
		}
		ctx.Logger.Info("Files in container: %v", files)

		for _, file := range files {
			ctx.Logger.Info("File: %s", file)
		}

		ctx.Logger.Info("Decompressing image... from %s -> to %s", fmt.Sprintf("/tmp/%s", sourceImageName), "/tmp/")

		if _, err := tools.GetOperationsTool().DecompressXZ(ctx.GoContext, fmt.Sprintf("/tmp/%s", sourceImageName), "/tmp"); err != nil {
			return fmt.Errorf("failed to decompress image: %w", err)
		}

		if err := tools.GetOperationsTool().Remove(ctx.GoContext, fmt.Sprintf("/tmp/%s", sourceImageName), false); err != nil {
			return fmt.Errorf("failed to remove source image: %w", err)
		}

		ctx.Workflow.Store.Put("ubuntu.image.decompressed.path", "/tmp")
		ctx.Workflow.Store.Put("ubuntu.image.decompressed.file", fmt.Sprintf("/tmp/%s", decompressedImagePath))
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	// Open source file for reading
	sourceFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy contents
	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Ensure contents are written to disk
	err = destFile.Sync()
	if err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	return nil
}

// GetOperationsTool returns the image operations tool
func (a *ImagePrepareAction) GetOperationsTool() (interface{}, error) {
	// FIXME: Implementation needed
	if platform.IsLinux() {
		return nil, fmt.Errorf("native image tool not yet implemented")
	} else if platform.DockerAvailable() {
		return nil, fmt.Errorf("Docker image tool not yet implemented")
	} else {
		return nil, fmt.Errorf("no image tool available for this platform (requires Linux or Docker)")
	}
}

// generateCacheKey creates a cache key for the image
func generateCacheKey(inputHash, osType, boardType string) string {
	return fmt.Sprintf("%s-%s-%s", osType, boardType, inputHash[:7])
}

// ensureCommand checks if a command is available
func ensureCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// execTool executes a command with arguments
func execTool(toolName string, args ...string) error {
	cmd := exec.Command(toolName, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
