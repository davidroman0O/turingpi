package tools

import (
	"context"

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

// RemoveContainer removes a container
func (t *ContainerToolImpl) RemoveContainer(ctx context.Context, containerID string) error {
	return t.registry.Remove(ctx, containerID)
}
