package tpi

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/davidroman0O/turingpi/pkg/tpi/bmc"
	"github.com/davidroman0O/turingpi/pkg/tpi/cache"
	"github.com/davidroman0O/turingpi/pkg/tpi/state"
)

// TuringPiProvider implements the Provider interface and provides
// core functionality for managing a Turing Pi cluster.
type TuringPiProvider struct {
	config        TPIConfig     // The validated and defaulted configuration
	stateFilePath string        // Absolute path to the state file
	stateManager  state.Manager // State management interface
}

// Ensure TuringPiProvider implements Provider interface
var _ Provider = (*TuringPiProvider)(nil)

// NewTuringPi creates a new TuringPiProvider instance with the provided configuration.
func NewTuringPi(config TPIConfig) (*TuringPiProvider, error) {
	log.Println("Initializing Turing Pi Provider...")

	// Validate BMC configuration
	if config.BMCIP == "" {
		return nil, fmt.Errorf("TPIConfig validation failed: missing required field 'BMCIP'")
	}
	if config.BMCUser == "" {
		log.Println("BMCUser not specified, defaulting to 'root'")
		config.BMCUser = "root"
	}

	// Validate node configurations
	configuredNodes := 0
	for id, node := range map[NodeID]*NodeConfig{
		Node1: config.Node1,
		Node2: config.Node2,
		Node3: config.Node3,
		Node4: config.Node4,
	} {
		if node != nil {
			configuredNodes++
			if err := validateNodeConfig(*node, id); err != nil {
				return nil, err
			}
		}
	}

	if configuredNodes == 0 {
		return nil, fmt.Errorf("TPIConfig validation failed: at least one Node (Node1-Node4) must be configured")
	}

	// Set up cache directory
	if config.CacheDir == "" {
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			config.CacheDir = ".tpi_cache"
			log.Printf("Warning: Could not find user cache directory (%v), using default relative path: %s", err, config.CacheDir)
		} else {
			config.CacheDir = filepath.Join(userCacheDir, "tpi")
		}
	}

	// Ensure absolute path for cache directory
	absCacheDir, err := filepath.Abs(config.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for CacheDir '%s': %w", config.CacheDir, err)
	}
	config.CacheDir = absCacheDir

	// Create cache directory
	if err := os.MkdirAll(config.CacheDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory '%s': %w", config.CacheDir, err)
	}
	log.Printf("Using cache directory: %s", config.CacheDir)

	// Set up state file
	if config.StateFileName == "" {
		config.StateFileName = "tpi_state.json"
	}
	stateFilePath := filepath.Join(config.CacheDir, config.StateFileName)
	log.Printf("Using state file: %s", stateFilePath)

	// Initialize state manager
	stateMgr, err := state.NewFileStateManager(stateFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize state manager: %w", err)
	}

	provider := &TuringPiProvider{
		config:        config,
		stateFilePath: stateFilePath,
		stateManager:  stateMgr,
	}

	log.Println("Turing Pi Provider initialized successfully.")
	return provider, nil
}

// validateNodeConfig checks if a specific node's configuration is valid.
func validateNodeConfig(nodeCfg NodeConfig, nodeID NodeID) error {
	if nodeCfg.IP == "" {
		return fmt.Errorf("TPIConfig validation failed: Node%d configuration is missing required field 'IP'", nodeID)
	}
	if nodeCfg.Board == "" {
		return fmt.Errorf("TPIConfig validation failed: Node%d configuration is missing required field 'Board'", nodeID)
	}
	return nil
}

// GetNodeConfig returns the configuration for a specific node.
func (p *TuringPiProvider) GetNodeConfig(nodeID NodeID) *NodeConfig {
	switch nodeID {
	case Node1:
		return p.config.Node1
	case Node2:
		return p.config.Node2
	case Node3:
		return p.config.Node3
	case Node4:
		return p.config.Node4
	default:
		return nil
	}
}

// GetCacheDir returns the cache directory path.
func (p *TuringPiProvider) GetCacheDir() string {
	return p.config.CacheDir
}

// GetPrepImageDir returns the image preparation directory path.
func (p *TuringPiProvider) GetPrepImageDir() string {
	if p.config.PrepImageDir != "" {
		return p.config.PrepImageDir
	}
	return filepath.Join(p.config.CacheDir, "prep")
}

// GetBMCSSHConfig returns the SSH configuration for BMC access.
func (p *TuringPiProvider) GetBMCSSHConfig() bmc.SSHConfig {
	return bmc.SSHConfig{
		Host:     p.config.BMCIP,
		User:     p.config.BMCUser,
		Password: p.config.BMCPassword,
	}
}

// GetRemoteCache returns the remote cache interface
func (p *TuringPiProvider) GetRemoteCache() cache.Cache {
	// For now, we'll just use the local cache as the remote cache too
	// This will be replaced with actual remote cache implementation later
	remoteCacheDir := filepath.Join(p.config.CacheDir, "remote")
	remoteCache, err := cache.NewFSCache(remoteCacheDir)
	if err != nil {
		log.Printf("Warning: Failed to initialize remote cache: %v", err)
		return nil
	}
	return remoteCache
}

// GetLocalCache returns the local cache interface
func (p *TuringPiProvider) GetLocalCache() cache.Cache {
	localCacheDir := filepath.Join(p.config.CacheDir, "local")
	localCache, err := cache.NewFSCache(localCacheDir)
	if err != nil {
		log.Printf("Warning: Failed to initialize local cache: %v", err)
		return nil
	}
	return localCache
}

// Cache returns the default cache interface (local)
func (p *TuringPiProvider) Cache() cache.Cache {
	return p.GetLocalCache()
}

// TODO: Define Context interface and implementation more completely.
// TODO: Implement robust derivation logic for Node details (Gateway, DNS, Hostname).

// TODO: Implement calculateInputHash more robustly, maybe hash file contents for CopyLocalFile?

// Run executes a workflow template for a node
func (p *TuringPiProvider) Run(template func(ctx Context, cluster Cluster, node Node) error) func(ctx Context, nodeID NodeID) error {
	return func(ctx Context, nodeID NodeID) error {
		// Get node config
		nodeConfig := p.GetNodeConfig(nodeID)
		if nodeConfig == nil {
			return fmt.Errorf("no configuration found for node %d", nodeID)
		}

		// Create node instance
		node := Node{
			ID:     nodeID,
			Config: nodeConfig,
		}

		// Execute the template
		return template(ctx, p, node)
	}
}
