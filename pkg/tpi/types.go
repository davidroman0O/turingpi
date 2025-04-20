package tpi

import (
	"os"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/bmc"
	"github.com/davidroman0O/turingpi/pkg/tpi/cache"
	"github.com/davidroman0O/turingpi/pkg/tpi/imageops"
	"github.com/davidroman0O/turingpi/pkg/tpi/state"
)

// NodeID represents a node identifier
type NodeID = state.NodeID

// Node ID constants
const (
	Node1 NodeID = 1
	Node2 NodeID = 2
	Node3 NodeID = 3
	Node4 NodeID = 4

	// Special node ID for image preparation
	NodePrepareID NodeID = 0
)

// Board represents a supported board type
type Board = state.BoardType

// OSIdentifier uniquely identifies an operating system and its version
type OSIdentifier struct {
	Type    string // e.g., "ubuntu"
	Version string // e.g., "22.04"
}

// TPIConfig holds the global configuration for the Turing Pi toolkit
type TPIConfig struct {
	// Cache directories
	CacheDir      string `yaml:"cacheDir,omitempty"`      // Base directory for all caching
	PrepImageDir  string `yaml:"prepImageDir,omitempty"`  // Directory for image preparation
	StateFileName string `yaml:"stateFileName,omitempty"` // Name of the state file

	// Global settings
	DefaultBoard Board  // Default board type for nodes without explicit config
	LogLevel     string // Logging verbosity level

	// BMC configuration
	BMCIP       string `yaml:"bmcIP"`       // IP address of the BMC
	BMCUser     string `yaml:"bmcUser"`     // Username for BMC access
	BMCPassword string `yaml:"bmcPassword"` // Password for BMC access

	// Node configurations
	Node1 *NodeConfig `yaml:"node1,omitempty"`
	Node2 *NodeConfig `yaml:"node2,omitempty"`
	Node3 *NodeConfig `yaml:"node3,omitempty"`
	Node4 *NodeConfig `yaml:"node4,omitempty"`
}

// NodeConfig holds the configuration for a single node
type NodeConfig struct {
	IP         string   `yaml:"ip"`                // Static IP for the node
	Board      Board    `yaml:"board"`             // Hardware board type
	MacAddress string   `yaml:"macAddress"`        // Optional MAC address
	Network    *Network `yaml:"network,omitempty"` // Network configuration
}

// Network holds network configuration for a node
type Network struct {
	Gateway    string   `yaml:"gateway"`    // Gateway IP address
	DNSServers []string `yaml:"dnsServers"` // DNS server addresses
}

// ImageResult represents the result of an image build operation
type ImageResult struct {
	ImagePath string // Path to the built image
	Board     Board  // Target board type
	InputHash string // Hash of input parameters used to build the image
}

// Context represents the execution context for operations
type Context interface {
	Deadline() (deadline time.Time, ok bool)
	Done() <-chan struct{}
	Err() error
	Value(key interface{}) interface{}
}

// Cluster represents a Turing Pi cluster
type Cluster interface {
	// GetNodeConfig returns the configuration for a specific node
	GetNodeConfig(nodeID NodeID) *NodeConfig

	// GetStateManager returns the state management interface
	GetStateManager() StateManager

	// GetCacheDir returns the cache directory path
	GetCacheDir() string

	// GetPrepImageDir returns the image preparation directory path
	GetPrepImageDir() string

	// GetBMCSSHConfig returns the SSH configuration for BMC access
	GetBMCSSHConfig() bmc.SSHConfig

	// GetRemoteCache returns the remote cache interface
	GetRemoteCache() cache.Cache

	// GetLocalCache returns the local cache interface
	GetLocalCache() cache.Cache

	// Cache returns the default cache interface (local)
	Cache() cache.Cache
}

// StateManager handles persistent state storage
type StateManager interface {
	// GetNodeState retrieves the state for a specific node
	GetNodeState(nodeID NodeID) (*NodeState, error)

	// UpdateNodeState updates the state for a specific node
	UpdateNodeState(state *NodeState) error
}

// NodeState represents the current state of a node
type NodeState struct {
	LastImageTime time.Time // When the last image was built
	LastImageHash string    // Hash of the last built image
	LastImagePath string    // Path to the last built image
	LastError     string    // Last error encountered, if any
}

// OSProvider provides OS-specific functionality
type OSProvider interface {
	// NewImageBuilder creates a new image builder for the specified node
	NewImageBuilder(nodeID NodeID) ImageBuilder

	// NewOSInstaller creates a new OS installer for the specified node
	NewOSInstaller(nodeID NodeID) OSInstaller

	// NewPostInstaller creates a new post-installation configurator
	NewPostInstaller(nodeID NodeID) PostInstaller
}

// ImageBuilder defines the interface for customizing OS images
type ImageBuilder interface {
	// Configure accepts an OS-specific configuration struct
	Configure(config interface{}) error

	// Run executes the image customization process
	Run(ctx Context, cluster Cluster) (ImageResult, error)
}

// OSInstaller defines the interface for installing an OS on a node
type OSInstaller interface {
	// Configure accepts an OS-specific installation configuration
	Configure(config interface{}) error

	// UsingImage specifies the image to install
	UsingImage(imageResult ImageResult) OSInstaller

	// Run executes the OS installation process
	Run(ctx Context, cluster Cluster) error
}

// PostInstaller defines the interface for post-installation configuration
type PostInstaller interface {
	// Configure accepts an OS-specific post-installation configuration
	Configure(config interface{}) error

	// Run executes the post-installation configuration process
	Run(ctx Context, cluster Cluster) error
}

// Runtime provides basic operations for interacting with a node
type Runtime interface {
	// RunCommand executes a command on the node
	RunCommand(command string, timeout time.Duration) (stdout, stderr string, err error)

	// CopyFile transfers a file between the local and remote system
	CopyFile(localPath, remotePath string, toRemote bool) error
}

// UbuntuRuntime extends Runtime with Ubuntu-specific operations
type UbuntuRuntime interface {
	Runtime
}

// LocalRuntime extends Runtime with local filesystem operations
type LocalRuntime interface {
	Runtime

	// ReadFile reads a file from the local filesystem
	ReadFile(localPath string) ([]byte, error)

	// WriteFile writes data to the local filesystem
	WriteFile(localPath string, data []byte, perm os.FileMode) error
}

// ImageModifier is an alias for imageops.ImageModifier
type ImageModifier = imageops.ImageModifier

// Node represents a node in the cluster
type Node struct {
	ID     NodeID
	Config *NodeConfig
}

// NetworkConfig holds network configuration for OS installation
type NetworkConfig struct {
	IPCIDR     string
	Hostname   string
	Gateway    string
	DNSServers []string
}

// OSInstallConfig holds generic OS installation configuration
type OSInstallConfig struct {
	SSHKeys []string
}

// UbuntuInstallConfig holds Ubuntu-specific installation configuration
type UbuntuInstallConfig struct {
	InitialUserPassword string
	OSInstallConfig
}

// Provider is the main interface for cluster operations
type Provider interface {
	// Run executes a workflow template for a node
	Run(template func(ctx Context, cluster Cluster, node Node) error) func(ctx Context, nodeID NodeID) error
}
