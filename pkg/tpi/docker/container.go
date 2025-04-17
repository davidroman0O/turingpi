package docker

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
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

	return c, nil
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

# Keep container running
ENTRYPOINT ["sleep", "infinity"]
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
	// Prepare mount bindings
	binds := []string{}

	// Special handling for turingpi-prepare to maintain compatibility with old code
	if c.Config.DockerImage == "turingpi-prepare" {
		binds = []string{
			fmt.Sprintf("%s:/images:ro", c.Config.SourceDir),
			fmt.Sprintf("%s:/tmp", c.Config.TempDir),
			fmt.Sprintf("%s:/prepared-images", c.Config.OutputDir),
		}
	} else {
		// Standard mounts for other images
		// Add the standard mounts (source, temp, output directories)
		if c.Config.SourceDir != "" {
			binds = append(binds, fmt.Sprintf("%s:/source:ro", c.Config.SourceDir))
		}
		if c.Config.TempDir != "" {
			binds = append(binds, fmt.Sprintf("%s:/tmp", c.Config.TempDir))
		}
		if c.Config.OutputDir != "" {
			binds = append(binds, fmt.Sprintf("%s:/output", c.Config.OutputDir))
		}
	}

	// Add any additional mounts
	for hostPath, containerPath := range c.Config.AdditionalMounts {
		binds = append(binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// Create container
	resp, err := c.cli.ContainerCreate(
		c.ctx,
		&container.Config{
			Image: c.Config.DockerImage,
			Cmd:   []string{"sleep", "infinity"}, // Keep container running
			Tty:   true,
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
		return fmt.Errorf("error creating container: %w", err)
	}

	c.ContainerID = resp.ID

	// Start the container
	if err := c.cli.ContainerStart(c.ctx, c.ContainerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("error starting container: %w", err)
	}

	return nil
}

// ExecuteCommand runs a command in the Docker container and returns the output
func (c *Container) ExecuteCommand(cmd []string) (string, error) {
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
	// Stop container
	timeout := 10 // seconds
	err := c.cli.ContainerStop(c.ctx, c.ContainerID, container.StopOptions{Timeout: &timeout})
	if err != nil {
		return fmt.Errorf("error stopping container: %w", err)
	}

	// Remove container
	err = c.cli.ContainerRemove(c.ctx, c.ContainerID, container.RemoveOptions{Force: true})
	if err != nil {
		return fmt.Errorf("error removing container: %w", err)
	}

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
