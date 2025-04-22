// Package ubuntu provides actions for Ubuntu image preparation and deployment
package ubuntu

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/pkg/v2/cache"
	"github.com/davidroman0O/turingpi/pkg/v2/keys"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
	"github.com/davidroman0O/turingpi/pkg/v2/workflows/actions"
)

// ImageFinalizeAction handles cleanup, compression, and caching of the prepared image
type ImageFinalizeAction struct {
	actions.PlatformActionBase
}

// NewImageFinalizeAction creates a new action to finalize the image preparation
func NewImageFinalizeAction() *ImageFinalizeAction {
	return &ImageFinalizeAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"ubuntu-image-finalize",
			"Finalizes the Ubuntu image preparation by cleaning up, compressing, and caching",
		),
	}
}

// ExecuteNative implements execution on native platforms
func (a *ImageFinalizeAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// ExecuteDocker implements execution via Docker
func (a *ImageFinalizeAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// executeImpl is the shared implementation
func (a *ImageFinalizeAction) executeImpl(ctx *gostage.ActionContext, toolsProvider tools.ToolProvider) error {
	// Get node ID from store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return fmt.Errorf("failed to get node ID: %w", err)
	}

	// Check if we previously found a cached image
	cachedImagePath, err := store.Get[string](ctx.Store(), "CachedImagePath")
	if err == nil && cachedImagePath != "" {
		ctx.Logger.Info("Using previously found cached image: %s", cachedImagePath)

		// Store final image path for deployment
		if err := ctx.Store().Put("FinalImagePath", cachedImagePath); err != nil {
			return fmt.Errorf("failed to store final image path: %w", err)
		}

		return nil
	}

	// Check if we modified the image
	imageModified, err := store.GetOrDefault[bool](ctx.Store(), "ImageModified", false)
	if err != nil {
		return fmt.Errorf("failed to check if image was modified: %w", err)
	}

	if !imageModified {
		ctx.Logger.Info("No image modifications were performed, nothing to finalize")
		return nil
	}

	// Get required info from previous actions
	decompressedImgPath, err := store.Get[string](ctx.Store(), "DecompressedImagePath")
	if err != nil {
		return fmt.Errorf("failed to get decompressed image path: %w", err)
	}

	mountDir, err := store.Get[string](ctx.Store(), "MountDir")
	if err != nil {
		return fmt.Errorf("failed to get mount directory: %w", err)
	}

	// Get image operations tool
	imageTool, err := a.GetOperationsTool()
	if err != nil {
		return fmt.Errorf("failed to get image operations tool: %w", err)
	}

	// 1. Unmount filesystem
	ctx.Logger.Info("Unmounting filesystem at: %s", mountDir)
	if err := imageTool.UnmountFilesystem(context.Background(), mountDir); err != nil {
		ctx.Logger.Warn("Error unmounting filesystem: %v", err)
		// Continue anyway to try to clean up other resources
	}

	// 2. Unmap partitions
	ctx.Logger.Info("Unmapping partitions for: %s", decompressedImgPath)
	if err := imageTool.UnmapPartitions(context.Background(), decompressedImgPath); err != nil {
		ctx.Logger.Warn("Error unmapping partitions: %v", err)
		// Continue anyway to try to salvage what we can
	}

	// 3. Prepare output filename
	hostname, err := store.GetOrDefault[string](ctx.Store(), "Hostname", fmt.Sprintf("node%d", nodeID))
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	outputFilename := fmt.Sprintf("%s-rk1-ubuntu.img.xz", hostname)
	tempDir := filepath.Dir(decompressedImgPath)
	outputPath := filepath.Join(tempDir, outputFilename)

	// 4. Compress image
	ctx.Logger.Info("Compressing image to: %s", outputPath)
	if err := imageTool.CompressXZ(context.Background(), decompressedImgPath, outputPath); err != nil {
		return fmt.Errorf("failed to compress image: %w", err)
	}

	// 5. Cache the result
	ctx.Logger.Info("Caching compressed image")
	localCache := toolsProvider.GetLocalCache()
	if localCache != nil {
		// Get the cache key (should have been stored in previous action)
		cacheKey, err := store.Get[string](ctx.Store(), "ImageCacheKey")
		if err != nil {
			ctx.Logger.Warn("Could not get cache key: %v", err)
		} else {
			// Create metadata for cache
			inputHash, _ := store.GetOrDefault[string](ctx.Store(), "InputHash", "")

			metadata := cache.Metadata{
				Key:         cacheKey,
				Filename:    outputPath,
				ContentType: "application/octet-stream",
				Tags: map[string]string{
					"nodeID":   fmt.Sprint(nodeID),
					"hostname": hostname,
					"board":    "rk1",
					"os":       "ubuntu",
				},
				OSType:    "ubuntu",
				OSVersion: "latest",
				Hash:      inputHash,
			}

			// Open file for reading
			file, err := os.Open(outputPath)
			if err != nil {
				ctx.Logger.Warn("Failed to open image file for caching: %v", err)
			} else {
				defer file.Close()

				// Store in cache
				_, err = localCache.Put(context.Background(), cacheKey, metadata, file)
				if err != nil {
					ctx.Logger.Warn("Failed to cache image: %v", err)
				} else {
					ctx.Logger.Info("Image cached successfully with key: %s", cacheKey)
				}
			}
		}
	} else {
		ctx.Logger.Info("Local cache not available, skipping cache step")
	}

	// Store final image path for deployment
	if err := ctx.Store().Put("FinalImagePath", outputPath); err != nil {
		return fmt.Errorf("failed to store final image path: %w", err)
	}

	ctx.Logger.Info("Image preparation finalized successfully")
	return nil
}
