package container

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/client"
)

// DockerRegistry implements the Registry interface for Docker
type DockerRegistry struct {
	client     *client.Client
	containers map[string]*DockerContainer
	mu         sync.RWMutex
}

// DockerContextInfo holds context details
type DockerContextInfo struct {
	Name string
	Host string
}

// getDockerContextDetails gets detailed information about the current Docker context
func getDockerContextDetails() (DockerContextInfo, error) {
	var info DockerContextInfo

	// First get current context name
	nameCmd := exec.Command("docker", "context", "show")
	nameOutput, err := nameCmd.CombinedOutput()
	if err != nil {
		return info, fmt.Errorf("failed to get current Docker context: %w", err)
	}
	info.Name = strings.TrimSpace(string(nameOutput))
	// Remove the * suffix if present (indicates active context)
	info.Name = strings.TrimSuffix(info.Name, "*")
	info.Name = strings.TrimSpace(info.Name)

	// Then get host for that context
	hostCmd := exec.Command("docker", "context", "inspect", info.Name, "--format", "{{.Endpoints.docker.Host}}")
	hostOutput, err := hostCmd.CombinedOutput()
	if err != nil {
		// Fall back to default context inspect without name
		return getDockerHostFromContext()
	}

	info.Host = strings.TrimSpace(string(hostOutput))
	return info, nil
}

// getDockerHostFromContext tries to get Docker host info from docker context inspect
func getDockerHostFromContext() (DockerContextInfo, error) {
	var info DockerContextInfo

	// Try template format first
	cmd := exec.Command("docker", "context", "inspect", "--format", "{{.Endpoints.docker.Host}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return info, fmt.Errorf("failed to get Docker context: %w", err)
	}

	dockerHost := strings.TrimSpace(string(output))
	if dockerHost == "" {
		// Try the full inspect to parse JSON if the template format didn't work
		inspectCmd := exec.Command("docker", "context", "inspect")
		inspectOutput, inspectErr := inspectCmd.CombinedOutput()
		if inspectErr != nil {
			return info, fmt.Errorf("failed to inspect Docker context: %w", inspectErr)
		}

		// Simple parsing to find the Host field
		// This is a fallback method that doesn't require JSON parsing
		for _, line := range strings.Split(string(inspectOutput), "\n") {
			if strings.Contains(line, "\"Host\"") {
				parts := strings.Split(line, ":")
				if len(parts) >= 2 {
					host := strings.Join(parts[1:], ":")
					// Clean up quotes and commas
					host = strings.Trim(host, " \",")
					if host != "" {
						info.Host = host
						return info, nil
					}
				}
			}
		}
	}

	info.Host = dockerHost
	return info, nil
}

// tryDockerCommand tries a simple Docker CLI command to check if Docker is available
func tryDockerCommand() error {
	cmd := exec.Command("docker", "version")
	return cmd.Run()
}

// NewDockerRegistry creates a new Docker registry
func NewDockerRegistry() (Registry, error) {
	var cli *client.Client
	var err error

	// Get the Docker context details first
	contextInfo, contextErr := getDockerContextDetails()
	if contextErr == nil && contextInfo.Host != "" {
		fmt.Printf("Using Docker host from context: %s\n", contextInfo.Host)

		// Try to create client with explicit context host
		cli, err = client.NewClientWithOpts(
			client.FromEnv,
			client.WithAPIVersionNegotiation(),
			client.WithHost(contextInfo.Host),
		)

		if err != nil {
			fmt.Printf("Failed to connect with context host, falling back: %v\n", err)
		}
	}

	// If context approach didn't work, try default options
	if cli == nil {
		// Check if DOCKER_HOST is explicitly set
		dockerHost := os.Getenv("DOCKER_HOST")
		if dockerHost != "" {
			fmt.Printf("Using DOCKER_HOST from environment: %s\n", dockerHost)
		}

		// Try with default environment settings
		cli, err = client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	}

	// If still failing, return a clear error
	if err != nil {
		// Try Docker version command to see if Docker CLI works
		if versionErr := tryDockerCommand(); versionErr == nil {
			// Docker CLI works but SDK connection failed
			return nil, fmt.Errorf("Docker CLI is available but SDK connection failed: %w\nCheck your Docker context configuration with 'docker context ls'", err)
		}
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w\nEnsure Docker is running and DOCKER_HOST environment variable is set correctly if using a non-default socket", err)
	}

	return &DockerRegistry{
		client:     cli,
		containers: make(map[string]*DockerContainer),
	}, nil
}

// Create implements Registry.Create
func (r *DockerRegistry) Create(ctx context.Context, config ContainerConfig) (Container, error) {
	// Convert our config to Docker config
	containerConfig, hostConfig := convertConfig(config)

	// Create container
	resp, err := r.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, config.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	// Create container instance
	container := &DockerContainer{
		id:     resp.ID,
		client: r.client,
		config: config,
	}

	// Register container
	r.mu.Lock()
	r.containers[resp.ID] = container
	r.mu.Unlock()

	return container, nil
}

// Get implements Registry.Get
func (r *DockerRegistry) Get(ctx context.Context, id string) (Container, error) {
	r.mu.RLock()
	container, exists := r.containers[id]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("container %s not found", id)
	}

	return container, nil
}

// List implements Registry.List
func (r *DockerRegistry) List(ctx context.Context) ([]Container, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	containers := make([]Container, 0, len(r.containers))
	for _, container := range r.containers {
		containers = append(containers, container)
	}

	return containers, nil
}

// Remove implements Registry.Remove
func (r *DockerRegistry) Remove(ctx context.Context, id string) error {
	r.mu.Lock()
	container, exists := r.containers[id]
	if !exists {
		r.mu.Unlock()
		return fmt.Errorf("container %s not found", id)
	}
	delete(r.containers, id)
	r.mu.Unlock()

	return container.Cleanup(ctx)
}

// RemoveAll implements Registry.RemoveAll
func (r *DockerRegistry) RemoveAll(ctx context.Context) error {
	r.mu.Lock()
	containers := make([]*DockerContainer, 0, len(r.containers))
	for _, container := range r.containers {
		containers = append(containers, container)
	}
	r.containers = make(map[string]*DockerContainer)
	r.mu.Unlock()

	var lastErr error
	for _, container := range containers {
		if err := container.Cleanup(ctx); err != nil {
			lastErr = err
		}
	}

	return lastErr
}

// Stats implements Registry.Stats
func (r *DockerRegistry) Stats(ctx context.Context, id string) (*ContainerState, error) {
	r.mu.RLock()
	container, exists := r.containers[id]
	r.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("container %s not found", id)
	}

	// Get container info from Docker
	info, err := r.client.ContainerInspect(ctx, container.id)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	// Parse time strings
	created, _ := time.Parse(time.RFC3339Nano, info.Created)
	started, _ := time.Parse(time.RFC3339Nano, info.State.StartedAt)
	finished, _ := time.Parse(time.RFC3339Nano, info.State.FinishedAt)

	// Convert to our state type
	state := &ContainerState{
		ID:           info.ID,
		Name:         info.Name,
		Image:        info.Image,
		Command:      info.Config.Cmd,
		Created:      created,
		Started:      started,
		Finished:     finished,
		ExitCode:     info.State.ExitCode,
		Status:       info.State.Status,
		Running:      info.State.Running,
		Paused:       info.State.Paused,
		OOMKilled:    info.State.OOMKilled,
		Dead:         info.State.Dead,
		Pid:          info.State.Pid,
		Error:        info.State.Error,
		RestartCount: info.RestartCount,
	}

	return state, nil
}

// Close releases resources used by the registry
func (r *DockerRegistry) Close() error {
	ctx := context.Background()
	if err := r.RemoveAll(ctx); err != nil {
		return fmt.Errorf("failed to remove all containers: %w", err)
	}

	if err := r.client.Close(); err != nil {
		return fmt.Errorf("failed to close Docker client: %w", err)
	}

	return nil
}
