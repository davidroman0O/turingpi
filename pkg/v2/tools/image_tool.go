package tools

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	imageops "github.com/davidroman0O/turingpi/pkg/v2/image"
)

// ImageToolImpl is the implementation of the ImageTool interface
type ImageToolImpl struct {
	containerTool ContainerTool
	imgOps        imageops.ImageOps
}

// MockImageOps is a complete mock implementation of the ImageOps interface
type MockImageOps struct {
	// We embed the ImageOpsImpl to get any existing implementations
	*imageops.ImageOpsImpl
}

// Ensure MockImageOps implements the ImageOps interface
var _ imageops.ImageOps = (*MockImageOps)(nil)

// PrepareImage implements ImageOps
func (m *MockImageOps) PrepareImage(ctx context.Context, opts imageops.PrepareImageOptions) error {
	return fmt.Errorf("PrepareImage not implemented")
}

// ExecuteFileOperations implements ImageOps
func (m *MockImageOps) ExecuteFileOperations(ctx context.Context, params imageops.ExecuteParams) error {
	return fmt.Errorf("ExecuteFileOperations not implemented")
}

// MapPartitions implements ImageOps
func (m *MockImageOps) MapPartitions(ctx context.Context, imgPathAbs string) (string, error) {
	return "", fmt.Errorf("MapPartitions not implemented")
}

// CleanupPartitions implements ImageOps
func (m *MockImageOps) CleanupPartitions(ctx context.Context, imgPathAbs string) error {
	return fmt.Errorf("CleanupPartitions not implemented")
}

// MountFilesystem implements ImageOps
func (m *MockImageOps) MountFilesystem(ctx context.Context, partitionDevice, mountDir string) error {
	return fmt.Errorf("MountFilesystem not implemented")
}

// UnmountFilesystem implements ImageOps
func (m *MockImageOps) UnmountFilesystem(ctx context.Context, mountDir string) error {
	return fmt.Errorf("UnmountFilesystem not implemented")
}

// ApplyNetworkConfig implements ImageOps
func (m *MockImageOps) ApplyNetworkConfig(ctx context.Context, mountDir string, hostname string, ipCIDR string, gateway string, dnsServers []string) error {
	return fmt.Errorf("ApplyNetworkConfig not implemented")
}

// DecompressImageXZ implements ImageOps
func (m *MockImageOps) DecompressImageXZ(ctx context.Context, sourceImgXZAbs, tmpDir string) (string, error) {
	return "", fmt.Errorf("DecompressImageXZ not implemented")
}

// RecompressImageXZ implements ImageOps
func (m *MockImageOps) RecompressImageXZ(ctx context.Context, modifiedImgPath, finalXZPath string) error {
	return fmt.Errorf("RecompressImageXZ not implemented")
}

// WriteToImageFile implements ImageOps
func (m *MockImageOps) WriteToImageFile(ctx context.Context, mountDir, relativePath string, content []byte, perm fs.FileMode) error {
	if m.ImageOpsImpl != nil {
		return m.ImageOpsImpl.WriteToFile(mountDir, relativePath, content, perm)
	}
	return fmt.Errorf("WriteToImageFile not implemented")
}

// CopyFileToImage implements ImageOps
func (m *MockImageOps) CopyFileToImage(ctx context.Context, mountDir, localSourcePath, relativeDestPath string) error {
	if m.ImageOpsImpl != nil {
		return m.ImageOpsImpl.CopyFile(mountDir, localSourcePath, relativeDestPath)
	}
	return fmt.Errorf("CopyFileToImage not implemented")
}

// MkdirInImage implements ImageOps
func (m *MockImageOps) MkdirInImage(ctx context.Context, mountDir, relativePath string, perm fs.FileMode) error {
	if m.ImageOpsImpl != nil {
		return m.ImageOpsImpl.MakeDirectory(mountDir, relativePath, perm)
	}
	return fmt.Errorf("MkdirInImage not implemented")
}

// ChmodInImage implements ImageOps
func (m *MockImageOps) ChmodInImage(ctx context.Context, mountDir, relativePath string, perm fs.FileMode) error {
	if m.ImageOpsImpl != nil {
		return m.ImageOpsImpl.ChangePermissions(mountDir, relativePath, perm)
	}
	return fmt.Errorf("ChmodInImage not implemented")
}

// Close implements ImageOps
func (m *MockImageOps) Close() error {
	return nil
}

// NewImageTool creates a new ImageTool
func NewImageTool(containerTool ContainerTool) ImageTool {
	return &ImageToolImpl{
		containerTool: containerTool,
		imgOps:        &MockImageOps{&imageops.ImageOpsImpl{}},
	}
}

// PrepareImage prepares an image with the given options
func (t *ImageToolImpl) PrepareImage(ctx context.Context, opts imageops.PrepareImageOptions) error {
	// If we have a container tool, we can run the operations in a container
	// Otherwise, fall back to direct operations
	if t.containerTool != nil {
		// Implementation would create a container and run the operation there
		// This is just a placeholder
		return nil
	}

	// Direct implementation
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
func (t *ImageToolImpl) DecompressImageXZ(ctx context.Context, sourceXZ, targetImg string) (string, error) {
	return t.imgOps.DecompressImageXZ(ctx, sourceXZ, filepath.Dir(targetImg))
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
