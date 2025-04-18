package imageops

/*
This file is being deprecated in favor of the adapter pattern implementation.
It's kept for backward compatibility until all calling code is updated.
New code should use the adapter interface directly.
*/

import (
	"io/fs"
	"sync"

	"github.com/davidroman0O/turingpi/pkg/tpi/docker"
	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// globalAdapter is the singleton adapter instance
var globalAdapter ImageOpsAdapter

// Global adapter instance for singleton access
var (
	adapterMu sync.Mutex
	sourceDir string
	tempDir   string
	outputDir string
)

// GetAdapter returns the singleton adapter instance
// This allows code to get the adapter without reinitializing it every time
// Using the same instance reduces resource usage (Docker containers, etc.)
func GetAdapter() (ImageOpsAdapter, error) {
	adapterMu.Lock()
	defer adapterMu.Unlock()

	if globalAdapter != nil {
		return globalAdapter, nil
	}

	var err error
	globalAdapter, err = NewImageOpsAdapter(sourceDir, tempDir, outputDir)
	return globalAdapter, err
}

// InitAdapter initializes the global adapter with the specified directories
// This must be called before using the adapter functions
func InitAdapter(srcDir, tmpDir, outDir string) error {
	adapterMu.Lock()
	defer adapterMu.Unlock()

	// Save the directories for later use
	sourceDir = srcDir
	tempDir = tmpDir
	outputDir = outDir

	// Clean up any existing adapter
	if globalAdapter != nil {
		globalAdapter.Cleanup()
		globalAdapter = nil
	}

	// Initialize the adapter
	var err error
	globalAdapter, err = NewImageOpsAdapter(srcDir, tmpDir, outDir)
	return err
}

// CleanupAdapter releases all resources used by the global adapter
func CleanupAdapter() error {
	adapterMu.Lock()
	defer adapterMu.Unlock()

	if globalAdapter != nil {
		err := globalAdapter.Cleanup()
		globalAdapter = nil
		return err
	}
	return nil
}

// The following functions are provided for convenience
// They all use the global adapter instance

// PrepareImage prepares an image using the global adapter
func PrepareImage(opts PrepareImageOptions) (string, error) {
	adapter, err := GetAdapter()
	if err != nil {
		return "", err
	}
	return adapter.PrepareImage(opts)
}

// DecompressImageXZ decompresses an XZ-compressed disk image
func DecompressImageXZ(sourceImgXZAbs, tmpDir string) (string, error) {
	adapter, err := GetAdapter()
	if err != nil {
		return "", err
	}
	return adapter.DecompressImageXZ(sourceImgXZAbs, tmpDir)
}

// MapPartitions maps partitions in a disk image
func MapPartitions(imgPathAbs string) (string, error) {
	adapter, err := GetAdapter()
	if err != nil {
		return "", err
	}
	return adapter.MapPartitions(imgPathAbs)
}

// CleanupPartitions cleans up mapped partitions
func CleanupPartitions(imgPathAbs string) error {
	adapter, err := GetAdapter()
	if err != nil {
		return err
	}
	return adapter.CleanupPartitions(imgPathAbs)
}

// MountFilesystem mounts a filesystem
func MountFilesystem(partitionDevice, mountDir string) error {
	adapter, err := GetAdapter()
	if err != nil {
		return err
	}
	return adapter.MountFilesystem(partitionDevice, mountDir)
}

// UnmountFilesystem unmounts a filesystem
func UnmountFilesystem(mountDir string) error {
	adapter, err := GetAdapter()
	if err != nil {
		return err
	}
	return adapter.UnmountFilesystem(mountDir)
}

// WriteToImageFile writes content to a file in the mounted image
func WriteToImageFile(mountDir, relativePath string, content []byte, perm fs.FileMode) error {
	adapter, err := GetAdapter()
	if err != nil {
		return err
	}
	return adapter.WriteToImageFile(mountDir, relativePath, content, perm)
}

// CopyFileToImage copies a local file to the mounted image
func CopyFileToImage(mountDir, localSourcePath, relativeDestPath string) error {
	adapter, err := GetAdapter()
	if err != nil {
		return err
	}
	return adapter.CopyFileToImage(mountDir, localSourcePath, relativeDestPath)
}

// MkdirInImage creates a directory in the mounted image
func MkdirInImage(mountDir, relativePath string, perm fs.FileMode) error {
	adapter, err := GetAdapter()
	if err != nil {
		return err
	}
	return adapter.MkdirInImage(mountDir, relativePath, perm)
}

// ChmodInImage changes file permissions in the mounted image
func ChmodInImage(mountDir, relativePath string, perm fs.FileMode) error {
	adapter, err := GetAdapter()
	if err != nil {
		return err
	}
	return adapter.ChmodInImage(mountDir, relativePath, perm)
}

// ApplyNetworkConfig applies network configuration to the mounted image
func ApplyNetworkConfig(mountDir string, hostname string, ipCIDR string, gateway string, dnsServers []string) error {
	adapter, err := GetAdapter()
	if err != nil {
		return err
	}
	return adapter.ApplyNetworkConfig(mountDir, hostname, ipCIDR, gateway, dnsServers)
}

// RecompressImageXZ compresses a disk image with XZ
func RecompressImageXZ(modifiedImgPath, finalXZPath string) error {
	adapter, err := GetAdapter()
	if err != nil {
		return err
	}
	return adapter.RecompressImageXZ(modifiedImgPath, finalXZPath)
}

// ExecuteFileOperations executes a batch of file operations
func ExecuteFileOperations(params ExecuteFileOperationsParams) error {
	adapter, err := GetAdapter()
	if err != nil {
		return err
	}
	return adapter.ExecuteFileOperations(params)
}

// DockerAdapter returns the Docker adapter from the global adapter
func DockerAdapter() *docker.DockerAdapter {
	if platform.IsLinux() {
		// No Docker adapter needed on Linux
		return nil
	}

	adapter, err := GetAdapter()
	if err != nil {
		return nil
	}

	// Try to extract the Docker adapter from the adapter
	// This is a bit of a hack to maintain backward compatibility
	if adapterImpl, ok := adapter.(*imageOpsAdapter); ok && adapterImpl != nil {
		return adapterImpl.dockerAdapter
	}

	return nil
}
