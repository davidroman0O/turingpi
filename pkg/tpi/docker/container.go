package docker

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// Container represents a Docker container for executing commands
type Container struct {
	Config      *platform.DockerExecutionConfig
	ContainerID string
	cli         *client.Client
	ctx         context.Context
}

// New creates a new Docker container manager
func New(config *platform.DockerExecutionConfig) (*Container, error) {
	ctx := context.Background()
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

	c := &Container{
		Config: config,
		cli:    cli,
		ctx:    ctx,
	}

	// Ensure Docker image exists
	if err := c.ensureDockerImage(); err != nil {
		return nil, err
	}

	// Create a persistent container
	if err := c.createContainer(); err != nil {
		return nil, err
	}

	// Register container with the global registry
	registry := GetRegistry()
	registry.RegisterContainer(c.ContainerID)

	return c, nil
}

// tryDockerCommand tries a simple Docker CLI command to check if Docker is available
func tryDockerCommand() error {
	cmd := exec.Command("docker", "version")
	return cmd.Run()
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

		return info, fmt.Errorf("empty Docker host from context")
	}

	info.Host = dockerHost
	return info, nil
}

// ensureDockerImage makes sure the required Docker image is available
func (c *Container) ensureDockerImage() error {
	// Special handling for turingpi-prepare image
	if c.Config.DockerImage == "turingpi-prepare" {
		return c.ensureTuringPiPrepareImage()
	}

	// For standard images, check if image exists locally
	_, _, err := c.cli.ImageInspectWithRaw(c.ctx, c.Config.DockerImage)
	if err == nil {
		return nil // Image exists
	}

	// Pull the image
	reader, err := c.cli.ImagePull(c.ctx, c.Config.DockerImage, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("error pulling Docker image: %w", err)
	}
	defer reader.Close()

	// Wait for the pull to complete
	io.Copy(io.Discard, reader)
	return nil
}

// ensureTuringPiPrepareImage checks if the turingpi-prepare image exists, and builds it if not
func (c *Container) ensureTuringPiPrepareImage() error {
	// Check if the image already exists
	_, _, err := c.cli.ImageInspectWithRaw(c.ctx, "turingpi-prepare")
	if err == nil {
		fmt.Println("Using existing turingpi-prepare Docker image")
		return nil
	}

	fmt.Println("Building turingpi-prepare Docker image...")

	// Create a temporary directory for the Dockerfile
	tempDir, err := os.MkdirTemp("", "turingpi-dockerfile-*")
	if err != nil {
		return fmt.Errorf("failed to create temporary directory: %w", err)
	}
	defer os.RemoveAll(tempDir) // Clean up when done

	// Define the Dockerfile content
	// Note: replaced virtual packages with their concrete implementations
	dockerfileContent := `FROM ubuntu:22.04

# Install necessary tools - using specific package names instead of virtual packages
RUN apt-get update && apt-get install -y \
    kpartx \
    xz-utils \
    sudo \
    parted \
    e2fsprogs \
    dosfstools \
    mount \
    mawk \
    coreutils \
    util-linux \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /workspace

# Default command - will be overridden when exec is used
CMD ["sleep", "infinity"]
`

	// Write the Dockerfile to the temporary directory
	dockerfilePath := filepath.Join(tempDir, "Dockerfile")
	if err := os.WriteFile(dockerfilePath, []byte(dockerfileContent), 0644); err != nil {
		return fmt.Errorf("failed to write Dockerfile: %w", err)
	}

	// Use docker build command
	buildCmd := fmt.Sprintf("docker build -t turingpi-prepare -f %s %s",
		dockerfilePath, tempDir)

	// Use os.Exec for the build
	execCmd := exec.Command("bash", "-c", buildCmd)
	execCmd.Stdout = os.Stdout
	execCmd.Stderr = os.Stderr

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("Docker build failed: %w", err)
	}

	fmt.Println("Successfully built turingpi-prepare Docker image")
	return nil
}

// createContainer creates a persistent container for running commands
func (c *Container) createContainer() error {
	// Generate a unique container name if requested
	if c.Config.UseUniqueContainerName {
		// Always generate a new name to ensure uniqueness
		uniqueID := fmt.Sprintf("%d-%x", time.Now().UnixNano(), rand.Intn(0x10000))
		c.Config.ContainerName = fmt.Sprintf("%s-%s", c.Config.ContainerName, uniqueID)
		fmt.Printf("Using unique container name: %s\n", c.Config.ContainerName)
	}

	// First, check if a container with this name already exists
	if c.Config.ContainerName != "" {
		containers, err := c.cli.ContainerList(c.ctx, container.ListOptions{All: true})
		if err != nil {
			return fmt.Errorf("error listing containers: %w", err)
		}

		// Check for exact match on container name
		var existingContainerID string
		var existingContainerStatus string
		for _, containerItem := range containers {
			// Check container names (Docker adds a leading slash to container names)
			for _, name := range containerItem.Names {
				// Remove leading slash if present
				cleanName := name
				if len(name) > 0 && name[0] == '/' {
					cleanName = name[1:]
				}

				if cleanName == c.Config.ContainerName {
					fmt.Printf("Container with name %s already exists (ID: %s, Status: %s)\n",
						c.Config.ContainerName, containerItem.ID, containerItem.Status)
					existingContainerID = containerItem.ID
					existingContainerStatus = containerItem.Status
					break
				}
			}
			if existingContainerID != "" {
				break
			}
		}

		// Handle existing container
		if existingContainerID != "" {
			// If we're using unique container names, this shouldn't happen.
			// If it does, regenerate the name with an additional unique component
			if c.Config.UseUniqueContainerName {
				// Add another random component to ensure uniqueness
				extraUnique := fmt.Sprintf("%d-%x", time.Now().UnixNano(), rand.Intn(0x10000))
				c.Config.ContainerName = fmt.Sprintf("%s-retry-%s", c.Config.ContainerName, extraUnique)
				fmt.Printf("Container name collision detected despite unique name. Using new name: %s\n", c.Config.ContainerName)
			} else {
				// Always remove existing container for consistency, regardless of state
				fmt.Printf("Removing existing container: %s (Status: %s)\n", existingContainerID, existingContainerStatus)
				err := c.cli.ContainerRemove(c.ctx, existingContainerID, container.RemoveOptions{Force: true})
				if err != nil {
					return fmt.Errorf("error removing existing container: %w", err)
				}
				fmt.Printf("Successfully removed existing container: %s\n", existingContainerID)
			}
		}
	}

	// Prepare mount bindings
	binds := []string{}

	// Special handling for turingpi-prepare to maintain compatibility with old code
	if c.Config.DockerImage == "turingpi-prepare" {
		// Create directory if it doesn't exist
		if c.Config.TempDir != "" {
			if err := os.MkdirAll(c.Config.TempDir, 0755); err != nil {
				fmt.Printf("Warning: Failed to create temp directory %s: %v\n", c.Config.TempDir, err)
			}
		}

		fmt.Printf("DEBUG: Setting up mounts for turingpi-prepare image\n")
		fmt.Printf("DEBUG: Source Dir: %s -> /images:ro\n", c.Config.SourceDir)
		fmt.Printf("DEBUG: Temp Dir: %s -> /tmp\n", c.Config.TempDir)
		fmt.Printf("DEBUG: Output Dir: %s -> /prepared-images\n", c.Config.OutputDir)

		binds = []string{
			fmt.Sprintf("%s:/images:ro", c.Config.SourceDir),
			fmt.Sprintf("%s:/tmp", c.Config.TempDir),
			fmt.Sprintf("%s:/prepared-images", c.Config.OutputDir),
		}
	} else {
		// Standard mounts for other images
		fmt.Printf("DEBUG: Setting up standard mounts for image: %s\n", c.Config.DockerImage)

		// Add the standard mounts (source, temp, output directories)
		if c.Config.SourceDir != "" {
			fmt.Printf("DEBUG: Source Dir: %s -> /source:ro\n", c.Config.SourceDir)
			binds = append(binds, fmt.Sprintf("%s:/source:ro", c.Config.SourceDir))
		}
		if c.Config.TempDir != "" {
			fmt.Printf("DEBUG: Temp Dir: %s -> /tmp\n", c.Config.TempDir)
			binds = append(binds, fmt.Sprintf("%s:/tmp", c.Config.TempDir))
		}
		if c.Config.OutputDir != "" {
			fmt.Printf("DEBUG: Output Dir: %s -> /output\n", c.Config.OutputDir)
			binds = append(binds, fmt.Sprintf("%s:/output", c.Config.OutputDir))
		}
	}

	// Add any additional mounts
	for hostPath, containerPath := range c.Config.AdditionalMounts {
		fmt.Printf("DEBUG: Additional mount: %s -> %s\n", hostPath, containerPath)
		binds = append(binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// If we got here, we need to create a new container
	fmt.Printf("DEBUG: Creating container '%s' with image %s and %d volume bindings\n",
		c.Config.ContainerName, c.Config.DockerImage, len(binds))
	for i, bind := range binds {
		fmt.Printf("DEBUG: Volume binding %d: %s\n", i+1, bind)
	}

	// Create the container with a timeout to prevent hangs
	ctx, cancel := context.WithTimeout(c.ctx, 60*time.Second)
	defer cancel()

	resp, err := c.cli.ContainerCreate(
		ctx,
		&container.Config{
			Image: c.Config.DockerImage,
			// Don't set Cmd for turingpi-prepare as it already has an ENTRYPOINT
			Cmd: func() []string {
				if c.Config.DockerImage == "turingpi-prepare" {
					return nil // Don't specify CMD for turingpi-prepare
				}
				return []string{"sleep", "infinity"} // For other images
			}(),
			Tty: true,
		},
		&container.HostConfig{
			Binds: binds,
			// Add privileged mode for turingpi-prepare which needs to manipulate devices
			Privileged: c.Config.DockerImage == "turingpi-prepare",
		},
		nil,
		nil,
		c.Config.ContainerName,
	)
	if err != nil {
		// If container creation failed due to name conflict (despite our earlier check),
		// try again with a new unique name as a fallback
		if strings.Contains(err.Error(), "Conflict") || strings.Contains(err.Error(), "already in use") {
			fmt.Printf("Container name conflict detected despite our checks. Trying with a new unique name...\n")
			// Generate a new unique name
			c.Config.ContainerName = fmt.Sprintf("turingpi-image-builder-%d-%x",
				time.Now().UnixNano(), rand.Intn(0x10000))

			// Try again with the new name
			resp, err = c.cli.ContainerCreate(
				ctx,
				&container.Config{
					Image: c.Config.DockerImage,
					Cmd:   []string{"sleep", "infinity"},
					Tty:   true,
				},
				&container.HostConfig{
					Binds:      binds,
					Privileged: c.Config.DockerImage == "turingpi-prepare",
				},
				nil,
				nil,
				c.Config.ContainerName,
			)

			if err != nil {
				return fmt.Errorf("error creating container with fallback name: %w", err)
			}
		} else {
			return fmt.Errorf("error creating container: %w", err)
		}
	}

	c.ContainerID = resp.ID
	fmt.Printf("Created new container: %s (Name: %s)\n", c.ContainerID, c.Config.ContainerName)

	// Start the container with a timeout to prevent hangs
	startCtx, startCancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer startCancel()

	err = c.cli.ContainerStart(startCtx, c.ContainerID, container.StartOptions{})
	if err != nil {
		// Try to clean up the container we just created
		_ = c.cli.ContainerRemove(c.ctx, c.ContainerID, container.RemoveOptions{Force: true})
		return fmt.Errorf("error starting container: %w", err)
	}

	// Add inspection to verify mounts
	containerInfo, err := c.cli.ContainerInspect(c.ctx, c.ContainerID)
	if err != nil {
		fmt.Printf("Warning: Could not inspect container after creation: %v\n", err)
	} else {
		fmt.Printf("DEBUG: Container %s created and started successfully\n", c.ContainerID)
		fmt.Printf("DEBUG: Container has %d mounts configured\n", len(containerInfo.Mounts))
		for i, mount := range containerInfo.Mounts {
			fmt.Printf("DEBUG: Mount %d: Host=%s Container=%s Type=%s\n",
				i+1, mount.Source, mount.Destination, mount.Type)
		}
	}

	return nil
}

// ExecuteCommand runs a command in the Docker container and returns the output
func (c *Container) ExecuteCommand(cmd []string) (string, error) {
	// Check if the container is running first
	containerInfo, err := c.cli.ContainerInspect(c.ctx, c.ContainerID)
	if err != nil {
		return "", fmt.Errorf("error inspecting container: %w", err)
	}

	if !containerInfo.State.Running {
		// Container is not running, try to start it
		fmt.Printf("Container is not running, attempting to start container %s\n", c.ContainerID)
		if err := c.cli.ContainerStart(c.ctx, c.ContainerID, container.StartOptions{}); err != nil {
			return "", fmt.Errorf("error starting container: %w", err)
		}

		// Wait a moment for container to fully start
		time.Sleep(1 * time.Second)
	}

	// Create exec
	execConfig := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	}

	execCreateResp, err := c.cli.ContainerExecCreate(c.ctx, c.ContainerID, execConfig)
	if err != nil {
		return "", fmt.Errorf("error creating exec instance: %w", err)
	}

	// Attach to exec
	execAttachResp, err := c.cli.ContainerExecAttach(c.ctx, execCreateResp.ID, container.ExecStartOptions{
		Tty: false,
	})
	if err != nil {
		return "", fmt.Errorf("error attaching to exec instance: %w", err)
	}
	defer execAttachResp.Close()

	// Read output
	var outBuf, errBuf strings.Builder
	_, err = stdcopy.StdCopy(&outBuf, &errBuf, execAttachResp.Reader)
	if err != nil {
		return "", fmt.Errorf("error reading exec output: %w", err)
	}

	// Check exec exit code
	inspectResp, err := c.cli.ContainerExecInspect(c.ctx, execCreateResp.ID)
	if err != nil {
		return "", fmt.Errorf("error inspecting exec instance: %w", err)
	}

	if inspectResp.ExitCode != 0 {
		return outBuf.String(), fmt.Errorf("command failed with exit code %d: %s",
			inspectResp.ExitCode, errBuf.String())
	}

	return outBuf.String(), nil
}

// CopyFileToContainer copies a file from the host to the container
func (c *Container) CopyFileToContainer(srcPath, destPath string) error {
	// Docker cp command
	dockerCpCmd := exec.Command("docker", "cp", srcPath, fmt.Sprintf("%s:%s", c.ContainerID, destPath))
	output, err := dockerCpCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error copying file to container: %w, output: %s", err, string(output))
	}
	return nil
}

// Cleanup stops and removes the Docker container
func (c *Container) Cleanup() error {
	if c.ContainerID == "" {
		fmt.Println("No container ID to clean up")
		return nil
	}

	// Store containerID for registry cleanup even if removed during cleanup
	containerID := c.ContainerID

	fmt.Printf("Cleaning up container %s (Name: %s)...\n", c.ContainerID, c.Config.ContainerName)

	// Create a context with timeout for cleanup operations
	ctx, cancel := context.WithTimeout(c.ctx, 30*time.Second)
	defer cancel()

	// Check if container exists before trying to stop it
	_, err := c.cli.ContainerInspect(ctx, c.ContainerID)
	if err != nil {
		if client.IsErrNotFound(err) {
			fmt.Printf("Container %s no longer exists, nothing to clean up\n", c.ContainerID)
			// Unregister from registry even though it doesn't exist
			GetRegistry().UnregisterContainer(containerID)
			return nil
		}
		// For other errors, we still try to remove the container
		fmt.Printf("Warning: Error inspecting container %s: %v (will still try to remove)\n", c.ContainerID, err)
	}

	// Stop container with timeout
	stopTimeout := 10 // seconds
	stopErr := c.cli.ContainerStop(ctx, c.ContainerID, container.StopOptions{Timeout: &stopTimeout})
	if stopErr != nil {
		fmt.Printf("Warning: Error stopping container: %v (will still try to force remove)\n", stopErr)
		// Continue to removal even if stop fails
	} else {
		fmt.Printf("Container %s stopped successfully\n", c.ContainerID)
	}

	// Remove container with force option to ensure it's removed even if running
	removeErr := c.cli.ContainerRemove(ctx, c.ContainerID, container.RemoveOptions{Force: true})
	if removeErr != nil {
		// Unregister from registry even if error, as we can't do more
		GetRegistry().UnregisterContainer(containerID)
		return fmt.Errorf("error removing container: %w", removeErr)
	}

	fmt.Printf("Container %s (Name: %s) successfully removed\n", c.ContainerID, c.Config.ContainerName)
	// Unregister from registry after successful cleanup
	GetRegistry().UnregisterContainer(containerID)
	return nil
}

// GetContainerID returns the current container ID
func (c *Container) GetContainerID() string {
	return c.ContainerID
}

// GetContainerName returns the container name
func (c *Container) GetContainerName() string {
	return c.Config.ContainerName
}
