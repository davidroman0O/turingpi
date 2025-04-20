package tools

import (
	"fmt"
	"sync"

	"github.com/davidroman0O/turingpi/pkg/v2/bmc"
	"github.com/davidroman0O/turingpi/pkg/v2/cache"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/platform"
)

// TuringPiToolProvider is the central implementation of the ToolProvider interface
type TuringPiToolProvider struct {
	bmcTool         BMCTool
	nodeTool        NodeTool
	imageTool       OperationsTool
	containerTool   ContainerTool
	localCacheTool  LocalCacheTool
	remoteCacheTool RemoteCacheTool
	fsTool          FSTool
	mu              sync.RWMutex
}

// NewTuringPiToolProvider creates a new TuringPiToolProvider
func NewTuringPiToolProvider(config *TuringPiToolConfig) (*TuringPiToolProvider, error) {
	provider := &TuringPiToolProvider{}

	// Initialize tools
	if config.BMCExecutor != nil {
		provider.bmcTool = bmc.New(config.BMCExecutor)
	}

	if config.CacheDir != "" {
		fsCache, err := cache.NewFSCache(config.CacheDir)
		if err != nil {
			return nil, err
		}
		provider.localCacheTool = NewLocalCacheTool(fsCache)
	}

	// Container tools depend on platform
	if platform.DockerAvailable() {
		registry, err := container.NewDockerRegistry()
		if err == nil {
			provider.containerTool = NewContainerTool(registry)
		}
	}

	// Initialize operations-based image tool
	opsTool, err := NewOperationsTool(provider.containerTool)
	if err != nil {
		// Log the error but continue as this isn't critical
		fmt.Printf("Warning: Failed to initialize operations tool: %v\n", err)
	} else {
		provider.imageTool = opsTool
	}

	// Initialize node tool if BMC is available
	if provider.bmcTool != nil {
		provider.nodeTool = NewNodeTool(provider.bmcTool, config.NodeConfigs)
	}

	// Initialize remote cache if remote config is provided
	if config.RemoteCache != nil && provider.nodeTool != nil {
		sshConfig := cache.SSHConfig{
			Host:     config.RemoteCache.Host,
			Port:     22, // Default SSH port
			User:     config.RemoteCache.User,
			Password: config.RemoteCache.Password,
		}

		sshCache, err := cache.NewSSHCache(sshConfig, config.RemoteCache.RemotePath)
		if err == nil {
			provider.remoteCacheTool = NewRemoteCacheTool(
				config.RemoteCache.NodeID,
				provider.nodeTool,
				sshCache,
				config.RemoteCache.RemotePath,
			)
		} else {
			// Just log the error and continue without remote cache
			fmt.Printf("Failed to create remote cache: %v\n", err)
		}
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
func (p *TuringPiToolProvider) GetImageTool() OperationsTool {
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

// GetLocalCacheTool returns the local cache tool
func (p *TuringPiToolProvider) GetLocalCacheTool() LocalCacheTool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.localCacheTool
}

// GetRemoteCacheTool returns the remote cache tool
func (p *TuringPiToolProvider) GetRemoteCacheTool(nodeID int) RemoteCacheTool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.remoteCacheTool
}

// GetFSTool returns the filesystem tool
func (p *TuringPiToolProvider) GetFSTool() FSTool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.fsTool
}

// RemoteCacheConfig holds configuration for remote cache
type RemoteCacheConfig struct {
	// NodeID is the ID of the node where the cache is located
	NodeID int

	// Host is the hostname or IP address of the node
	Host string

	// User is the username for SSH
	User string

	// Password is the password for SSH
	Password string

	// RemotePath is the path on the remote system where cache will be stored
	RemotePath string
}

// TuringPiToolConfig holds configuration for the TuringPiToolProvider
type TuringPiToolConfig struct {
	// BMCExecutor is an executor for BMC commands
	BMCExecutor bmc.CommandExecutor

	// CacheDir is the directory for caching
	CacheDir string

	// NodeConfigs holds SSH configuration for nodes
	NodeConfigs map[int]*NodeConfig

	// RemoteCache holds configuration for the remote cache
	RemoteCache *RemoteCacheConfig
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
