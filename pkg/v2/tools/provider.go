package tools

import (
	"sync"

	"github.com/davidroman0O/turingpi/pkg/v2/bmc"
	"github.com/davidroman0O/turingpi/pkg/v2/cache"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/platform"
)

// TuringPiToolProvider is the central implementation of the ToolProvider interface
type TuringPiToolProvider struct {
	bmcTool       BMCTool
	nodeTool      NodeTool
	imageTool     ImageTool
	containerTool ContainerTool
	cacheTool     CacheTool
	fsTool        FSTool
	mu            sync.RWMutex
}

// NewTuringPiToolProvider creates a new TuringPiToolProvider
func NewTuringPiToolProvider(config *TuringPiToolConfig) (*TuringPiToolProvider, error) {
	provider := &TuringPiToolProvider{}

	// Initialize tools
	if config.BMCExecutor != nil {
		provider.bmcTool = NewBMCTool(config.BMCExecutor)
	}

	if config.CacheDir != "" {
		fsCache, err := cache.NewFSCache(config.CacheDir)
		if err != nil {
			return nil, err
		}
		provider.cacheTool = NewCacheTool(fsCache)
	}

	// Container tools depend on platform
	if platform.DockerAvailable() {
		registry, err := container.NewDockerRegistry()
		if err == nil {
			provider.containerTool = NewContainerTool(registry)
		}
	}

	// Initialize image tool
	provider.imageTool = NewImageTool(provider.containerTool)

	// Initialize node tool if BMC is available
	if provider.bmcTool != nil {
		provider.nodeTool = NewNodeTool(provider.bmcTool, config.NodeConfigs)
	}

	// Initialize filesystem tool
	provider.fsTool = NewFSTool()

	return provider, nil
}

// GetBMCTool returns the BMC tool
func (p *TuringPiToolProvider) GetBMCTool() BMCTool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.bmcTool
}

// GetNodeTool returns the node tool
func (p *TuringPiToolProvider) GetNodeTool() NodeTool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.nodeTool
}

// GetImageTool returns the image tool
func (p *TuringPiToolProvider) GetImageTool() ImageTool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.imageTool
}

// GetContainerTool returns the container tool
func (p *TuringPiToolProvider) GetContainerTool() ContainerTool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.containerTool
}

// GetCacheTool returns the cache tool
func (p *TuringPiToolProvider) GetCacheTool() CacheTool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.cacheTool
}

// GetFSTool returns the filesystem tool
func (p *TuringPiToolProvider) GetFSTool() FSTool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.fsTool
}

// TuringPiToolConfig holds configuration for the TuringPiToolProvider
type TuringPiToolConfig struct {
	// BMCExecutor is an executor for BMC commands
	BMCExecutor bmc.CommandExecutor

	// CacheDir is the directory for caching
	CacheDir string

	// NodeConfigs holds SSH configuration for nodes
	NodeConfigs map[int]*NodeConfig
}

// NodeConfig holds configuration for a node
type NodeConfig struct {
	// Host is the hostname or IP address of the node
	Host string

	// User is the username for SSH
	User string

	// Password is the password for SSH
	Password string
}
