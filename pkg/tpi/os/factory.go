package os

import (
	"fmt"
	"sync"

	"github.com/davidroman0O/turingpi/pkg/tpi"
)

// Provider defines the interface for OS-specific functionality providers.
// Each supported operating system must implement this interface to provide
// the necessary builders and installers.
type Provider interface {
	// NewImageBuilder creates a new image builder for the specified node.
	NewImageBuilder(nodeID tpi.NodeID) ImageBuilder

	// NewOSInstaller creates a new OS installer for the specified node.
	NewOSInstaller(nodeID tpi.NodeID) OSInstaller

	// NewPostInstaller creates a new post-installation configurator for the specified node.
	NewPostInstaller(nodeID tpi.NodeID) PostInstaller
}

// ImageBuilder defines the interface for customizing OS images.
type ImageBuilder interface {
	// Configure accepts an OS-specific configuration struct.
	Configure(config interface{}) error

	// CheckCache checks if an item exists for the configured CacheKey.
	// Returns true and metadata-derived ImageResult if key found in remote cache.
	CheckCache(ctx tpi.Context, cluster tpi.Cluster) (exists bool, result tpi.ImageResult, err error)

	// Run executes the image customization process.
	Run(ctx tpi.Context, cluster tpi.Cluster) (tpi.ImageResult, error)
}

// OSInstaller defines the interface for installing an OS on a node.
type OSInstaller interface {
	// Configure accepts an OS-specific installation configuration struct.
	Configure(config interface{}) error

	// UsingImage specifies the image to install.
	UsingImage(imageResult tpi.ImageResult) OSInstaller

	// Run executes the OS installation process.
	Run(ctx tpi.Context, cluster tpi.Cluster) error
}

// PostInstaller defines the interface for post-installation configuration.
type PostInstaller interface {
	// Configure accepts an OS-specific post-installation configuration struct.
	Configure(config interface{}) error

	// Run executes the post-installation configuration process.
	Run(ctx tpi.Context, cluster tpi.Cluster) error
}

var (
	providersMu sync.RWMutex
	providers   = make(map[OSIdentifier]Provider)
)

// RegisterProvider registers an OS provider for a specific OS identifier.
// This should be called during package initialization of OS-specific packages.
func RegisterProvider(id OSIdentifier, provider Provider) {
	providersMu.Lock()
	defer providersMu.Unlock()

	if provider == nil {
		panic("os: RegisterProvider provider is nil")
	}

	if _, exists := providers[id]; exists {
		panic(fmt.Sprintf("os: RegisterProvider called twice for OS %v", id))
	}

	providers[id] = provider
}

// GetProvider returns the registered provider for a specific OS identifier.
// Returns an error if no provider is registered for the given identifier.
func GetProvider(id OSIdentifier) (Provider, error) {
	providersMu.RLock()
	defer providersMu.RUnlock()

	if provider, exists := providers[id]; exists {
		return provider, nil
	}

	return nil, fmt.Errorf("os: no provider registered for OS %v", id)
}

// ListProviders returns a list of all registered OS identifiers.
// This can be useful for displaying available options to users.
func ListProviders() []OSIdentifier {
	providersMu.RLock()
	defer providersMu.RUnlock()

	var list []OSIdentifier
	for id := range providers {
		list = append(list, id)
	}
	return list
}
