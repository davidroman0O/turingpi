package imageops

import (
	"context"
	"io/fs"
)

// PrepareImageOptions holds options for preparing an OS image
type PrepareImageOptions struct {
	// Source image path (XZ compressed)
	SourceImgXZ string

	// Node number for configuration
	NodeNum int

	// Network configuration
	Hostname     string
	IPAddress    string
	Gateway      string
	DNSServers   []string
	IPCIDRSuffix string // e.g., "/24"

	// Directories
	TempDir   string // Directory for temporary files
	OutputDir string // Directory for final output
}

// Operation represents a single file operation
type Operation interface {
	// Execute performs the operation
	Execute(ctx context.Context, mountDir string) error
}

// WriteOperation represents an operation to write data to a file
type WriteOperation struct {
	Path     string
	Content  []byte
	FileMode fs.FileMode
}

// Execute implements Operation
func (o WriteOperation) Execute(ctx context.Context, mountDir string) error {
	ops := &ImageOpsImpl{}
	return ops.WriteToFile(mountDir, o.Path, o.Content, o.FileMode)
}

// CopyOperation represents an operation to copy a file
type CopyOperation struct {
	SourcePath string
	DestPath   string
}

// Execute implements Operation
func (o CopyOperation) Execute(ctx context.Context, mountDir string) error {
	ops := &ImageOpsImpl{}
	return ops.CopyFile(mountDir, o.SourcePath, o.DestPath)
}

// MkdirOperation represents an operation to create a directory
type MkdirOperation struct {
	Path     string
	FileMode fs.FileMode
}

// Execute implements Operation
func (o MkdirOperation) Execute(ctx context.Context, mountDir string) error {
	ops := &ImageOpsImpl{}
	return ops.MakeDirectory(mountDir, o.Path, o.FileMode)
}

// ChmodOperation represents an operation to change file permissions
type ChmodOperation struct {
	Path     string
	FileMode fs.FileMode
}

// Execute implements Operation
func (o ChmodOperation) Execute(ctx context.Context, mountDir string) error {
	ops := &ImageOpsImpl{}
	return ops.ChangePermissions(mountDir, o.Path, o.FileMode)
}

// ExecuteParams holds parameters for executing file operations
type ExecuteParams struct {
	MountDir   string
	Operations []Operation
}

// ImageOps defines the interface for image operations
type ImageOps interface {
	// PrepareImage prepares an image with the given options
	PrepareImage(ctx context.Context, opts PrepareImageOptions) error

	// ExecuteFileOperations executes a series of file operations on the image
	ExecuteFileOperations(ctx context.Context, params ExecuteParams) error

	// MapPartitions maps partitions in a disk image
	MapPartitions(ctx context.Context, imgPathAbs string) (string, error)

	// CleanupPartitions cleans up mapped partitions
	CleanupPartitions(ctx context.Context, imgPathAbs string) error

	// MountFilesystem mounts a filesystem
	MountFilesystem(ctx context.Context, partitionDevice, mountDir string) error

	// UnmountFilesystem unmounts a filesystem
	UnmountFilesystem(ctx context.Context, mountDir string) error

	// ApplyNetworkConfig applies network configuration to the mounted image
	ApplyNetworkConfig(ctx context.Context, mountDir string, hostname string, ipCIDR string, gateway string, dnsServers []string) error

	// DecompressImageXZ decompresses an XZ-compressed disk image
	DecompressImageXZ(ctx context.Context, sourceImgXZAbs, tmpDir string) (string, error)

	// RecompressImageXZ compresses a disk image with XZ
	RecompressImageXZ(ctx context.Context, modifiedImgPath, finalXZPath string) error

	// WriteToImageFile writes content to a file in the mounted image
	WriteToImageFile(ctx context.Context, mountDir, relativePath string, content []byte, perm fs.FileMode) error

	// CopyFileToImage copies a local file to the mounted image
	CopyFileToImage(ctx context.Context, mountDir, localSourcePath, relativeDestPath string) error

	// MkdirInImage creates a directory in the mounted image
	MkdirInImage(ctx context.Context, mountDir, relativePath string, perm fs.FileMode) error

	// ChmodInImage changes file permissions in the mounted image
	ChmodInImage(ctx context.Context, mountDir, relativePath string, perm fs.FileMode) error

	// Close releases any resources
	Close() error
}
