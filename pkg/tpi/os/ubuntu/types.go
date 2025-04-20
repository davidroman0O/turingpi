package ubuntu

import (
	"os"
	"time"
)

// CacheKeyProvider provides methods to generate cache keys
type CacheKeyProvider interface {
	// CacheKey returns a unique key for caching based on configuration
	CacheKey() string
}

// BaseConfig holds common configuration fields shared across various configurations.
type BaseConfig struct {
	// Version specifies the Ubuntu version to use
	Version string
	// Key is a user-defined unique identifier for this configuration
	// Used for caching and referencing this specific configuration
	Key string
	// Tags are arbitrary key-value pairs for metadata
	Tags map[string]string
	// Force flags whether to force an operation regardless of cache
	Force bool
}

// CacheKey generates a cache key from the base configuration
func (c *BaseConfig) CacheKey() string {
	if c == nil {
		return ""
	}
	if c.Key != "" {
		return c.Key
	}
	return c.Version
}

// RuntimeConfig holds configuration related to runtime customization.
// This can be embedded into other config types that need runtime customization.
type RuntimeConfig struct {
	// RuntimeCustomizationFunc allows for custom runtime operations
	// It provides access to both local and remote runtime environments
	RuntimeCustomizationFunc func(local LocalRuntime, remote UbuntuRuntime) error
}

// NetworkingConfig holds network-related configuration for Ubuntu.
type NetworkingConfig struct {
	// StaticIP is the static IP address with CIDR notation
	StaticIP string
	// Gateway is the network gateway address
	Gateway string
	// DNSServers is a list of DNS server IP addresses
	DNSServers []string
	// Hostname is the node hostname
	Hostname string
}

// LocalRuntime defines the interface for local system operations
type LocalRuntime interface {
	// ReadFile reads a file from the local filesystem
	ReadFile(localPath string) ([]byte, error)

	// WriteFile writes data to the local filesystem
	WriteFile(localPath string, data []byte, perm os.FileMode) error

	// RunCommand executes a command on the local system
	RunCommand(command string, timeout time.Duration) (stdout, stderr string, err error)

	// CopyFile transfers a file between the local and remote system
	CopyFile(localPath, remotePath string, toRemote bool) error
}

// UbuntuRuntime defines the interface for remote Ubuntu system operations
type UbuntuRuntime interface {
	// RunCommand executes a command on the remote Ubuntu system
	RunCommand(command string, timeout time.Duration) (stdout, stderr string, err error)

	// CopyFile transfers a file between the local and remote system
	CopyFile(localPath, remotePath string, toRemote bool) error
}
