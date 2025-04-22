// Package ubuntu provides actions for Ubuntu image preparation and deployment
package ubuntu

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/keys"
	"github.com/davidroman0O/turingpi/pkg/v2/platform"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
	"github.com/davidroman0O/turingpi/pkg/v2/workflows/actions"
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
	return a.executeImpl(ctx, tools)
}

// ExecuteDocker implements execution via Docker
func (a *ImagePrepareAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	// return a.executeImpl(ctx, tools)
	// Get required parameters from the store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return fmt.Errorf("failed to get node ID: %w", err)
	}

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

	tempDir, err := tools.GetTmpCache().CreateTempDir(ctx.GoContext, fmt.Sprintf("image-prepare-node-%d-%d", nodeID, time.Now().Unix()))
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}

	sourceImageName := filepath.Base(sourceImagePath)
	targetImagePath := filepath.Join(tempDir, sourceImageName)
	if err = copyFile(sourceImagePath, targetImagePath); err != nil {
		return fmt.Errorf("failed to copy source image: %w", err)
	}

	ctn, err := tools.GetContainerTool().CreateContainer(ctx.GoContext, container.ContainerConfig{
		Image: "ubuntu:22.04",
		Name:  fmt.Sprintf("image-prepare-node-%d-%d", nodeID, time.Now().Unix()),
		Resources: container.ResourceLimits{
			CPUShares: 2,
			Memory:    2048,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create container: %w", err)
	}

	if strings.HasSuffix(targetImagePath, ".xz") {
		targetImagePath = strings.TrimSuffix(targetImagePath, ".xz")
		if _, err = tools.GetImageTool().DecompressImageXZ(ctx.GoContext, targetImagePath, targetImagePath); err != nil {
			return fmt.Errorf("failed to decompress source image: %w", err)
		}
	}

	return nil
}

// executeImpl is the shared implementation
func (a *ImagePrepareAction) executeImpl(ctx *gostage.ActionContext, toolsProvider tools.ToolProvider) error {

	// // Get network configuration - optional but recommended
	// hostname, _ := store.GetOrDefault[string](ctx.Store(), "Hostname", fmt.Sprintf("rk1-node-%d", nodeID))
	// // Get network configuration - optional but recommended
	// hostname, _ := store.GetOrDefault[string](ctx.Store(), "Hostname", fmt.Sprintf("rk1-node-%d", nodeID))
	// ipCIDR, _ := store.GetOrDefault[string](ctx.Store(), "IPCIDR", "")
	// gateway, _ := store.GetOrDefault[string](ctx.Store(), "Gateway", "")
	// dnsServers, _ := store.GetOrDefault[string](ctx.Store(), "DNSServers", "")

	// ctx.Logger.Info("Image preparation for node %d", nodeID)
	// ctx.Logger.Info("Source image: %s", sourceImagePath)
	// ctx.Logger.Info("Hostname: %s", hostname)
	// ctx.Logger.Info("IP CIDR: %s", ipCIDR)
	// ctx.Logger.Info("Gateway: %s", gateway)
	// ctx.Logger.Info("DNS Servers: %s", dnsServers)

	// // Generate a unique input hash based on settings
	// h := sha256.New()
	// h.Write([]byte(sourceImagePath))
	// h.Write([]byte(hostname))
	// h.Write([]byte(ipCIDR))
	// h.Write([]byte(gateway))
	// h.Write([]byte(dnsServers))
	// inputHash := hex.EncodeToString(h.Sum(nil))
	// ctx.Logger.Info("Input hash: %s", inputHash)

	// // Get local cache tool for looking up existing images
	// localCache := toolsProvider.GetLocalCache()
	// if localCache != nil {
	// 	// Create cache key based on details
	// 	cacheKey := generateCacheKey(inputHash, "ubuntu", "rk1")
	// 	ctx.Logger.Debug("Cache key: %s", cacheKey)

	// 	// Store cache key for later use
	// 	if err := ctx.Store().Put("ImageCacheKey", cacheKey); err != nil {
	// 		return fmt.Errorf("failed to store image cache key: %w", err)
	// 	}

	// 	// Check if the image already exists in cache
	// 	exists, err := localCache.Exists(context.Background(), cacheKey)
	// 	if err == nil && exists {
	// 		ctx.Logger.Info("Found prepared image in cache with key: %s", cacheKey)

	// 		// Get metadata
	// 		metadata, err := localCache.Stat(context.Background(), cacheKey)
	// 		if err == nil {
	// 			// Store cache info in context
	// 			if err := ctx.Store().Put("CachedImagePath", metadata.Filename); err != nil {
	// 				return fmt.Errorf("failed to store cached image path: %w", err)
	// 			}

	// 			ctx.Logger.Info("Using cached image: %s", metadata.Filename)
	// 			return nil
	// 		}

	// 		ctx.Logger.Warn("Error reading cached image metadata: %v", err)
	// 	}
	// }

	// // If we get here, we need to prepare a new image
	// ctx.Logger.Info("No usable cached image found, will prepare a new one")

	// // Get local cache directory
	// var cacheDir string
	// if localCache != nil {
	// 	cacheDir = localCache.Location() // Use Location() method to get the cache directory path
	// } else {
	// 	// Fallback to temp directory if no cache available
	// 	cacheDir = os.TempDir()
	// }

	// // Create a temp working directory in the cache directory
	// tempWorkDir, err := os.MkdirTemp(cacheDir, fmt.Sprintf("turingpi-image-node%d-*", nodeID))
	// if err != nil {
	// 	return fmt.Errorf("failed to create temp directory: %w", err)
	// }
	// // Store temp working directory in context for later cleanup if needed
	// if err := ctx.Store().Put("TempWorkDir", tempWorkDir); err != nil {
	// 	return fmt.Errorf("failed to store temp work dir: %w", err)
	// }
	// ctx.Logger.Info("Created temporary directory: %s", tempWorkDir)

	// // Copy the source image to the temp directory
	// sourceImageName := filepath.Base(sourceImagePath)
	// targetImagePath := filepath.Join(tempWorkDir, sourceImageName)

	// ctx.Logger.Info("Copying source image to temporary directory...")
	// if err := copyFile(sourceImagePath, targetImagePath); err != nil {
	// 	return fmt.Errorf("failed to copy source image: %w", err)
	// }

	// // Update source image path in workflow store to point to the copied file
	// if err := ctx.Store().Put("WorkingImagePath", targetImagePath); err != nil {
	// 	return fmt.Errorf("failed to store working image path: %w", err)
	// }

	// ctx.Logger.Info("Source image copied to: %s", targetImagePath)

	// // 1. Decompress the image (if it's compressed)
	// if strings.HasSuffix(targetImagePath, ".xz") {
	// 	ctx.Logger.Info("Decompressing source image...")

	// 	// Get the image tool from the tools provider
	// 	imageTool := toolsProvider.GetImageTool()
	// 	if imageTool == nil {
	// 		return fmt.Errorf("image operations tool not available")
	// 	}

	// 	// Use the tool to decompress the XZ file
	// 	resultPath, err := imageTool.DecompressImageXZ(context.Background(), targetImagePath, filepath.Dir(targetImagePath))
	// 	if err != nil {
	// 		return fmt.Errorf("failed to decompress source image: %w", err)
	// 	}

	// 	// Update image path to decompressed version
	// 	if err := ctx.Store().Put("DecompressedImagePath", resultPath); err != nil {
	// 		return fmt.Errorf("failed to store decompressed image path: %w", err)
	// 	}

	// 	ctx.Logger.Info("Decompressed image path: %s", resultPath)
	// }

	// // Store the prepared image for the next action
	// preparedImagePath := targetImagePath
	// if strings.HasSuffix(targetImagePath, ".xz") {
	// 	preparedImagePath = strings.TrimSuffix(targetImagePath, ".xz")
	// }

	// if err := ctx.Store().Put("PreparedImagePath", preparedImagePath); err != nil {
	// 	return fmt.Errorf("failed to store prepared image path: %w", err)
	// }

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

// GetImageTool returns the image operations tool
func (a *ImagePrepareAction) GetImageTool() (interface{}, error) {
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
