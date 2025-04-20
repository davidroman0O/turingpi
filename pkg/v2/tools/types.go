package tools

import (
	"context"
	"io"
	"io/fs"
	"time"

	"github.com/davidroman0O/turingpi/pkg/v2/bmc"
	"github.com/davidroman0O/turingpi/pkg/v2/cache"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/node"
)

// BMCTool provides an interface for interacting with the BMC (Board Management Controller)
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
	// ExecuteCommand executes a BMC-specific command
	ExecuteCommand(ctx context.Context, command string) (stdout string, stderr string, err error)
}

// ContainerTool provides an interface for working with containers
type ContainerTool interface {
	// CreateContainer creates a new container
	CreateContainer(ctx context.Context, config container.ContainerConfig) (container.Container, error)
	// GetContainer retrieves a container by ID
	GetContainer(ctx context.Context, containerID string) (container.Container, error)
	// ListContainers returns all managed containers
	ListContainers(ctx context.Context) ([]container.Container, error)
	// StartContainer starts a container
	StartContainer(ctx context.Context, containerID string) error
	// StopContainer stops a container
	StopContainer(ctx context.Context, containerID string) error
	// KillContainer forcefully stops a container
	KillContainer(ctx context.Context, containerID string) error
	// PauseContainer pauses a container
	PauseContainer(ctx context.Context, containerID string) error
	// UnpauseContainer unpauses a container
	UnpauseContainer(ctx context.Context, containerID string) error
	// RunCommand executes a command in a container
	RunCommand(ctx context.Context, containerID string, cmd []string) (string, error)
	// RunDetachedCommand executes a command in a detached mode
	RunDetachedCommand(ctx context.Context, containerID string, cmd []string) error
	// CopyToContainer copies a file or directory to a container
	CopyToContainer(ctx context.Context, containerID, hostPath, containerPath string) error
	// CopyFromContainer copies a file or directory from a container
	CopyFromContainer(ctx context.Context, containerID, containerPath, hostPath string) error
	// GetContainerLogs returns container logs
	GetContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error)
	// WaitForContainer waits for the container to exit
	WaitForContainer(ctx context.Context, containerID string) (int, error)
	// RemoveContainer removes a container
	RemoveContainer(ctx context.Context, containerID string) error
	// RemoveAllContainers removes all managed containers
	RemoveAllContainers(ctx context.Context) error
	// GetContainerStats returns container statistics
	GetContainerStats(ctx context.Context, containerID string) (*container.ContainerState, error)
	// CloseRegistry releases all resources and removes all containers
	CloseRegistry() error
}

// OperationsTool provides an interface for disk image operations
type OperationsTool interface {
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
	// ReadFile reads a file from the mounted image
	ReadFile(ctx context.Context, mountDir, relativePath string) ([]byte, error)
	// CopyToDevice copies an image to a device
	CopyToDevice(ctx context.Context, imagePath, device string) error
	// ResizePartition resizes the last partition of a device to fill available space
	ResizePartition(ctx context.Context, device string) error
	// ValidateImage validates that an image file exists and is a valid disk image
	ValidateImage(ctx context.Context, imagePath string) error
	// ExtractBootFiles extracts kernel and initrd files from a mounted boot partition
	ExtractBootFiles(ctx context.Context, bootMountPoint, outputDir string) (string, string, error)
	// ApplyDTBOverlay applies a device tree overlay to a mounted boot partition
	ApplyDTBOverlay(ctx context.Context, bootMountPoint, dtbOverlayPath string) error
	// ApplyNetworkConfig applies network configuration to a mounted system
	ApplyNetworkConfig(ctx context.Context, mountDir, hostname, ipCIDR, gateway string, dnsServers []string) error
	// DecompressTarGZ decompresses a tar.gz archive to a directory
	DecompressTarGZ(ctx context.Context, sourceTarGZ, outputDir string) error
	// CompressTarGZ compresses a directory to a tar.gz archive
	CompressTarGZ(ctx context.Context, sourceDir, outputTarGZ string) error
	// DecompressGZ decompresses a GZ-compressed file
	DecompressGZ(ctx context.Context, sourceGZ, outputDir string) (string, error)
	// CompressGZ compresses a file using GZ compression
	CompressGZ(ctx context.Context, sourcePath, outputGZ string) error
}

// NodeTool provides an interface for interacting with nodes
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

// ToolProvider provides access to all the tools
type ToolProvider interface {
	// GetBMCTool returns the BMC tool
	GetBMCTool() BMCTool
	// GetNodeTool returns the node tool
	GetNodeTool() NodeTool
	// GetImageTool returns the image tool
	GetImageTool() OperationsTool
	// GetContainerTool returns the container tool
	GetContainerTool() ContainerTool
	// GetLocalCache returns the local filesystem cache
	GetLocalCache() *cache.FSCache
	// GetRemoteCache returns the remote SSH cache
	GetRemoteCache() *cache.SSHCache
}
