// Package ubuntu provides actions for Ubuntu image preparation and deployment
package ubuntu

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/cache"
	"github.com/davidroman0O/turingpi/keys"
	"github.com/davidroman0O/turingpi/tools"
	"github.com/davidroman0O/turingpi/workflows/actions"
)

const (
	// MaxRetries is the maximum number of upload retries
	MaxRetries = 3
	// RetryDelay is the delay between retries in seconds
	RetryDelay = 5
	// ChunkSize is the maximum file size to upload at once (100MB)
	ChunkSize = 100 * 1024 * 1024
)

// ImageUploadAction uploads the prepared image to the BMC
type ImageUploadAction struct {
	actions.PlatformActionBase
}

// NewImageUploadAction creates a new action to upload an Ubuntu image to the BMC
func NewImageUploadAction() *ImageUploadAction {
	return &ImageUploadAction{
		PlatformActionBase: actions.NewPlatformActionBase(
			"ubuntu-image-upload",
			"Uploads the prepared Ubuntu image to the BMC for flashing",
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

	// Get the compressed image path and extract just the filename
	compressedImagePath, err := store.Get[string](ctx.Store(), "ubuntu.image.compressed.file")
	if err != nil {
		return fmt.Errorf("failed to get compressed image path: %w", err)
	}

	// Get the source image directory from the host machine
	sourceImageDir, err := store.Get[string](ctx.Store(), "ubuntu.image.source.dir")
	if err != nil {
		return fmt.Errorf("failed to get source image directory: %w", err)
	}

	// Extract image filename
	imageXZName := filepath.Base(compressedImagePath)

	// Create the full host path for the compressed image
	hostImagePath := filepath.Join(sourceImageDir, imageXZName)
	ctx.Logger.Info("Using host image path: %s", hostImagePath)

	// Define remote paths on BMC
	remoteBaseDir := "/root/imgs" // Standard cache location on BMC
	remoteNodeDir := fmt.Sprintf("%s/%d", remoteBaseDir, nodeID)
	remoteXZPath := fmt.Sprintf("%s/%s", remoteNodeDir, imageXZName)

	// Get BMC tool
	bmcTool := toolsProvider.GetBMCTool()
	if bmcTool == nil {
		return fmt.Errorf("BMC tool not available")
	}

	// Create remote directory
	ctx.Logger.Info("Creating remote directory: %s", remoteNodeDir)
	_, stderr, err := bmcTool.ExecuteCommand(ctx.GoContext, fmt.Sprintf("mkdir -p %s", remoteNodeDir))
	if err != nil {
		return fmt.Errorf("failed to create remote directory: %w (stderr: %s)", err, stderr)
	}

	// Check if file already exists on BMC
	ctx.Logger.Info("Checking if image already exists on BMC: %s", remoteXZPath)
	stdout, stderr, err := bmcTool.ExecuteCommand(ctx.GoContext, fmt.Sprintf("test -f %s && echo 'exists' || echo 'not_exists'", remoteXZPath))
	if err != nil {
		return fmt.Errorf("failed to check if image exists on BMC: %w (stderr: %s)", err, stderr)
	}

	if strings.TrimSpace(stdout) == "exists" {
		ctx.Logger.Info("Image already exists on BMC: %s", remoteXZPath)
	} else {
		// Check available disk space on BMC
		ctx.Logger.Info("Checking available disk space on BMC")
		stdout, stderr, err := bmcTool.ExecuteCommand(ctx.GoContext, "df -h /root")
		if err != nil {
			ctx.Logger.Warn("Failed to check disk space: %v (stderr: %s)", err, stderr)
		} else {
			ctx.Logger.Info("Disk space information:\n%s", stdout)
		}

		// Upload image to BMC
		ctx.Logger.Info("Uploading image to BMC: %s -> %s", hostImagePath, remoteXZPath)

		// First attempt: Try direct upload using BMC.UploadFile
		uploadSuccessful := false

		// Implement retry logic for direct upload
		for attempt := 1; attempt <= MaxRetries; attempt++ {
			ctx.Logger.Info("Direct upload attempt %d of %d", attempt, MaxRetries)

			err = bmcTool.UploadFile(ctx.GoContext, hostImagePath, remoteXZPath)
			if err == nil {
				ctx.Logger.Info("Direct upload successful")
				uploadSuccessful = true
				break
			}

			ctx.Logger.Warn("Direct upload attempt %d failed: %v", attempt, err)

			if attempt < MaxRetries {
				ctx.Logger.Info("Retrying in %d seconds...", RetryDelay)
				time.Sleep(RetryDelay * time.Second)

				// Clean up any partial file from the failed attempt
				_, _, _ = bmcTool.ExecuteCommand(ctx.GoContext, fmt.Sprintf("rm -f %s", remoteXZPath))
			}
		}

		// If direct upload failed, fall back to the cache method
		if !uploadSuccessful {
			ctx.Logger.Info("Direct upload failed, trying fallback method: cache upload")

			// Get the remote cache
			remoteCache := toolsProvider.GetRemoteCache()
			if remoteCache == nil {
				return fmt.Errorf("remote cache not available")
			}

			// Define a cache key that will match the destination path
			cacheKey := fmt.Sprintf("node%d/%s", nodeID, imageXZName)

			// Get file info for metadata
			fileInfo, err := os.Stat(hostImagePath)
			if err != nil {
				return fmt.Errorf("failed to get file info: %w", err)
			}

			// Create metadata for the file
			metadata := cache.Metadata{
				Filename:    imageXZName,
				Size:        fileInfo.Size(),
				ModTime:     fileInfo.ModTime(),
				ContentType: "application/octet-stream",
				Tags: map[string]string{
					"type":   "ubuntu-image",
					"nodeID": fmt.Sprintf("%d", nodeID),
				},
			}

			// Implement retry logic for cache upload
			for attempt := 1; attempt <= MaxRetries; attempt++ {
				ctx.Logger.Info("Cache upload attempt %d of %d", attempt, MaxRetries)

				// Open the local file for reading using the host machine path
				file, err := os.Open(hostImagePath)
				if err != nil {
					return fmt.Errorf("failed to open local file: %w", err)
				}

				// Upload the file to remote cache
				_, err = remoteCache.Put(ctx.GoContext, cacheKey, metadata, file)

				// Close the file regardless of success
				file.Close()

				if err == nil {
					// Upload succeeded
					uploadSuccessful = true
					break
				}

				ctx.Logger.Warn("Cache upload attempt %d failed: %v", attempt, err)

				if attempt < MaxRetries {
					ctx.Logger.Info("Retrying in %d seconds...", RetryDelay)
					time.Sleep(RetryDelay * time.Second)

					// Check if remote file exists but is incomplete (possible from failed upload)
					checkStdout, _, checkErr := bmcTool.ExecuteCommand(ctx.GoContext, fmt.Sprintf("test -f %s && echo 'exists' || echo 'not_exists'", remoteCache.Location()+"/"+cacheKey+".data"))
					if checkErr == nil && strings.TrimSpace(checkStdout) == "exists" {
						ctx.Logger.Info("Removing partial file from previous attempt")
						_, _, _ = bmcTool.ExecuteCommand(ctx.GoContext, fmt.Sprintf("rm -f %s", remoteCache.Location()+"/"+cacheKey+".data"))
					}
				}
			}

			// If cache upload succeeded, copy to final location
			if uploadSuccessful && remoteCache.Location() != "" && strings.HasPrefix(cacheKey, "node") {
				ctx.Logger.Info("Copying file from cache to destination: %s -> %s", remoteCache.Location()+"/"+cacheKey+".data", remoteXZPath)
				_, stderr, err = bmcTool.ExecuteCommand(ctx.GoContext, fmt.Sprintf("cp %s %s", remoteCache.Location()+"/"+cacheKey+".data", remoteXZPath))
				if err != nil {
					return fmt.Errorf("failed to copy image from cache to destination: %w (stderr: %s)", err, stderr)
				}
			}
		}

		// If both direct and cache uploads failed, fall back to the chunked upload as a last resort
		if !uploadSuccessful {
			ctx.Logger.Info("All standard upload methods failed, trying last resort: chunked upload")

			// Get file info for the chunked upload
			fileInfo, err := os.Stat(hostImagePath)
			if err != nil {
				return fmt.Errorf("failed to get file info: %w", err)
			}

			// Create a temporary file on BMC for the upload
			tempRemotePath := fmt.Sprintf("%s.part", remoteXZPath)

			// Clean up any existing partial file
			_, _, _ = bmcTool.ExecuteCommand(ctx.GoContext, fmt.Sprintf("rm -f %s", tempRemotePath))

			// Create the file
			_, stderr, err = bmcTool.ExecuteCommand(ctx.GoContext, fmt.Sprintf("touch %s", tempRemotePath))
			if err != nil {
				return fmt.Errorf("failed to create temporary file on BMC: %w (stderr: %s)", err, stderr)
			}

			// Open the local file
			file, err := os.Open(hostImagePath)
			if err != nil {
				return fmt.Errorf("failed to open local file: %w", err)
			}
			defer file.Close()

			// Get file size
			fileSize := fileInfo.Size()
			totalChunks := (fileSize + ChunkSize - 1) / ChunkSize // Ceiling division

			// Read in chunks and upload each chunk
			buffer := make([]byte, ChunkSize)
			chunk := 0
			totalUploaded := int64(0)

			for {
				// Read a chunk
				n, err := file.Read(buffer)
				if err != nil && err != io.EOF {
					return fmt.Errorf("error reading from file: %w", err)
				}

				if n == 0 {
					break // End of file
				}

				chunk++
				ctx.Logger.Info("Uploading chunk %d of %d (%.1f%%)", chunk, totalChunks, float64(totalUploaded)*100.0/float64(fileSize))

				// Create a temporary file for this chunk
				chunkFile := fmt.Sprintf("/tmp/chunk_%d.tmp", chunk)
				err = os.WriteFile(chunkFile, buffer[:n], 0644)
				if err != nil {
					return fmt.Errorf("failed to write chunk to temporary file: %w", err)
				}

				// Use SCP to upload the chunk to BMC
				chunkUploadCmd := fmt.Sprintf("cat %s | dd bs=1024k of=%s seek=%d conv=notrunc",
					chunkFile, tempRemotePath, totalUploaded/1024/1024)

				_, stderr, err = bmcTool.ExecuteCommand(ctx.GoContext, chunkUploadCmd)
				if err != nil {
					// Clean up the temporary chunk file
					os.Remove(chunkFile)
					return fmt.Errorf("failed to upload chunk %d: %w (stderr: %s)", chunk, err, stderr)
				}

				// Clean up the temporary chunk file
				os.Remove(chunkFile)

				totalUploaded += int64(n)
			}

			// Verify the file size
			verifyCmd := fmt.Sprintf("stat -c %%s %s", tempRemotePath)
			stdout, stderr, err = bmcTool.ExecuteCommand(ctx.GoContext, verifyCmd)
			if err != nil {
				return fmt.Errorf("failed to verify uploaded file size: %w (stderr: %s)", err, stderr)
			}

			uploadedSize := strings.TrimSpace(stdout)
			ctx.Logger.Info("Uploaded file size: %s bytes (expected: %d bytes)", uploadedSize, fileSize)

			// Move the temporary file to the final location
			_, stderr, err = bmcTool.ExecuteCommand(ctx.GoContext, fmt.Sprintf("mv %s %s", tempRemotePath, remoteXZPath))
			if err != nil {
				return fmt.Errorf("failed to move temporary file to final location: %w (stderr: %s)", err, stderr)
			}

			uploadSuccessful = true
		}

		if !uploadSuccessful {
			return fmt.Errorf("failed to upload image to BMC after multiple attempts")
		}

		ctx.Logger.Info("Image uploaded successfully")
	}

	// For RK1, we need to decompress the image on the BMC before flashing
	// (The old code shows that we need an uncompressed image for flashing)
	imageName := strings.TrimSuffix(imageXZName, ".xz")
	remoteImgPath := fmt.Sprintf("%s/%s", remoteNodeDir, imageName)

	// Check if uncompressed image already exists
	ctx.Logger.Info("Checking if uncompressed image exists on BMC: %s", remoteImgPath)
	stdout, stderr, err = bmcTool.ExecuteCommand(ctx.GoContext, fmt.Sprintf("test -f %s && echo 'exists' || echo 'not_exists'", remoteImgPath))
	if err != nil {
		return fmt.Errorf("failed to check if uncompressed image exists on BMC: %w (stderr: %s)", err, stderr)
	}

	if strings.TrimSpace(stdout) == "exists" {
		ctx.Logger.Info("Uncompressed image already exists on BMC: %s", remoteImgPath)
	} else {
		// Decompress image on BMC
		ctx.Logger.Info("Decompressing image on BMC: %s", remoteXZPath)
		stdout, stderr, err = bmcTool.ExecuteCommand(ctx.GoContext, fmt.Sprintf("unxz -f -k %s", remoteXZPath))
		if err != nil {
			return fmt.Errorf("failed to decompress image on BMC: %w (stderr: %s)", err, stderr)
		}

		ctx.Logger.Info("Image decompressed successfully")
	}

	// Store remote image path for flashing
	if err := ctx.Store().Put("RemoteImagePath", remoteImgPath); err != nil {
		return fmt.Errorf("failed to store remote image path: %w", err)
	}

	ctx.Logger.Info("Image upload completed successfully. Remote path: %s", remoteImgPath)
	return nil
}
