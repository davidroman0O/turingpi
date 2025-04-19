package imageops

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/docker"
	"github.com/davidroman0O/turingpi/pkg/tpi/imageops/ops"
	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
	"github.com/docker/docker/api/types/mount"
)

// dockerInitTimeout is the timeout for Docker initialization operations
const dockerInitTimeout = 30 * time.Second

// dockerCommandTimeout is the timeout for Docker command execution
const dockerCommandTimeout = 10 * time.Minute

// initDocker initializes the Docker configuration for cross-platform operations
func (a *imageOpsAdapter) initDocker(sourceDir, tempDir, outputDir string) error {
	// Create a context with timeout for initialization
	ctx, cancel := context.WithTimeout(context.Background(), dockerInitTimeout)
	defer cancel()

	log.Printf("InitDockerConfig called with:\n")
	log.Printf("  sourceDir: %s\n", sourceDir)
	log.Printf("  tempDir: %s\n", tempDir)
	log.Printf("  outputDir: %s\n", outputDir)

	// Create a temporary config first to set the image name
	containerName := fmt.Sprintf("turingpi-prep-%d", time.Now().UnixNano())

	// Convert paths to absolute paths
	sourceAbs, err := filepath.Abs(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for source dir: %w", err)
	}
	tempAbs, err := filepath.Abs(tempDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for temp dir: %w", err)
	}
	outputAbs, err := filepath.Abs(outputDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for output dir: %w", err)
	}

	config := &ops.DockerConfig{
		Image:           "turingpi-prepare",
		ContainerName:   containerName,
		NetworkDisabled: true,
		WorkingDir:      "/workspace",
		SourceDir:       sourceAbs,
		TempDir:         tempAbs,
		OutputDir:       outputAbs,
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: sourceAbs,
				Target: "/source",
			},
			{
				Type:   mount.TypeBind,
				Source: tempAbs,
				Target: "/temp",
			},
			{
				Type:   mount.TypeBind,
				Source: outputAbs,
				Target: "/output",
			},
		},
	}

	// Create the Docker adapter with retries
	var lastErr error
	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		select {
		case <-ctx.Done():
			return fmt.Errorf("Docker initialization timed out after %v", dockerInitTimeout)
		default:
			log.Printf("Attempting to create Docker client (attempt %d/%d)...\n", retry+1, maxRetries)

			// Create a Docker configuration that uses the turingpi-prepare image
			dockerConfig := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)
			dockerConfig.DockerImage = "turingpi-prepare"

			adapter, err := docker.NewAdapter(ctx, sourceDir, tempDir, outputDir)
			if err == nil {
				// Store the adapter and config
				a.dockerAdapter = adapter
				a.dockerConfig = config
				log.Printf("Docker client initialized successfully")
				return nil
			}

			lastErr = fmt.Errorf("failed to create Docker adapter: %w", err)

			if retry < maxRetries-1 {
				waitTime := time.Duration(retry+1) * time.Second
				log.Printf("Docker connection attempt %d failed: %v. Retrying in %v...\n",
					retry+1, lastErr, waitTime)
				time.Sleep(waitTime)
			}
		}
	}

	return fmt.Errorf("critical error: failed to initialize Docker after %d attempts: %w",
		maxRetries, lastErr)
}

// executeDockerCommand executes a command in the Docker container with timeout
func (a *imageOpsAdapter) executeDockerCommand(command string) (string, error) {
	if !a.isDockerInitialized() {
		// Try to reinitialize Docker if it's not initialized
		if err := a.initDocker(a.sourceDir, a.tempDir, a.outputDir); err != nil {
			return "", fmt.Errorf("failed to reinitialize Docker: %w", err)
		}
	}

	// Create context with timeout for command execution
	ctx, cancel := context.WithTimeout(context.Background(), dockerCommandTimeout)
	defer cancel()

	// Log command execution
	log.Printf("Executing Docker command: %s\n", command)

	// Use the Docker adapter to execute the command
	if a.dockerAdapter != nil {
		return a.dockerAdapter.ExecuteCommand(ctx, command)
	}

	return "", fmt.Errorf("Docker adapter is not initialized")
}

// isDockerInitialized checks if Docker is properly initialized
func (a *imageOpsAdapter) isDockerInitialized() bool {
	return a.dockerAdapter != nil && a.dockerConfig != nil
}

// cleanupDocker ensures proper cleanup of Docker resources
func (a *imageOpsAdapter) cleanupDocker() error {
	if !a.isDockerInitialized() {
		return nil
	}

	log.Printf("Cleaning up Docker resources...")

	if a.dockerAdapter != nil {
		if err := a.dockerAdapter.Close(); err != nil {
			log.Printf("Warning: Failed to close Docker adapter: %v", err)
		}
		a.dockerAdapter = nil
	}

	a.dockerConfig = nil
	return nil
}
