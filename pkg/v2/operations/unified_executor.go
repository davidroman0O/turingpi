package operations

import (
	"context"
	"fmt"
	"runtime"
	"sync"

	"github.com/davidroman0O/turingpi/pkg/v2/container"
)

// ExecutionMode defines how commands should be executed
type ExecutionMode int

const (
	// ExecuteAuto determines the execution mode based on the OS
	// Linux uses native, non-Linux uses container
	ExecuteAuto ExecutionMode = iota

	// ExecuteNative forces native command execution
	ExecuteNative

	// ExecuteContainer forces container-based execution
	ExecuteContainer
)

// UnifiedExecutor provides a unified interface for command execution
// that can work across different platforms using either native commands
// or containers as appropriate
type UnifiedExecutor struct {
	// The current execution mode
	mode ExecutionMode

	// Container registry for creating containers
	registry container.Registry

	// For ExecuteContainer mode, we might keep a long-lived container
	container     container.Container
	containerLock sync.RWMutex

	// Container configuration
	containerConfig container.ContainerConfig

	// Flag to control whether we use a persistent container
	// or create a new one for each command
	usePersistentContainer bool

	// Track if we've initialized
	initialized bool
	initLock    sync.Mutex
}

// UnifiedExecutorOptions configures the unified executor
type UnifiedExecutorOptions struct {
	// Mode specifies the execution mode (auto, native, container)
	Mode ExecutionMode

	// Registry is the container registry to use
	Registry container.Registry

	// ContainerConfig is the configuration for container creation
	ContainerConfig container.ContainerConfig

	// UsePersistentContainer indicates whether to use a persistent container
	UsePersistentContainer bool
}

// NewUnifiedExecutor creates a new UnifiedExecutor with the specified options
func NewUnifiedExecutor(options UnifiedExecutorOptions) *UnifiedExecutor {
	// Default to auto mode if not specified
	mode := options.Mode

	// If registry is nil and mode is container, switch to auto
	if options.Registry == nil && mode == ExecuteContainer {
		mode = ExecuteAuto
	}

	return &UnifiedExecutor{
		mode:                   mode,
		registry:               options.Registry,
		containerConfig:        options.ContainerConfig,
		usePersistentContainer: options.UsePersistentContainer,
	}
}

// initialize ensures the executor is ready to execute commands
func (e *UnifiedExecutor) initialize(ctx context.Context) error {
	e.initLock.Lock()
	defer e.initLock.Unlock()

	if e.initialized {
		return nil
	}

	// Determine effective mode
	effectiveMode := e.mode
	if effectiveMode == ExecuteAuto {
		if runtime.GOOS == "linux" {
			effectiveMode = ExecuteNative
		} else {
			effectiveMode = ExecuteContainer
		}
	}

	// If we're using container mode, we need a registry
	if effectiveMode == ExecuteContainer && e.registry == nil {
		return fmt.Errorf("container registry is required for container execution mode")
	}

	// If we're using a persistent container, create it now
	if effectiveMode == ExecuteContainer && e.usePersistentContainer {
		container, err := e.registry.Create(ctx, e.containerConfig)
		if err != nil {
			return fmt.Errorf("failed to create container: %w", err)
		}

		// Start the container
		if err := container.Start(ctx); err != nil {
			// Clean up if start fails
			_ = e.registry.Remove(ctx, container.ID())
			return fmt.Errorf("failed to start container: %w", err)
		}

		e.containerLock.Lock()
		e.container = container
		e.containerLock.Unlock()
	}

	e.initialized = true
	return nil
}

// getExecutor returns the appropriate executor based on the mode
func (e *UnifiedExecutor) getExecutor(ctx context.Context) (CommandExecutor, func(), error) {
	// Initialize if needed
	if !e.initialized {
		if err := e.initialize(ctx); err != nil {
			return nil, nil, err
		}
	}

	// Determine effective mode
	effectiveMode := e.mode
	if effectiveMode == ExecuteAuto {
		if runtime.GOOS == "linux" {
			effectiveMode = ExecuteNative
		} else {
			effectiveMode = ExecuteContainer
		}
	}

	// For native execution, just return a NativeExecutor
	if effectiveMode == ExecuteNative {
		return &NativeExecutor{}, func() {}, nil
	}

	// For container execution with a persistent container
	if e.usePersistentContainer {
		e.containerLock.RLock()
		container := e.container
		e.containerLock.RUnlock()

		return NewContainerExecutor(container), func() {}, nil
	}

	// For container execution with a temporary container
	container, err := e.registry.Create(ctx, e.containerConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create temporary container: %w", err)
	}

	// Start the container
	if err := container.Start(ctx); err != nil {
		// Clean up if start fails
		_ = e.registry.Remove(ctx, container.ID())
		return nil, nil, fmt.Errorf("failed to start temporary container: %w", err)
	}

	// Create a cleanup function
	cleanup := func() {
		ctx := context.Background() // Use a new context for cleanup
		_ = e.registry.Remove(ctx, container.ID())
	}

	return NewContainerExecutor(container), cleanup, nil
}

// Execute implements CommandExecutor.Execute
func (e *UnifiedExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	executor, cleanup, err := e.getExecutor(ctx)
	if err != nil {
		return nil, err
	}

	if cleanup != nil {
		defer cleanup()
	}

	return executor.Execute(ctx, name, args...)
}

// ExecuteWithInput implements CommandExecutor.ExecuteWithInput
func (e *UnifiedExecutor) ExecuteWithInput(ctx context.Context, input string, name string, args ...string) ([]byte, error) {
	executor, cleanup, err := e.getExecutor(ctx)
	if err != nil {
		return nil, err
	}

	if cleanup != nil {
		defer cleanup()
	}

	return executor.ExecuteWithInput(ctx, input, name, args...)
}

// ExecuteInPath implements CommandExecutor.ExecuteInPath
func (e *UnifiedExecutor) ExecuteInPath(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	executor, cleanup, err := e.getExecutor(ctx)
	if err != nil {
		return nil, err
	}

	if cleanup != nil {
		defer cleanup()
	}

	return executor.ExecuteInPath(ctx, dir, name, args...)
}

// Close cleans up resources associated with the executor
func (e *UnifiedExecutor) Close() error {
	if !e.initialized {
		return nil
	}

	e.containerLock.Lock()
	defer e.containerLock.Unlock()

	if e.container != nil {
		ctx := context.Background()
		containerID := e.container.ID()
		e.container = nil

		return e.registry.Remove(ctx, containerID)
	}

	return nil
}
