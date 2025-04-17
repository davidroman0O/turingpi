package imageops

import (
	"fmt"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/docker"
	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// InitDockerConfig implements ImageOpsAdapter.InitDockerConfig
func (a *imageOpsAdapter) InitDockerConfig(sourceDir, tempDir, outputDir string) error {
	fmt.Printf("InitDockerConfig called with:\n")
	fmt.Printf("  sourceDir: %s\n", sourceDir)
	fmt.Printf("  tempDir: %s\n", tempDir)
	fmt.Printf("  outputDir: %s\n", outputDir)

	// Create a temporary config first to set the image name
	config := platform.NewDefaultDockerConfig(sourceDir, tempDir, outputDir)

	// Set the image to turingpi-prepare which triggers special handling
	config.DockerImage = "turingpi-prepare"

	fmt.Printf("Docker configuration prepared:\n")
	fmt.Printf("  Image: %s\n", config.DockerImage)
	fmt.Printf("  Container Name: %s\n", config.ContainerName)
	fmt.Printf("  Source Dir: %s\n", config.SourceDir)
	fmt.Printf("  Temp Dir: %s\n", config.TempDir)
	fmt.Printf("  Output Dir: %s\n", config.OutputDir)
	fmt.Printf("  Additional Mounts: %d\n", len(config.AdditionalMounts))

	// Create the adapter with our custom config - with retries
	var err error
	maxRetries := 3
	for retry := 0; retry < maxRetries; retry++ {
		fmt.Printf("Attempting to create Docker adapter (attempt %d/%d)...\n", retry+1, maxRetries)
		a.dockerAdapter, err = docker.NewAdapterWithConfig(config)
		if err == nil {
			break
		}

		if retry < maxRetries-1 {
			waitTime := time.Duration(retry+1) * time.Second
			fmt.Printf("Docker connection attempt %d failed: %v. Retrying in %v...\n",
				retry+1, err, waitTime)
			time.Sleep(waitTime)
		}
	}

	if err != nil {
		// Clear any partially initialized adapter
		a.dockerAdapter = nil
		a.dockerConfig = nil
		a.dockerContainerID = ""
		return fmt.Errorf("critical error: failed to initialize Docker after %d attempts: %w",
			maxRetries, err)
	}

	// Keep the DockerConfig for backward compatibility
	a.dockerConfig = a.dockerAdapter.Container.Config
	a.dockerContainerID = a.dockerAdapter.GetContainerID()

	fmt.Printf("Docker adapter initialized successfully.\n")
	fmt.Printf("  Container ID: %s\n", a.dockerContainerID)
	fmt.Printf("  Container Name: %s\n", a.dockerAdapter.GetContainerName())

	return nil
}

// GetDockerAdapter returns the underlying Docker adapter
func (a *imageOpsAdapter) GetDockerAdapter() *docker.DockerAdapter {
	return a.dockerAdapter
}
