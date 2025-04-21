package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/davidroman0O/turingpi/pkg/v2/bmc"
	"github.com/davidroman0O/turingpi/pkg/v2/cache"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/platform"
)

// BMCToolAdapter adapts the bmc.BMC type to implement the BMCTool interface
type BMCToolAdapter struct {
	bmc bmc.BMC
}

// GetPowerStatus retrieves the power status of a specific node
func (a *BMCToolAdapter) GetPowerStatus(ctx context.Context, nodeID int) (*bmc.PowerStatus, error) {
	return a.bmc.GetPowerStatus(ctx, nodeID)
}

// PowerOn turns on a specific node
func (a *BMCToolAdapter) PowerOn(ctx context.Context, nodeID int) error {
	return a.bmc.PowerOn(ctx, nodeID)
}

// PowerOff turns off a specific node
func (a *BMCToolAdapter) PowerOff(ctx context.Context, nodeID int) error {
	return a.bmc.PowerOff(ctx, nodeID)
}

// Reset performs a hard reset on a specific node
func (a *BMCToolAdapter) Reset(ctx context.Context, nodeID int) error {
	return a.bmc.Reset(ctx, nodeID)
}

// GetInfo retrieves information about the BMC
func (a *BMCToolAdapter) GetInfo(ctx context.Context) (*bmc.BMCInfo, error) {
	return a.bmc.GetInfo(ctx)
}

// Reboot reboots the BMC chip
func (a *BMCToolAdapter) Reboot(ctx context.Context) error {
	return a.bmc.Reboot(ctx)
}

// UpdateFirmware updates the BMC firmware
func (a *BMCToolAdapter) UpdateFirmware(ctx context.Context, firmwarePath string) error {
	return a.bmc.UpdateFirmware(ctx, firmwarePath)
}

// ExecuteCommand executes a BMC-specific command
func (a *BMCToolAdapter) ExecuteCommand(ctx context.Context, command string) (stdout string, stderr string, err error) {
	return a.bmc.ExecuteCommand(ctx, command)
}

// GetNodeUSBMode gets the USB mode for a specific node
func (a *BMCToolAdapter) GetNodeUSBMode(ctx context.Context, nodeID int) (string, error) {
	// Not implemented in the base BMC, return a placeholder
	return "device", fmt.Errorf("GetNodeUSBMode not implemented")
}

// SetNodeUSBMode sets the USB mode for a specific node
func (a *BMCToolAdapter) SetNodeUSBMode(ctx context.Context, nodeID int, mode string) error {
	// Not implemented in the base BMC
	return fmt.Errorf("SetNodeUSBMode not implemented")
}

// GetClusterHealth gets the health status of the entire cluster
func (a *BMCToolAdapter) GetClusterHealth(ctx context.Context) (map[string]interface{}, error) {
	// Not implemented in the base BMC
	return map[string]interface{}{
		"status": "unknown",
	}, fmt.Errorf("GetClusterHealth not implemented")
}

// GetSerialConsole connects to the serial console of a specific node
func (a *BMCToolAdapter) GetSerialConsole(ctx context.Context, nodeID int) (io.ReadWriteCloser, error) {
	// Not implemented in the base BMC
	return nil, fmt.Errorf("GetSerialConsole not implemented")
}

// SetBootMode sets the boot mode for a specific node
func (a *BMCToolAdapter) SetBootMode(ctx context.Context, nodeID int, mode string) error {
	// Not implemented in the base BMC
	return fmt.Errorf("SetBootMode not implemented")
}

// GetBootMode gets the current boot mode for a specific node
func (a *BMCToolAdapter) GetBootMode(ctx context.Context, nodeID int) (string, error) {
	// Not implemented in the base BMC
	return "normal", fmt.Errorf("GetBootMode not implemented")
}

// NewBMCToolAdapter creates a new BMCToolAdapter from a bmc.BMC instance
func NewBMCToolAdapter(bmc bmc.BMC) BMCTool {
	return &BMCToolAdapter{
		bmc: bmc,
	}
}

// TuringPiToolProvider is the central implementation of the ToolProvider interface
type TuringPiToolProvider struct {
	bmcTool       BMCTool
	imageTool     OperationsTool
	containerTool ContainerTool
	localCache    *cache.FSCache
	remoteCache   *cache.SSHCache
	mu            sync.RWMutex
}

// NewTuringPiToolProvider creates a new TuringPiToolProvider
func NewTuringPiToolProvider(config *TuringPiToolConfig) (*TuringPiToolProvider, error) {
	provider := &TuringPiToolProvider{}

	// Initialize tools
	if config.BMCExecutor != nil {
		bmcInstance := bmc.New(config.BMCExecutor)
		provider.bmcTool = NewBMCToolAdapter(bmcInstance)
	} else {
		return nil, fmt.Errorf("BMCExecutor is required")
	}

	if config.CacheDir != "" {
		// Initialize the local cache directly
		fsCache, err := cache.NewFSCache(config.CacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize local cache: %w", err)
		}
		provider.localCache = fsCache
	}

	// Initialize container tool if Docker is available
	// Skip if TURINGPI_SKIP_DOCKER is set to true
	skipDocker := os.Getenv("TURINGPI_SKIP_DOCKER") == "true"
	if !skipDocker && platform.DockerAvailable() {
		registry, err := container.NewDockerRegistry()
		if err == nil {
			provider.containerTool = NewContainerTool(registry)
		}
	}

	// Initialize operations-based image tool (skip if Docker is skipped)
	if !skipDocker && provider.containerTool != nil {
		opsTool, err := NewOperationsTool(provider.containerTool)
		if err != nil {
			// Log the error but continue as this isn't critical
			fmt.Printf("Warning: Failed to initialize operations tool: %v\n", err)
		} else {
			provider.imageTool = opsTool
		}
	} else if skipDocker {
		fmt.Println("Skipping Docker and operations tools initialization as TURINGPI_SKIP_DOCKER=true")
	}

	// Initialize remote cache if remote config is provided
	if config.RemoteCache != nil && config.RemoteCache.Host != "" {
		sshConfig := cache.SSHConfig{
			Host:     config.RemoteCache.Host,
			Port:     config.RemoteCache.Port,
			User:     config.RemoteCache.User,
			Password: config.RemoteCache.Password,
		}

		sshCache, err := cache.NewSSHCache(sshConfig, config.RemoteCache.RemotePath)
		if err == nil {
			provider.remoteCache = sshCache
		} else {
			// Just log the error and continue without remote cache
			fmt.Printf("Failed to create remote cache: %v\n", err)
		}
	}

	return provider, nil
}

// GetBMCTool returns the BMC tool
func (p *TuringPiToolProvider) GetBMCTool() BMCTool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.bmcTool
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

// GetLocalCache returns the local filesystem cache
func (p *TuringPiToolProvider) GetLocalCache() *cache.FSCache {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.localCache
}

// GetRemoteCache returns the remote SSH cache
func (p *TuringPiToolProvider) GetRemoteCache() *cache.SSHCache {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.remoteCache
}

// RemoteCacheConfig holds configuration for remote cache
type RemoteCacheConfig struct {
	// Host is the hostname or IP address of the node
	Host string

	// User is the username for SSH
	User string

	// Password is the password for SSH
	Password string

	// RemotePath is the path on the remote system where cache will be stored
	RemotePath string

	// Port is the SSH port (default: 22)
	Port int
}

// TuringPiToolConfig holds configuration for the TuringPiToolProvider
type TuringPiToolConfig struct {
	// BMCExecutor is an executor for BMC commands
	BMCExecutor bmc.CommandExecutor

	// CacheDir is the directory for caching
	CacheDir string

	// RemoteCache holds configuration for the remote cache
	RemoteCache *RemoteCacheConfig
}
