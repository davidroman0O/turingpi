// Package container provides a unified interface for container operations
package container

import (
	"context"
	"io"
	"time"
)

// ContainerConfig holds configuration for container creation
type ContainerConfig struct {
	// Image is the container image to use
	Image string

	// Name is the container name (optional)
	Name string

	// Command is the command to run in the container
	Command []string

	// Environment variables
	Env map[string]string

	// Volume mounts (host:container)
	Mounts map[string]string

	// Working directory in container
	WorkDir string

	// Network mode (host, bridge, none)
	NetworkMode string

	// Whether to run in privileged mode
	Privileged bool

	// Additional capabilities
	Capabilities []string

	// Resource limits
	Resources ResourceLimits
}

// ResourceLimits defines container resource constraints
type ResourceLimits struct {
	// CPU limits
	CPUShares  int64
	CPUQuota   int64
	CPUPeriod  int64
	CPUSetCPUs string
	CPUSetMems string

	// Memory limits (in bytes)
	Memory     int64
	MemorySwap int64

	// IO limits
	BlkioWeight uint16
}

// Container represents a running container instance
type Container interface {
	// ID returns the container ID
	ID() string

	// Start starts the container
	Start(ctx context.Context) error

	// Stop stops the container
	Stop(ctx context.Context) error

	// Kill forcefully stops the container
	Kill(ctx context.Context) error

	// Pause pauses the container
	Pause(ctx context.Context) error

	// Unpause unpauses the container
	Unpause(ctx context.Context) error

	// Exec executes a command in the container
	Exec(ctx context.Context, cmd []string) (string, error)

	// ExecDetached executes a command in the container without waiting for output
	ExecDetached(ctx context.Context, cmd []string) error

	// CopyTo copies a file/directory into the container
	CopyTo(ctx context.Context, hostPath, containerPath string) error

	// CopyFrom copies a file/directory from the container
	CopyFrom(ctx context.Context, containerPath, hostPath string) error

	// Logs returns container logs
	Logs(ctx context.Context) (io.ReadCloser, error)

	// Wait waits for the container to exit
	Wait(ctx context.Context) (int, error)

	// Cleanup removes the container and its resources
	Cleanup(ctx context.Context) error
}

// ContainerState represents the state of a container
type ContainerState struct {
	ID           string
	Name         string
	Image        string
	Command      []string
	Created      time.Time
	Started      time.Time
	Finished     time.Time
	ExitCode     int
	Status       string
	Running      bool
	Paused       bool
	OOMKilled    bool
	Dead         bool
	Pid          int
	Error        string
	RestartCount int
}

// Registry manages container lifecycle and tracking
type Registry interface {
	// Create creates a new container
	Create(ctx context.Context, config ContainerConfig) (Container, error)

	// Get returns a container by ID
	Get(ctx context.Context, id string) (Container, error)

	// List returns all managed containers
	List(ctx context.Context) ([]Container, error)

	// Remove removes a container
	Remove(ctx context.Context, id string) error

	// RemoveAll removes all managed containers
	RemoveAll(ctx context.Context) error

	// Stats returns container statistics
	Stats(ctx context.Context, id string) (*ContainerState, error)

	// RegisterExistingContainer registers an existing container with the registry
	RegisterExistingContainer(ctx context.Context, id string, config ContainerConfig) (Container, error)

	// Close releases all resources and removes all containers
	Close() error
}
