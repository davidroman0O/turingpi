// Package ubuntu provides actions for Ubuntu image preparation and deployment
package ubuntu

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/pkg/v2/cache"
	"github.com/davidroman0O/turingpi/pkg/v2/keys"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
	"github.com/davidroman0O/turingpi/pkg/v2/workflows/actions"
)

// ImageUploadAction uploads the prepared image to the remote cache on the Turing Pi
type ImageUploadAction struct {
	actions.PlatformActionBase
}

// NewImageUploadAction creates a new action to upload the image to the remote cache
func NewImageUploadAction() *ImageUploadAction {
	return &ImageUploadAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"ubuntu-image-upload",
			"Uploads the prepared Ubuntu image to the remote cache on the Turing Pi",
		),
	}
}

// ExecuteNative implements execution on native platforms
func (a *ImageUploadAction) ExecuteNative(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// ExecuteDocker implements execution via Docker
func (a *ImageUploadAction) ExecuteDocker(ctx *gostage.ActionContext, tools tools.ToolProvider) error {
	return a.executeImpl(ctx, tools)
}

// executeImpl is the shared implementation
func (a *ImageUploadAction) executeImpl(ctx *gostage.ActionContext, toolsProvider tools.ToolProvider) error {
	// Get node ID from store
	nodeID, err := store.GetOrDefault[int](ctx.Store(), keys.CurrentNodeID, 1)
	if err != nil {
		return fmt.Errorf("failed to get node ID: %w", err)
	}

	// Get the path to the image file
	finalImagePath, err := store.Get[string](ctx.Store(), "FinalImagePath")
	if err != nil {
		return fmt.Errorf("failed to get final image path: %w", err)
	}

	// Get the remote cache
	remoteCache := toolsProvider.GetRemoteCache()
	if remoteCache == nil {
		ctx.Logger.Warn("Remote cache is not available, skipping upload")
		// Store remote path as empty to indicate no upload was performed
		if err := ctx.Store().Put("RemoteImagePath", ""); err != nil {
			return fmt.Errorf("failed to store remote image path: %w", err)
		}
		return nil
	}

	// Prepare remote cache key
	hostname, err := store.GetOrDefault[string](ctx.Store(), "Hostname", fmt.Sprintf("node%d", nodeID))
	if err != nil {
		return fmt.Errorf("failed to get hostname: %w", err)
	}

	remoteKey := fmt.Sprintf("node%d-ubuntu-rk1", nodeID)
	ctx.Logger.Info("Uploading image to remote cache with key: %s", remoteKey)

	// Check if it already exists in remote cache
	exists, err := remoteCache.Exists(context.Background(), remoteKey)
	if err == nil && exists {
		ctx.Logger.Info("Image already exists in remote cache, checking if it needs updating")

		// Check if the existing image matches our input hash
		metadata, err := remoteCache.Stat(context.Background(), remoteKey)
		if err == nil {
			inputHash, _ := store.GetOrDefault[string](ctx.Store(), "InputHash", "")
			if metadata.Hash == inputHash {
				ctx.Logger.Info("Existing remote image is up to date, skipping upload")

				// Store remote path for deployment
				if err := ctx.Store().Put("RemoteImagePath", metadata.Filename); err != nil {
					return fmt.Errorf("failed to store remote image path: %w", err)
				}

				return nil
			} else {
				ctx.Logger.Info("Existing remote image is outdated, will upload new version")
			}
		}
	}

	// Open the local image file
	imageFile, err := os.Open(finalImagePath)
	if err != nil {
		return fmt.Errorf("failed to open image file for upload: %w", err)
	}
	defer imageFile.Close()

	// Get file info for metadata
	fileInfo, err := imageFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get image file info: %w", err)
	}

	// Create metadata for remote cache
	inputHash, _ := store.GetOrDefault[string](ctx.Store(), "InputHash", "")
	metadata := cache.Metadata{
		Key:         remoteKey,
		Filename:    filepath.Base(finalImagePath),
		ContentType: "application/octet-stream",
		Size:        fileInfo.Size(),
		ModTime:     time.Now(),
		Hash:        inputHash,
		Tags: map[string]string{
			"nodeID":   fmt.Sprint(nodeID),
			"hostname": hostname,
			"board":    "rk1",
			"os":       "ubuntu",
		},
		OSType:    "ubuntu",
		OSVersion: "latest",
	}

	// Upload to remote cache
	ctx.Logger.Info("Uploading image to remote cache (size: %d bytes)", fileInfo.Size())
	uploadedMetadata, err := remoteCache.Put(context.Background(), remoteKey, metadata, imageFile)
	if err != nil {
		return fmt.Errorf("failed to upload image to remote cache: %w", err)
	}

	ctx.Logger.Info("Image uploaded successfully to: %s", uploadedMetadata.Filename)

	// Store remote path for deployment
	if err := ctx.Store().Put("RemoteImagePath", uploadedMetadata.Filename); err != nil {
		return fmt.Errorf("failed to store remote image path: %w", err)
	}

	return nil
}
