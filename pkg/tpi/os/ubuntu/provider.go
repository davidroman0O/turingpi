package ubuntu

import (
	"github.com/davidroman0O/turingpi/pkg/tpi"
	"github.com/davidroman0O/turingpi/pkg/tpi/os"
)

// Provider implements the os.Provider interface for Ubuntu.
type Provider struct{}

// init registers the Ubuntu provider with the factory.
func init() {
	// Register for Ubuntu 22.04 LTS
	os.RegisterProvider(os.OSIdentifier{
		Type:    "ubuntu",
		Version: V2204LTS,
	}, &Provider{})
}

// NewImageBuilder creates a new Ubuntu image builder for the specified node.
func (p *Provider) NewImageBuilder(nodeID tpi.NodeID) os.ImageBuilder {
	return NewImageBuilder(nodeID)
}

// NewOSInstaller creates a new Ubuntu OS installer for the specified node.
func (p *Provider) NewOSInstaller(nodeID tpi.NodeID) os.OSInstaller {
	installer := NewOSInstaller(nodeID)
	return installer
}

// NewPostInstaller creates a new Ubuntu post-installation configurator for the specified node.
func (p *Provider) NewPostInstaller(nodeID tpi.NodeID) os.PostInstaller {
	return NewPostInstaller(nodeID)
}

// Ensure Provider implements os.Provider interface
var _ os.Provider = (*Provider)(nil)
