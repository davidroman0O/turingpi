// Package tools provides interfaces for TuringPi tools
package tools

import (
	"context"
	"io"
	"io/fs"
	"time"

	"github.com/davidroman0O/turingpi/pkg/v2/bmc"
	"github.com/davidroman0O/turingpi/pkg/v2/cache"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
	imageops "github.com/davidroman0O/turingpi/pkg/v2/image"
	"github.com/davidroman0O/turingpi/pkg/v2/node"
)

// ToolProvider is the central interface for accessing all TuringPi tools
type ToolProvider interface {
	// GetBMCTool returns the BMC tool
	GetBMCTool() BMCTool

	// GetNodeTool returns the node tool
	GetNodeTool() NodeTool

	// GetImageTool returns the image tool
	GetImageTool() ImageTool

	// GetContainerTool returns the container tool
	GetContainerTool() ContainerTool

	// GetCacheTool returns the cache tool
	GetCacheTool() CacheTool

	// GetFSTool returns the filesystem tool
	GetFSTool() FSTool
}

// BMCTool provides operations for interacting with the Board Management Controller
type BMCTool interface {
	// GetPowerStatus retrieves the power status of a specific node
	GetPowerStatus(ctx context.Context, nodeID int) (*bmc.PowerStatus, error)

	// PowerOn turns on a specific node
	PowerOn(ctx context.Context, nodeID int) error

	// PowerOff turns off a specific node
	PowerOff(ctx context.Context, nodeID int) error

	// Reset performs a hard reset on a specific node
	Reset(ctx context.Context, nodeID int) error

	// GetInfo retrieves information about the BMC
	GetInfo(ctx context.Context) (*bmc.BMCInfo, error)

	// Reboot reboots the BMC chip
	Reboot(ctx context.Context) error

	// UpdateFirmware updates the BMC firmware
	UpdateFirmware(ctx context.Context, firmwarePath string) error
}

// NodeTool provides operations for interacting with compute nodes
type NodeTool interface {
	// ExecuteCommand runs a non-interactive command on the target node via SSH
	ExecuteCommand(ctx context.Context, nodeID int, command string) (stdout string, stderr string, err error)

	// ExpectAndSend performs a sequence of expect/send interactions over an SSH session
	ExpectAndSend(ctx context.Context, nodeID int, steps []node.InteractionStep, timeout time.Duration) (string, error)

	// CopyFile copies a file to or from the node
	CopyFile(ctx context.Context, nodeID int, localPath, remotePath string, toNode bool) error

	// GetInfo retrieves detailed information about the node
	GetInfo(ctx context.Context, nodeID int) (*node.NodeInfo, error)
}

// ImageTool provides operations for manipulating OS images
type ImageTool interface {
	// PrepareImage prepares an image with the given options
	PrepareImage(ctx context.Context, opts imageops.PrepareImageOptions) error

	// MapPartitions maps partitions in a disk image
	MapPartitions(ctx context.Context, imgPath string) (string, error)

	// UnmapPartitions unmaps partitions in a disk image
	UnmapPartitions(ctx context.Context, imgPath string) error

	// MountFilesystem mounts a filesystem
	MountFilesystem(ctx context.Context, device, mountDir string) error

	// UnmountFilesystem unmounts a filesystem
	UnmountFilesystem(ctx context.Context, mountDir string) error

	// DecompressImageXZ decompresses an XZ-compressed disk image
	DecompressImageXZ(ctx context.Context, sourceXZ, targetImg string) (string, error)

	// CompressImageXZ compresses a disk image with XZ
	CompressImageXZ(ctx context.Context, sourceImg, targetXZ string) error

	// WriteFile writes content to a file in the mounted image
	WriteFile(ctx context.Context, mountDir, relativePath string, content []byte, perm fs.FileMode) error

	// CopyFile copies a file to the mounted image
	CopyFile(ctx context.Context, mountDir, sourcePath, destPath string) error
}

// ContainerTool provides operations for managing containers
type ContainerTool interface {
	// CreateContainer creates a new container
	CreateContainer(ctx context.Context, config container.ContainerConfig) (container.Container, error)

	// RunCommand executes a command in a container
	RunCommand(ctx context.Context, containerID string, cmd []string) (string, error)

	// CopyToContainer copies a file or directory to a container
	CopyToContainer(ctx context.Context, containerID, hostPath, containerPath string) error

	// CopyFromContainer copies a file or directory from a container
	CopyFromContainer(ctx context.Context, containerID, containerPath, hostPath string) error

	// RemoveContainer removes a container
	RemoveContainer(ctx context.Context, containerID string) error
}

// CacheTool provides operations for caching data
type CacheTool interface {
	// Put stores content in the cache with associated metadata
	Put(ctx context.Context, key string, metadata cache.Metadata, reader io.Reader) (*cache.Metadata, error)

	// Get retrieves content and metadata from the cache
	Get(ctx context.Context, key string) (*cache.Metadata, io.ReadCloser, error)

	// Exists checks if an item exists in the cache
	Exists(ctx context.Context, key string) (bool, error)

	// List returns metadata for all items matching the filter tags
	List(ctx context.Context, filterTags map[string]string) ([]cache.Metadata, error)

	// Delete removes an item from the cache
	Delete(ctx context.Context, key string) error
}

// FSTool provides filesystem operations
type FSTool interface {
	// CreateDir creates a directory
	CreateDir(path string, perm fs.FileMode) error

	// WriteFile writes content to a file
	WriteFile(path string, content []byte, perm fs.FileMode) error

	// ReadFile reads a file's content
	ReadFile(path string) ([]byte, error)

	// FileExists checks if a file exists
	FileExists(path string) bool

	// CopyFile copies a file
	CopyFile(src, dst string) error

	// RemoveFile removes a file
	RemoveFile(path string) error

	// CalculateFileHash computes a hash for a file
	CalculateFileHash(path string) (string, error)
}
