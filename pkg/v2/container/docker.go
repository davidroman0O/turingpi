package container

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// DockerContainer implements the Container interface for Docker
type DockerContainer struct {
	id     string
	client *client.Client
	config ContainerConfig
}

// ID implements Container.ID
func (c *DockerContainer) ID() string {
	return c.id
}

// Start implements Container.Start
func (c *DockerContainer) Start(ctx context.Context) error {
	return c.client.ContainerStart(ctx, c.id, container.StartOptions{})
}

// Stop implements Container.Stop
func (c *DockerContainer) Stop(ctx context.Context) error {
	timeout := 10 // seconds
	return c.client.ContainerStop(ctx, c.id, container.StopOptions{Timeout: &timeout})
}

// Kill implements Container.Kill
func (c *DockerContainer) Kill(ctx context.Context) error {
	return c.client.ContainerKill(ctx, c.id, "SIGKILL")
}

// Pause implements Container.Pause
func (c *DockerContainer) Pause(ctx context.Context) error {
	return c.client.ContainerPause(ctx, c.id)
}

// Unpause implements Container.Unpause
func (c *DockerContainer) Unpause(ctx context.Context) error {
	return c.client.ContainerUnpause(ctx, c.id)
}

// Exec implements Container.Exec
func (c *DockerContainer) Exec(ctx context.Context, cmd []string) (string, error) {
	// Create exec
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: true,
		AttachStderr: true,
	}
	execResp, err := c.client.ContainerExecCreate(ctx, c.id, execConfig)
	if err != nil {
		return "", fmt.Errorf("failed to create exec: %w", err)
	}

	// Attach to exec - using the correct pattern with ContainerExecAttach for attached execution
	resp, err := c.client.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{
		// Not setting Detach here since ContainerExecAttach doesn't support detached mode
		Tty: false,
	})
	if err != nil {
		return "", fmt.Errorf("failed to attach to exec: %w", err)
	}
	defer resp.Close()

	// Read output
	var outBuf, errBuf strings.Builder
	_, err = stdcopy.StdCopy(&outBuf, &errBuf, resp.Reader)
	if err != nil {
		return "", fmt.Errorf("failed to read exec output: %w", err)
	}

	// Check exit code
	inspectResp, err := c.client.ContainerExecInspect(ctx, execResp.ID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect exec: %w", err)
	}

	if inspectResp.ExitCode != 0 {
		return "", fmt.Errorf("command failed with exit code %d: %s", inspectResp.ExitCode, errBuf.String())
	}

	return outBuf.String(), nil
}

// ExecDetached implements Container.ExecDetached
func (c *DockerContainer) ExecDetached(ctx context.Context, cmd []string) error {
	// Create exec
	execConfig := container.ExecOptions{
		Cmd:          cmd,
		AttachStdout: false,
		AttachStderr: false,
		Detach:       true,
	}
	execResp, err := c.client.ContainerExecCreate(ctx, c.id, execConfig)
	if err != nil {
		return fmt.Errorf("failed to create exec: %w", err)
	}

	// Start the exec instance in detached mode
	err = c.client.ContainerExecStart(ctx, execResp.ID, container.ExecStartOptions{
		Detach: true,
	})
	if err != nil {
		return fmt.Errorf("failed to start exec: %w", err)
	}

	return nil
}

// CopyTo implements Container.CopyTo
func (c *DockerContainer) CopyTo(ctx context.Context, hostPath, containerPath string) error {
	// Read the source file
	srcInfo, err := os.Stat(hostPath)
	if err != nil {
		return fmt.Errorf("failed to stat source path: %w", err)
	}

	// Create a tar header
	header := &tar.Header{
		Name:    filepath.Base(containerPath),
		Size:    srcInfo.Size(),
		Mode:    int64(srcInfo.Mode()),
		ModTime: srcInfo.ModTime(),
	}

	// Create a pipe
	pr, pw := io.Pipe()

	// Start copying in a goroutine
	go func() {
		tw := tar.NewWriter(pw)
		file, err := os.Open(hostPath)
		if err != nil {
			pw.CloseWithError(err)
			return
		}
		defer file.Close()

		if err := tw.WriteHeader(header); err != nil {
			pw.CloseWithError(err)
			return
		}
		if _, err := io.Copy(tw, file); err != nil {
			pw.CloseWithError(err)
			return
		}
		if err := tw.Close(); err != nil {
			pw.CloseWithError(err)
			return
		}
		pw.Close()
	}()

	// Copy to container
	return c.client.CopyToContainer(ctx, c.id, filepath.Dir(containerPath), pr, container.CopyToContainerOptions{
		AllowOverwriteDirWithFile: true,
	})
}

// CopyFrom implements Container.CopyFrom
func (c *DockerContainer) CopyFrom(ctx context.Context, containerPath, hostPath string) error {
	// Get content from container
	reader, _, err := c.client.CopyFromContainer(ctx, c.id, containerPath)
	if err != nil {
		return fmt.Errorf("failed to copy from container: %w", err)
	}
	defer reader.Close()

	// Create the destination directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(hostPath), 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Extract the tar stream
	tr := tar.NewReader(reader)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar header: %w", err)
		}

		// Create the file
		file, err := os.Create(hostPath)
		if err != nil {
			return fmt.Errorf("failed to create destination file: %w", err)
		}
		defer file.Close()

		if _, err := io.Copy(file, tr); err != nil {
			return fmt.Errorf("failed to write file content: %w", err)
		}

		// Set file permissions
		if err := os.Chmod(hostPath, os.FileMode(header.Mode)); err != nil {
			return fmt.Errorf("failed to set file permissions: %w", err)
		}
	}

	return nil
}

// Logs implements Container.Logs
func (c *DockerContainer) Logs(ctx context.Context) (io.ReadCloser, error) {
	return c.client.ContainerLogs(ctx, c.id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
}

// Wait implements Container.Wait
func (c *DockerContainer) Wait(ctx context.Context) (int, error) {
	statusCh, errCh := c.client.ContainerWait(ctx, c.id, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return -1, err
	case status := <-statusCh:
		return int(status.StatusCode), nil
	}
}

// Cleanup implements Container.Cleanup
func (c *DockerContainer) Cleanup(ctx context.Context) error {
	return c.client.ContainerRemove(ctx, c.id, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
}

// convertConfig converts our ContainerConfig to Docker's container.Config
func convertConfig(cfg ContainerConfig) (*container.Config, *container.HostConfig) {
	// Convert environment variables
	env := make([]string, 0, len(cfg.Env))
	for k, v := range cfg.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	// Create container config
	containerConfig := &container.Config{
		Image:      cfg.Image,
		Cmd:        cfg.Command,
		Env:        env,
		WorkingDir: cfg.WorkDir,
	}

	// Convert volume mounts
	binds := make([]string, 0, len(cfg.Mounts))
	for hostPath, containerPath := range cfg.Mounts {
		binds = append(binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// Create host config with resources mapping to Docker SDK v28 fields
	hostConfig := &container.HostConfig{
		Binds:       binds,
		NetworkMode: container.NetworkMode(cfg.NetworkMode),
		Privileged:  cfg.Privileged,
		CapAdd:      cfg.Capabilities,
		Resources: container.Resources{
			CPUShares:   cfg.Resources.CPUShares,
			CPUPeriod:   cfg.Resources.CPUPeriod,
			CPUQuota:    cfg.Resources.CPUQuota,
			CpusetCpus:  cfg.Resources.CPUSetCPUs,
			CpusetMems:  cfg.Resources.CPUSetMems,
			Memory:      cfg.Resources.Memory,
			MemorySwap:  cfg.Resources.MemorySwap,
			BlkioWeight: cfg.Resources.BlkioWeight,
		},
	}

	return containerConfig, hostConfig
}
