package tools

import (
	"context"
	"io"

	"github.com/davidroman0O/turingpi/pkg/v2/container"
)

// ContainerToolImpl is the implementation of the ContainerTool interface
type ContainerToolImpl struct {
	registry container.Registry
}

// NewContainerTool creates a new ContainerTool
func NewContainerTool(registry container.Registry) ContainerTool {
	return &ContainerToolImpl{
		registry: registry,
	}
}

// CreateContainer creates a new container
func (t *ContainerToolImpl) CreateContainer(ctx context.Context, config container.ContainerConfig) (container.Container, error) {
	return t.registry.Create(ctx, config)
}

// GetContainer retrieves a container by ID
func (t *ContainerToolImpl) GetContainer(ctx context.Context, containerID string) (container.Container, error) {
	return t.registry.Get(ctx, containerID)
}

// ListContainers returns all managed containers
func (t *ContainerToolImpl) ListContainers(ctx context.Context) ([]container.Container, error) {
	return t.registry.List(ctx)
}

// StartContainer starts a container
func (t *ContainerToolImpl) StartContainer(ctx context.Context, containerID string) error {
	c, err := t.registry.Get(ctx, containerID)
	if err != nil {
		return err
	}
	return c.Start(ctx)
}

// StopContainer stops a container
func (t *ContainerToolImpl) StopContainer(ctx context.Context, containerID string) error {
	c, err := t.registry.Get(ctx, containerID)
	if err != nil {
		return err
	}
	return c.Stop(ctx)
}

// KillContainer forcefully stops a container
func (t *ContainerToolImpl) KillContainer(ctx context.Context, containerID string) error {
	c, err := t.registry.Get(ctx, containerID)
	if err != nil {
		return err
	}
	return c.Kill(ctx)
}

// PauseContainer pauses a container
func (t *ContainerToolImpl) PauseContainer(ctx context.Context, containerID string) error {
	c, err := t.registry.Get(ctx, containerID)
	if err != nil {
		return err
	}
	return c.Pause(ctx)
}

// UnpauseContainer unpauses a container
func (t *ContainerToolImpl) UnpauseContainer(ctx context.Context, containerID string) error {
	c, err := t.registry.Get(ctx, containerID)
	if err != nil {
		return err
	}
	return c.Unpause(ctx)
}

// RunCommand executes a command in a container
func (t *ContainerToolImpl) RunCommand(ctx context.Context, containerID string, cmd []string) (string, error) {
	c, err := t.registry.Get(ctx, containerID)
	if err != nil {
		return "", err
	}

	stdout, err := c.Exec(ctx, cmd)
	if err != nil {
		return "", err
	}

	return stdout, nil
}

// RunDetachedCommand executes a command in a container without waiting for output
func (t *ContainerToolImpl) RunDetachedCommand(ctx context.Context, containerID string, cmd []string) error {
	c, err := t.registry.Get(ctx, containerID)
	if err != nil {
		return err
	}
	return c.ExecDetached(ctx, cmd)
}

// CopyToContainer copies a file or directory to a container
func (t *ContainerToolImpl) CopyToContainer(ctx context.Context, containerID, hostPath, containerPath string) error {
	c, err := t.registry.Get(ctx, containerID)
	if err != nil {
		return err
	}

	return c.CopyTo(ctx, hostPath, containerPath)
}

// CopyFromContainer copies a file or directory from a container
func (t *ContainerToolImpl) CopyFromContainer(ctx context.Context, containerID, containerPath, hostPath string) error {
	c, err := t.registry.Get(ctx, containerID)
	if err != nil {
		return err
	}

	return c.CopyFrom(ctx, containerPath, hostPath)
}

// GetContainerLogs returns container logs
func (t *ContainerToolImpl) GetContainerLogs(ctx context.Context, containerID string) (io.ReadCloser, error) {
	c, err := t.registry.Get(ctx, containerID)
	if err != nil {
		return nil, err
	}
	return c.Logs(ctx)
}

// WaitForContainer waits for the container to exit
func (t *ContainerToolImpl) WaitForContainer(ctx context.Context, containerID string) (int, error) {
	c, err := t.registry.Get(ctx, containerID)
	if err != nil {
		return -1, err
	}
	return c.Wait(ctx)
}

// RemoveContainer removes a container
func (t *ContainerToolImpl) RemoveContainer(ctx context.Context, containerID string) error {
	return t.registry.Remove(ctx, containerID)
}

// RemoveAllContainers removes all managed containers
func (t *ContainerToolImpl) RemoveAllContainers(ctx context.Context) error {
	return t.registry.RemoveAll(ctx)
}

// GetContainerStats returns container statistics
func (t *ContainerToolImpl) GetContainerStats(ctx context.Context, containerID string) (*container.ContainerState, error) {
	return t.registry.Stats(ctx, containerID)
}

// CloseRegistry releases all resources and removes all containers
func (t *ContainerToolImpl) CloseRegistry() error {
	return t.registry.Close()
}
