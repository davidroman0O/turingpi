package tpi

import (
	"context"
	"os"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/bmc"
	"github.com/davidroman0O/turingpi/pkg/tpi/imageops"
	"github.com/davidroman0O/turingpi/pkg/tpi/state"
)

// Import NodeID and BoardType from state package
type NodeID = state.NodeID
type BoardType = state.BoardType

// Node ID constants - defined here for backward compatibility
// For new code, prefer to import these directly from the state package
const (
	Node1 NodeID = 1
	Node2 NodeID = 2
	Node3 NodeID = 3
	Node4 NodeID = 4

	// NOTE: Board type constants (RK1, CM4) should be imported directly from state package:
	// import "github.com/davidroman0O/turingpi/pkg/tpi/state"
	// state.RK1, state.CM4
)

// TPIConfig holds the overall configuration for the Turing Pi cluster
// and the execution environment for the tpi tool.
// It can be populated directly or potentially loaded from a configuration file (e.g., YAML).
type TPIConfig struct {
	// IP address of the Turing Pi Board Management Controller (BMC).
	// This is required for interacting with the board's hardware functions.
	IP string `yaml:"ip"`

	// Credentials for connecting to the BMC
	BMCUser     string `yaml:"bmcUser,omitempty"`
	BMCPassword string `yaml:"bmcPassword,omitempty"` // Consider more secure methods later

	// Path to the directory used for caching prepared images and storing the execution state file.
	// If empty, it defaults to a standard user cache directory (e.g., ~/.cache/tpi).
	// This directory is used for the state file and as the default location for prepared images
	// when no explicit output directory is specified with WithOutputDirectory.
	CacheDir string `yaml:"cacheDir,omitempty"`

	// Path to a temporary local folder specifically for image preparation processing.
	// This directory is used for all temporary files during image processing, including:
	// - Decompressed image files (which can be very large)
	// - Mount points for filesystems
	// - Partition mapping
	//
	// If empty, the system's temporary directory will be used.
	// It's recommended to set this to a location with ample disk space for large image files.
	// NOTE: This is NOT where final prepared images are stored - those go to either:
	// 1. The directory specified by WithOutputDirectory(), or
	// 2. The CacheDir if no output directory is explicitly specified
	PrepImageDir string `yaml:"prepImageDir,omitempty"`

	// Name of the state file within CacheDir.
	// If empty, it defaults to "tpi_state.json".
	StateFileName string `yaml:"stateFileName,omitempty"`

	// --- Node Configurations ---
	// Configuration for each node slot. Use pointers (*NodeConfig) to allow
	// users to configure only the nodes they intend to manage.
	// At least one node configuration must be provided.
	Node1 *NodeConfig `yaml:"node1,omitempty"`
	Node2 *NodeConfig `yaml:"node2,omitempty"`
	Node3 *NodeConfig `yaml:"node3,omitempty"`
	Node4 *NodeConfig `yaml:"node4,omitempty"`

	// Future: Configuration for credentials access (SSH user, password, key paths).
	// TODO: Define Credentials config structure
	// Credentials map[NodeID]CredentialConfig `yaml:"credentials,omitempty"`
}

// NodeConfig holds the specific configuration for a single compute node.
// This information is primarily used for configuring the OS network
// and identifying the node for post-install operations.
type NodeConfig struct {
	// Static IP assigned AFTER configuration. Used for post-install SSH. REQUIRED.
	IP string `yaml:"ip"`

	// Board type installed in this node slot.
	// This is required to select appropriate flashing tools and potentially OS configurations. REQUIRED.
	Board BoardType `yaml:"board"`

	// Optional: MAC Address of the node's network interface.
	// Can be useful for identification purposes, especially before the static IP is configured.
	MacAddress string `yaml:"macAddress,omitempty"`

	// Future: Node-specific overrides for gateway, DNS etc. if needed.
}

// --- Workflow Context & Data --- //

// Cluster represents the Turing Pi cluster and provides methods for interacting with it.
// This interface hides the implementation details of the executor and provides a cleaner API.
type Cluster interface {
	// GetNodeConfig returns the configuration for a specific node
	GetNodeConfig(nodeID NodeID) *NodeConfig

	// GetStateManager returns the state manager instance
	GetStateManager() state.Manager

	// GetCacheDir returns the cache directory path
	GetCacheDir() string

	// GetPrepImageDir returns the image preparation directory path
	GetPrepImageDir() string

	// GetBMCSSHConfig returns the SSH configuration for BMC access
	GetBMCSSHConfig() bmc.SSHConfig
}

// Context extends the standard context.Context with TPI-specific fields and methods.
type Context interface {
	context.Context // Embed standard context.Context to get all its methods

	// Log returns the context's logger
	Log() Logger
}

// Node represents the specific compute node being processed within the workflow function.
// It combines static configuration with dynamic/derived information.
type Node struct {
	ID     NodeID      // The identifier (Node1, Node2, etc.)
	Config *NodeConfig // Reference to the static configuration for this node

	// --- Derived/Calculated values needed for NetworkConfig --- //
	// These would be populated by the Run orchestrator before calling the workflow func.
	// TODO: Define logic to derive these values (maybe in Run orchestrator)
	IPAddress  string   // e.g., "192.168.1.100" (Without CIDR)
	Hostname   string   // e.g., "tpi-node1"
	Gateway    string   // e.g., "192.168.1.1"
	DNSServers []string // e.g., ["1.1.1.1", "8.8.8.8"]
}

// --- Builder Configuration Structs --- //

// NetworkConfig defines the network settings to be applied during image customization.
// These values are typically derived from the specific Node context.
type NetworkConfig struct {
	IPCIDR     string // e.g., "192.168.1.100/24"
	Hostname   string
	Gateway    string
	DNSServers []string // Use string slice for multiple servers
}

// OSInstallConfig holds generic OS installation parameters that might apply across different OS types.
type OSInstallConfig struct {
	// Public SSH keys to add to the default user's authorized_keys file.
	// Support depends on the image (e.g., cloud-init).
	SSHKeys []string `yaml:"sshKeys,omitempty"`
	// Add other generic options like timezone, locale?
}

// UbuntuInstallConfig holds Ubuntu-specific installation parameters.
type UbuntuInstallConfig struct {
	// Password for the default user (e.g., 'ubuntu').
	// Use with caution; SSH keys are preferred.
	InitialUserPassword string `yaml:"initialUserPassword,omitempty"`
	// Add other Ubuntu-specific options if needed (e.g., specific package selections?)
}

// --- Phase Results --- //

// ImageResult holds information about the customized image created by Phase 1.
type ImageResult struct {
	// Absolute path to the final, potentially recompressed, customized image file.
	ImagePath string
	// Board type the image was prepared for.
	Board BoardType
	// Hash of the inputs used to create this image, for state checking.
	InputHash string
}

// --- Callback Helper Types --- //

// ImageModifier provides methods to manipulate the mounted filesystem during the
// image preparation phase.
type ImageModifier = imageops.ImageModifier

// LocalRuntime provides methods to interact with the local filesystem
// (the machine running the tpi tool) during the Phase 3 post-install callback.
type LocalRuntime interface {
	// CopyFile copies a file between the local machine and the remote node.
	// If toRemote is true, localSourcePath -> remoteDestPath.
	// If toRemote is false, remoteSourcePath -> localDestPath.
	CopyFile(localPath, remotePath string, toRemote bool) error

	// ReadFile reads a file from the local filesystem.
	ReadFile(localPath string) ([]byte, error)
	// WriteFile writes data to the local filesystem.
	WriteFile(localPath string, data []byte, perm os.FileMode) error
	// RunCommand executes a command on the local machine.
	RunCommand(command string, timeout time.Duration) (stdout, stderr string, err error)
}

// UbuntuRuntime provides methods to interact with a remote node running Ubuntu
// during the Phase 3 post-install callback. Assumes SSH connection is established.
type UbuntuRuntime interface {
	// RunCommand executes a non-interactive command on the remote node.
	RunCommand(command string, timeout time.Duration) (stdout, stderr string, err error)

	// CopyFile copies a file between the local machine and the remote node.
	CopyFile(localPath, remotePath string, toRemote bool) error

	// TODO: Add more helpers? (e.g., user management, specific config file editing)
}
