// Package imageops provides functions for OS image preparation and modification.
package imageops

import (
	"io/fs"

	"github.com/davidroman0O/turingpi/pkg/tpi/internal/imageops"
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

// FileOperation defines the interface for file operations within the image
type FileOperation interface {
	Type() string
	Execute(mountDir string) error
}

// WriteOperation implements FileOperation for writing data to a file
type WriteOperation struct {
	RelativePath string
	Data         []byte
	Perm         fs.FileMode
}

func (op WriteOperation) Type() string {
	return "write"
}

func (op WriteOperation) Execute(mountDir string) error {
	internalOp := imageops.WriteOperation{
		RelativePath: op.RelativePath,
		Data:         op.Data,
		Perm:         op.Perm,
	}
	return internalOp.Execute(mountDir)
}

// CopyLocalOperation implements FileOperation for copying files
type CopyLocalOperation struct {
	LocalSourcePath  string
	RelativeDestPath string
}

func (op CopyLocalOperation) Type() string {
	return "copyLocal"
}

func (op CopyLocalOperation) Execute(mountDir string) error {
	internalOp := imageops.CopyLocalOperation{
		LocalSourcePath:  op.LocalSourcePath,
		RelativeDestPath: op.RelativeDestPath,
	}
	return internalOp.Execute(mountDir)
}

// MkdirOperation implements FileOperation for creating directories
type MkdirOperation struct {
	RelativePath string
	Perm         fs.FileMode
}

func (op MkdirOperation) Type() string {
	return "mkdir"
}

func (op MkdirOperation) Execute(mountDir string) error {
	internalOp := imageops.MkdirOperation{
		RelativePath: op.RelativePath,
		Perm:         op.Perm,
	}
	return internalOp.Execute(mountDir)
}

// ChmodOperation implements FileOperation for changing permissions
type ChmodOperation struct {
	RelativePath string
	Perm         fs.FileMode
}

func (op ChmodOperation) Type() string {
	return "chmod"
}

func (op ChmodOperation) Execute(mountDir string) error {
	internalOp := imageops.ChmodOperation{
		RelativePath: op.RelativePath,
		Perm:         op.Perm,
	}
	return internalOp.Execute(mountDir)
}

// ExecuteFileOperationsParams contains parameters for ExecuteFileOperations
type ExecuteFileOperationsParams struct {
	MountDir   string
	Operations []FileOperation
}

// NewImageOpsAdapter creates a new adapter for image operations.
func NewImageOpsAdapter(sourceDir, tempDir, outputDir string) (ImageOpsAdapter, error) {
	internalAdapter, err := imageops.NewImageOpsAdapter(sourceDir, tempDir, outputDir)
	if err != nil {
		return nil, err
	}

	return &imageOpsAdapterWrapper{
		internal: internalAdapter,
	}, nil
}

// imageOpsAdapterWrapper wraps the internal imageops adapter
type imageOpsAdapterWrapper struct {
	internal imageops.ImageOpsAdapter
}

// PrepareImage implements ImageOpsAdapter.PrepareImage
func (a *imageOpsAdapterWrapper) PrepareImage(opts PrepareImageOptions) (string, error) {
	internalOpts := imageops.PrepareImageOptions{
		SourceImgXZ:  opts.SourceImgXZ,
		NodeNum:      opts.NodeNum,
		IPAddress:    opts.IPAddress,
		IPCIDRSuffix: opts.IPCIDRSuffix,
		Hostname:     opts.Hostname,
		Gateway:      opts.Gateway,
		DNSServers:   opts.DNSServers,
		OutputDir:    opts.OutputDir,
		TempDir:      opts.TempDir,
	}
	return a.internal.PrepareImage(internalOpts)
}

// DecompressImageXZ implements ImageOpsAdapter.DecompressImageXZ
func (a *imageOpsAdapterWrapper) DecompressImageXZ(sourceImgXZAbs, tmpDir string) (string, error) {
	return a.internal.DecompressImageXZ(sourceImgXZAbs, tmpDir)
}

// MapPartitions implements ImageOpsAdapter.MapPartitions
func (a *imageOpsAdapterWrapper) MapPartitions(imgPathAbs string) (string, error) {
	return a.internal.MapPartitions(imgPathAbs)
}

// CleanupPartitions implements ImageOpsAdapter.CleanupPartitions
func (a *imageOpsAdapterWrapper) CleanupPartitions(imgPathAbs string) error {
	return a.internal.CleanupPartitions(imgPathAbs)
}

// MountFilesystem implements ImageOpsAdapter.MountFilesystem
func (a *imageOpsAdapterWrapper) MountFilesystem(partitionDevice, mountDir string) error {
	return a.internal.MountFilesystem(partitionDevice, mountDir)
}

// UnmountFilesystem implements ImageOpsAdapter.UnmountFilesystem
func (a *imageOpsAdapterWrapper) UnmountFilesystem(mountDir string) error {
	return a.internal.UnmountFilesystem(mountDir)
}

// ApplyNetworkConfig implements ImageOpsAdapter.ApplyNetworkConfig
func (a *imageOpsAdapterWrapper) ApplyNetworkConfig(mountDir, hostname, ipCIDR, gateway string, dnsServers []string) error {
	return a.internal.ApplyNetworkConfig(mountDir, hostname, ipCIDR, gateway, dnsServers)
}

// RecompressImageXZ implements ImageOpsAdapter.RecompressImageXZ
func (a *imageOpsAdapterWrapper) RecompressImageXZ(modifiedImgPath, finalXZPath string) error {
	return a.internal.RecompressImageXZ(modifiedImgPath, finalXZPath)
}

// ExecuteFileOperations implements ImageOpsAdapter.ExecuteFileOperations
func (a *imageOpsAdapterWrapper) ExecuteFileOperations(params ExecuteFileOperationsParams) error {
	// Convert our operations to internal operations
	internalOps := make([]imageops.FileOperation, len(params.Operations))
	for i, op := range params.Operations {
		switch o := op.(type) {
		case WriteOperation:
			internalOps[i] = imageops.WriteOperation{
				RelativePath: o.RelativePath,
				Data:         o.Data,
				Perm:         o.Perm,
			}
		case CopyLocalOperation:
			internalOps[i] = imageops.CopyLocalOperation{
				LocalSourcePath:  o.LocalSourcePath,
				RelativeDestPath: o.RelativeDestPath,
			}
		case MkdirOperation:
			internalOps[i] = imageops.MkdirOperation{
				RelativePath: o.RelativePath,
				Perm:         o.Perm,
			}
		case ChmodOperation:
			internalOps[i] = imageops.ChmodOperation{
				RelativePath: o.RelativePath,
				Perm:         o.Perm,
			}
		}
	}

	internalParams := imageops.ExecuteFileOperationsParams{
		MountDir:   params.MountDir,
		Operations: internalOps,
	}

	return a.internal.ExecuteFileOperations(internalParams)
}

// WriteToImageFile implements ImageOpsAdapter.WriteToImageFile
func (a *imageOpsAdapterWrapper) WriteToImageFile(mountDir, relativePath string, content []byte, perm fs.FileMode) error {
	return a.internal.WriteToImageFile(mountDir, relativePath, content, perm)
}

// CopyFileToImage implements ImageOpsAdapter.CopyFileToImage
func (a *imageOpsAdapterWrapper) CopyFileToImage(mountDir, localSourcePath, relativeDestPath string) error {
	return a.internal.CopyFileToImage(mountDir, localSourcePath, relativeDestPath)
}

// MkdirInImage implements ImageOpsAdapter.MkdirInImage
func (a *imageOpsAdapterWrapper) MkdirInImage(mountDir, relativePath string, perm fs.FileMode) error {
	return a.internal.MkdirInImage(mountDir, relativePath, perm)
}

// ChmodInImage implements ImageOpsAdapter.ChmodInImage
func (a *imageOpsAdapterWrapper) ChmodInImage(mountDir, relativePath string, perm fs.FileMode) error {
	return a.internal.ChmodInImage(mountDir, relativePath, perm)
}

// Cleanup implements ImageOpsAdapter.Cleanup
func (a *imageOpsAdapterWrapper) Cleanup() error {
	return a.internal.Cleanup()
}
