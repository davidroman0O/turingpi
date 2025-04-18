// Package imageops provides functions for OS image preparation and modification.
package imageops

import (
	"io/fs"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/docker"
	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// ImageOpsAdapter defines the interface for image operations.
type ImageOpsAdapter interface {
	// PrepareImage decompresses a disk image, modifies it with network settings, and recompresses it.
	PrepareImage(opts PrepareImageOptions) (string, error)

	// DecompressImageXZ decompresses an XZ-compressed disk image.
	DecompressImageXZ(sourceImgXZAbs, tmpDir string) (string, error)

	// MapPartitions uses kpartx to map disk partitions.
	MapPartitions(imgPathAbs string) (string, error)

	// CleanupPartitions unmaps partitions.
	CleanupPartitions(imgPathAbs string) error

	// MountFilesystem mounts a filesystem.
	MountFilesystem(partitionDevice, mountDir string) error

	// UnmountFilesystem unmounts a filesystem.
	UnmountFilesystem(mountDir string) error

	// ApplyNetworkConfig applies network config to the mounted filesystem.
	ApplyNetworkConfig(mountDir string, hostname string, ipCIDR string, gateway string, dnsServers []string) error

	// RecompressImageXZ compresses a disk image with XZ.
	RecompressImageXZ(modifiedImgPath, finalXZPath string) error

	// ExecuteFileOperations executes a batch of file operations.
	ExecuteFileOperations(params ExecuteFileOperationsParams) error

	// WriteToImageFile writes content to a file within the mounted image.
	WriteToImageFile(mountDir, relativePath string, content []byte, perm fs.FileMode) error

	// CopyFileToImage copies a local file into the mounted image.
	CopyFileToImage(mountDir, localSourcePath, relativeDestPath string) error

	// MkdirInImage creates a directory within the mounted image.
	MkdirInImage(mountDir, relativePath string, perm fs.FileMode) error

	// ChmodInImage changes permissions of a file/directory within the mounted image.
	ChmodInImage(mountDir, relativePath string, perm fs.FileMode) error

	// Cleanup releases any resources used by the adapter.
	Cleanup() error
}

// PrepareImageOptions contains all parameters needed to prepare an image
type PrepareImageOptions struct {
	SourceImgXZ  string   // Path to the source compressed image
	NodeNum      int      // Node number (used for default hostname if needed)
	IPAddress    string   // IP address without CIDR
	IPCIDRSuffix string   // CIDR suffix (e.g., "/24")
	Hostname     string   // Hostname to set
	Gateway      string   // Gateway IP address
	DNSServers   []string // List of DNS server IPs
	OutputDir    string   // Directory to store output image
	TempDir      string   // Directory for temporary processing
}

// imageOpsAdapter implements the ImageOpsAdapter interface.
type imageOpsAdapter struct {
	dockerAdapter *docker.DockerAdapter
	dockerConfig  *platform.DockerExecutionConfig
	startTime     time.Time
}

// NewImageOpsAdapter creates a new adapter for image operations.
func NewImageOpsAdapter(sourceDir, tempDir, outputDir string) (ImageOpsAdapter, error) {
	adapter := &imageOpsAdapter{
		startTime: time.Now(),
	}

	// Initialize Docker for non-Linux platforms
	if !platform.IsLinux() {
		if err := adapter.initDocker(sourceDir, tempDir, outputDir); err != nil {
			return nil, err
		}
	}

	return adapter, nil
}

// Cleanup releases any resources used by the adapter.
func (a *imageOpsAdapter) Cleanup() error {
	if a.dockerAdapter != nil {
		return a.dockerAdapter.Close()
	}
	return nil
}

// The adapter will need helper methods and implementations of the interface methods,
// which will be contained in other files.
