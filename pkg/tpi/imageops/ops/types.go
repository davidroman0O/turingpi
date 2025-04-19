// Package ops provides file operation types and implementations for image manipulation
package ops

import (
	"io/fs"

	"github.com/docker/docker/api/types/mount"
)

// DockerConfig represents Docker-specific configuration
type DockerConfig struct {
	Image           string
	ContainerName   string
	Mounts          []mount.Mount
	WorkingDir      string
	NetworkDisabled bool
	SourceDir       string
	TempDir         string
	OutputDir       string
}

// PrepareImageOptions contains all parameters needed to prepare an image
type PrepareImageOptions struct {
	SourceImgXZ    string   // Path to the source compressed image
	NodeNum        int      // Node number (used for default hostname if needed)
	IPAddress      string   // IP address without CIDR
	IPCIDRSuffix   string   // CIDR suffix (e.g., "/24")
	Hostname       string   // Hostname to set
	Gateway        string   // Gateway IP address
	DNSServers     []string // List of DNS server IPs
	OutputDir      string   // Directory to store output image
	TempDir        string   // Directory for temporary processing
	KeepTempFiles  bool     // Whether to keep temporary files for debugging
	VerifyChecksum bool     // Whether to verify checksums after operations
}

// Operation represents a single file operation to be performed
type Operation interface {
	Type() string
	Execute(mountDir string) error
	Verify(mountDir string) error
}

// WriteOperation represents a file write operation
type WriteOperation struct {
	Path     string
	Content  []byte
	FileMode fs.FileMode
}

// CopyOperation represents a file copy operation
type CopyOperation struct {
	SourcePath string
	DestPath   string
}

// MkdirOperation represents a directory creation operation
type MkdirOperation struct {
	Path     string
	FileMode fs.FileMode
}

// ChmodOperation represents a permission change operation
type ChmodOperation struct {
	Path     string
	FileMode fs.FileMode
}

// ExecuteParams contains parameters for file operations
type ExecuteParams struct {
	MountDir    string
	Operations  []Operation
	VerifyWrite bool // Whether to verify file contents after write
}
