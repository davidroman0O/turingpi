package tools

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"

	imageops "github.com/davidroman0O/turingpi/pkg/v2/image"
)

// ImageToolImpl is the implementation of the ImageTool interface
type ImageToolImpl struct {
	containerTool ContainerTool
	imgOps        imageops.ImageOps
}

// NewImageTool creates a new ImageTool
func NewImageTool(containerTool ContainerTool) ImageTool {
	return &ImageToolImpl{
		containerTool: containerTool,
		imgOps:        &imageops.ImageOpsImpl{},
	}
}

// PrepareImage prepares an image with the given options
func (t *ImageToolImpl) PrepareImage(ctx context.Context, opts imageops.PrepareImageOptions) error {
	return t.imgOps.PrepareImage(ctx, opts)
}

// MapPartitions maps partitions in a disk image
func (t *ImageToolImpl) MapPartitions(ctx context.Context, imgPath string) (string, error) {
	return t.imgOps.MapPartitions(ctx, imgPath)
}

// UnmapPartitions unmaps partitions in a disk image
func (t *ImageToolImpl) UnmapPartitions(ctx context.Context, imgPath string) error {
	return t.imgOps.CleanupPartitions(ctx, imgPath)
}

// MountFilesystem mounts a filesystem
func (t *ImageToolImpl) MountFilesystem(ctx context.Context, device, mountDir string) error {
	return t.imgOps.MountFilesystem(ctx, device, mountDir)
}

// UnmountFilesystem unmounts a filesystem
func (t *ImageToolImpl) UnmountFilesystem(ctx context.Context, mountDir string) error {
	return t.imgOps.UnmountFilesystem(ctx, mountDir)
}

// DecompressImageXZ decompresses an XZ-compressed disk image
func (t *ImageToolImpl) DecompressImageXZ(ctx context.Context, sourceXZ, targetDir string) (string, error) {
	return t.imgOps.DecompressImageXZ(ctx, sourceXZ, targetDir)
}

// CompressImageXZ compresses a disk image with XZ
func (t *ImageToolImpl) CompressImageXZ(ctx context.Context, sourceImg, targetXZ string) error {
	return t.imgOps.RecompressImageXZ(ctx, sourceImg, targetXZ)
}

// WriteFile writes content to a file in the mounted image
func (t *ImageToolImpl) WriteFile(ctx context.Context, mountDir, relativePath string, content []byte, perm fs.FileMode) error {
	return t.imgOps.WriteToImageFile(ctx, mountDir, relativePath, content, perm)
}

// CopyFile copies a file to the mounted image
func (t *ImageToolImpl) CopyFile(ctx context.Context, mountDir, sourcePath, destPath string) error {
	return t.imgOps.CopyFileToImage(ctx, mountDir, sourcePath, destPath)
}

// ReadFile reads a file from the mounted image
func (t *ImageToolImpl) ReadFile(ctx context.Context, mountDir, relativePath string) ([]byte, error) {
	// Use standard file system operations - this is not directly part of the ImageOps interface
	fullPath := filepath.Join(mountDir, relativePath)
	return os.ReadFile(fullPath)
}
