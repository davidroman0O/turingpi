package imageops

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/docker"
	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// containerID stores the ID of the persistent container we create for operations
var containerID string

// initDocker initializes the Docker configuration for cross-platform operations
func (a *imageOpsAdapter) initDocker(sourceDir, tempDir, outputDir string) error {
	// We'll use the Docker adapter to manage Docker resources
	var err error

	log.Printf("InitDockerConfig called with:\n")
	log.Printf("  sourceDir: %s\n", sourceDir)
	log.Printf("  tempDir: %s\n", tempDir)
	log.Printf("  outputDir: %s\n", outputDir)

	// Create a temporary config first to set the image name
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)

	// Set the image to turingpi-prepare which triggers special handling
	config.DockerImage = "turingpi-prepare"

	// Ensure we use a unique container name to avoid conflicts
	config.UseUniqueContainerName = true

	log.Printf("Docker configuration prepared:\n")
	log.Printf("  Image: %s\n", config.DockerImage)
	log.Printf("  Container Name: %s\n", config.ContainerName)
	log.Printf("  Source Dir: %s\n", config.SourceDir)
	log.Printf("  Temp Dir: %s\n", config.TempDir)
	log.Printf("  Output Dir: %s\n", config.OutputDir)
	log.Printf("  Additional Mounts: %d\n", len(config.AdditionalMounts))

	// Create the adapter with our custom config - with retries
	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		log.Printf("Attempting to create Docker adapter (attempt %d/%d)...\n", retry+1, maxRetries)
		ctx := context.Background()
		a.dockerAdapter, err = docker.NewAdapter(ctx, sourceDir, tempDir, outputDir)
		if err == nil {
			break
		}

		if retry < maxRetries-1 {
			waitTime := time.Duration(retry+1) * time.Second
			log.Printf("Docker connection attempt %d failed: %v. Retrying in %v...\n",
				retry+1, err, waitTime)
			time.Sleep(waitTime)
		}
	}

	if err != nil {
		// Clear any partially initialized adapter
		if a.dockerAdapter != nil {
			ctx := context.Background()
			a.dockerAdapter.Cleanup(ctx)
		}
		a.dockerAdapter = nil
		a.dockerConfig = nil
		containerID = ""
		return fmt.Errorf("critical error: failed to initialize Docker after %d attempts: %w",
			maxRetries, err)
	}

	// Keep the DockerConfig for backward compatibility
	a.dockerConfig = a.dockerAdapter.Container.Config
	containerID = a.dockerAdapter.GetContainerID()

	log.Printf("Docker adapter initialized successfully.\n")
	log.Printf("  Container ID: %s\n", containerID)
	log.Printf("  Container Name: %s\n", a.dockerAdapter.GetContainerName())

	return nil
}

// executeDockerCommand executes a command in the Docker container
func (a *imageOpsAdapter) executeDockerCommand(command string) (string, error) {
	if a.dockerAdapter == nil {
		return "", fmt.Errorf("Docker adapter not initialized but required for command execution")
	}

	ctx := context.Background()
	return a.dockerAdapter.ExecuteCommand(ctx, command)
}

// isDockerInitialized checks if Docker is properly initialized
func (a *imageOpsAdapter) isDockerInitialized() bool {
	return a.dockerAdapter != nil && a.dockerConfig != nil
}
