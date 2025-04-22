package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/davidroman0O/turingpi/pkg/v2/bmc"
	"github.com/davidroman0O/turingpi/pkg/v2/cache"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/operations"
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
	bmcTool        BMCTool
	operationsTool OperationsTool
	containerTool  ContainerTool
	localCache     *cache.FSCache
	remoteCache    *cache.SSHCache
	tmpCache       *cache.TempFSCache
	mu             sync.RWMutex
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

	// Initialize temporary cache with auto-cleanup
	if config.TempCacheDir != "" {
		// If a specific temp directory is provided, use it
		tmpCache, err := cache.NewTempFSCache(config.TempCacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize temporary cache: %w", err)
		}
		provider.tmpCache = tmpCache
	} else {
		// Otherwise create a system temp directory
		tmpCache, err := cache.CreateTempCache("")
		if err != nil {
			return nil, fmt.Errorf("failed to initialize temporary cache: %w", err)
		}
		provider.tmpCache = tmpCache
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

		// Set up container config for the operations tool
		containerConfig := container.ContainerConfig{
			Image:        "ubuntu:latest",
			Name:         fmt.Sprintf("turingpi-operations-%d", time.Now().UnixNano()),
			Command:      []string{"sleep", "infinity"},
			Privileged:   true,
			InitCommands: [][]string{},
			Capabilities: []string{
				"SYS_ADMIN", // Required for mount operations
				"MKNOD",     // Required for device operations
			},
			// Set working directory to /workdir
			WorkDir: "/workdir",
		}

		// Create operations tool with our container tool and specific config
		options := OperationsToolOptions{
			ContainerTool:   provider.containerTool,
			ExecutionMode:   operations.ExecuteAuto,
			ContainerConfig: containerConfig,
			// UsePersistentContainer: true, // Use persistent container for better performance
		}

		opsTool, err := NewOperationsToolWithOptions(options)

		// opsTool, err := NewOperationsTool(provider.containerTool)
		if err != nil {
			// Log the error but continue as this isn't critical
			fmt.Printf("Warning: Failed to initialize operations tool: %v\n", err)
		} else {
			provider.operationsTool = opsTool
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

	if provider.bmcTool == nil {
		return nil, fmt.Errorf("BMC tool is not initialized")
	}

	if provider.operationsTool == nil {
		return nil, fmt.Errorf("image tool is not initialized")
	}

	if provider.containerTool == nil {
		return nil, fmt.Errorf("container tool is not initialized")
	}

	if provider.localCache == nil {
		return nil, fmt.Errorf("local cache is not initialized")
	}

	if provider.remoteCache == nil {
		return nil, fmt.Errorf("remote cache is not initialized")
	}

	return provider, nil
}

// GetBMCTool returns the BMC tool
func (p *TuringPiToolProvider) GetBMCTool() BMCTool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.bmcTool
}

// GetOperationsTool returns the operations tool instance
func (p *TuringPiToolProvider) GetOperationsTool() OperationsTool {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.operationsTool == nil {
		p.mu.RUnlock()
		p.mu.Lock()
		defer p.mu.Unlock()

		// Double-check
		if p.operationsTool == nil {
			// Set up container config for the operations tool
			containerConfig := container.ContainerConfig{
				Image:        "ubuntu:latest",
				Name:         fmt.Sprintf("turingpi-operations-%d", time.Now().UnixNano()),
				Command:      []string{"sleep", "infinity"},
				Privileged:   true,
				InitCommands: [][]string{},
				Capabilities: []string{
					"SYS_ADMIN", // Required for mount operations
					"MKNOD",     // Required for device operations
				},
				// Set working directory to /workdir
				WorkDir: "/workdir",
			}

			// Create operations tool with our container tool and specific config
			options := OperationsToolOptions{
				ContainerTool:          p.GetContainerTool(),
				ExecutionMode:          operations.ExecuteAuto,
				ContainerConfig:        containerConfig,
				UsePersistentContainer: true, // Use persistent container for better performance
			}

			tool, err := NewOperationsToolWithOptions(options)
			if err != nil {
				// Log the error but don't fail - will try again next time
				fmt.Printf("Failed to create operations tool: %v\n", err)
				return nil
			}

			p.operationsTool = tool
		}
	}

	return p.operationsTool
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

// GetTmpCache returns the temporary filesystem cache
func (p *TuringPiToolProvider) GetTmpCache() *cache.TempFSCache {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.tmpCache
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

	// TempCacheDir is the directory for temporary caching
	// If specified, temporary cache will be created in this directory
	// If not specified, a system temporary directory will be used
	TempCacheDir string

	// RemoteCache holds configuration for the remote cache
	RemoteCache *RemoteCacheConfig
}

// Close cleans up resources
func (p *TuringPiToolProvider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var multiErr []error

	// Close and unset the image tool if set
	if p.operationsTool != nil {
		if err := p.operationsTool.Close(); err != nil {
			multiErr = append(multiErr, fmt.Errorf("failed to close image tool: %w", err))
		}
		p.operationsTool = nil
	}

	// Close and unset the container tool if set
	if p.containerTool != nil {
		if err := p.containerTool.CloseRegistry(); err != nil {
			multiErr = append(multiErr, fmt.Errorf("failed to close container tool: %w", err))
		}
		p.containerTool = nil
	}

	// Close and unset the local cache if set
	if p.localCache != nil {
		if err := p.localCache.Close(); err != nil {
			multiErr = append(multiErr, fmt.Errorf("failed to close local cache: %w", err))
		}
		p.localCache = nil
	}

	// Close and unset the remote cache if set
	if p.remoteCache != nil {
		if err := p.remoteCache.Close(); err != nil {
			multiErr = append(multiErr, fmt.Errorf("failed to close remote cache: %w", err))
		}
		p.remoteCache = nil
	}

	// Close and unset the tmp cache if set
	if p.tmpCache != nil {
		if err := p.tmpCache.Close(); err != nil {
			multiErr = append(multiErr, fmt.Errorf("failed to close tmp cache: %w", err))
		}
		p.tmpCache = nil
	}

	// Return the first error if any, or nil if no errors
	if len(multiErr) > 0 {
		// Combine all errors into one error message
		var errMsg string
		for i, err := range multiErr {
			if i > 0 {
				errMsg += "; "
			}
			errMsg += err.Error()
		}
		return fmt.Errorf("multiple errors during close: %s", errMsg)
	}

	return nil
}

// NewTuringPiToolProviderForTesting creates a provider with relaxed requirements for testing
// This function should only be used in test code, never in production
func NewTuringPiToolProviderForTesting(config *TuringPiToolConfig, skipChecks bool) (*TuringPiToolProvider, error) {
	provider := &TuringPiToolProvider{}

	// Initialize tools
	if config.BMCExecutor != nil {
		bmcInstance := bmc.New(config.BMCExecutor)
		provider.bmcTool = NewBMCToolAdapter(bmcInstance)
	} else if !skipChecks {
		return nil, fmt.Errorf("BMCExecutor is required")
	}

	if config.CacheDir != "" {
		// Initialize the local cache directly
		fsCache, err := cache.NewFSCache(config.CacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize local cache: %w", err)
		}
		provider.localCache = fsCache
	} else if !skipChecks {
		return nil, fmt.Errorf("local cache is not initialized")
	}

	// Initialize temporary cache with auto-cleanup
	if config.TempCacheDir != "" {
		// If a specific temp directory is provided, use it
		tmpCache, err := cache.NewTempFSCache(config.TempCacheDir)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize temporary cache: %w", err)
		}
		provider.tmpCache = tmpCache
	} else {
		// Otherwise create a system temp directory
		tmpCache, err := cache.CreateTempCache("")
		if err != nil {
			return nil, fmt.Errorf("failed to initialize temporary cache: %w", err)
		}
		provider.tmpCache = tmpCache
	}

	// Initialize container tool if Docker is available
	// Skip if TURINGPI_SKIP_DOCKER is set to true or skipChecks is true
	skipDocker := os.Getenv("TURINGPI_SKIP_DOCKER") == "true" || skipChecks
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
			provider.operationsTool = opsTool
		}
	} else if skipDocker && !skipChecks {
		fmt.Println("Skipping Docker and operations tools initialization as TURINGPI_SKIP_DOCKER=true")
	}

	// Initialize remote cache if remote config is provided
	if config.RemoteCache != nil && config.RemoteCache.Host != "" && !skipChecks {
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

	// Skip all requirement checks if skipChecks is true
	if skipChecks {
		return provider, nil
	}

	// Otherwise perform the normal requirement checks
	if provider.bmcTool == nil {
		return nil, fmt.Errorf("BMC tool is not initialized")
	}

	if provider.operationsTool == nil {
		return nil, fmt.Errorf("image tool is not initialized")
	}

	if provider.containerTool == nil {
		return nil, fmt.Errorf("container tool is not initialized")
	}

	if provider.localCache == nil {
		return nil, fmt.Errorf("local cache is not initialized")
	}

	if provider.remoteCache == nil {
		return nil, fmt.Errorf("remote cache is not initialized")
	}

	return provider, nil
}
