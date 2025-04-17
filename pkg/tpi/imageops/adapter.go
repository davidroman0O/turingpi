package imageops

import (
	"github.com/davidroman0O/turingpi/pkg/tpi/docker"
	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// ImageOpsAdapter defines the interface for image operations
type ImageOpsAdapter interface {
	// PrepareImage decompresses a disk image, modifies it with network settings, and recompresses it
	PrepareImage(opts PrepareImageOptions) (string, error)
	// InitDockerConfig initializes the Docker configuration for cross-platform operations
	InitDockerConfig(sourceDir, tempDir, outputDir string) error
	// GetDockerAdapter returns the underlying Docker adapter
	GetDockerAdapter() *docker.DockerAdapter
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

// imageOpsAdapter implements the ImageOpsAdapter interface
type imageOpsAdapter struct {
	dockerConfig      *platform.DockerExecutionConfig
	dockerContainerID string
	dockerAdapter     *docker.DockerAdapter
}

// NewImageOpsAdapter creates a new instance of ImageOpsAdapter
func NewImageOpsAdapter() ImageOpsAdapter {
	return &imageOpsAdapter{}
}
