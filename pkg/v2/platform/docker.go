package platform

// DockerExecutionConfig holds configuration for Docker container execution
type DockerExecutionConfig struct {
	// Docker image to use
	DockerImage string

	// Name for the container
	ContainerName string

	// Whether to append a unique suffix to the container name
	UseUniqueContainerName bool

	// Source directory to mount in container
	SourceDir string

	// Temporary directory to mount in container
	TempDir string

	// Output directory to mount in container
	OutputDir string

	// Additional mount points (host path -> container path)
	AdditionalMounts map[string]string

	// Environment variables to set in container
	Environment map[string]string

	// Working directory in container
	WorkingDir string

	// Whether to run container in privileged mode
	Privileged bool

	// Additional capabilities to add
	Capabilities []string

	// Network mode (e.g., "host", "none", "bridge")
	NetworkMode string
}

// NewDefaultDockerConfig creates a new DockerExecutionConfig with default values
func NewDefaultDockerConfig(sourceDir, tempDir, outputDir string) *DockerExecutionConfig {
	return &DockerExecutionConfig{
		DockerImage:            "ubuntu:22.04",
		ContainerName:          "turingpi-container",
		UseUniqueContainerName: true,
		SourceDir:              sourceDir,
		TempDir:                tempDir,
		OutputDir:              outputDir,
		AdditionalMounts:       make(map[string]string),
		Environment:            make(map[string]string),
		WorkingDir:             "/workspace",
		Privileged:             false,
		Capabilities:           []string{},
		NetworkMode:            "bridge",
	}
}

// WithImage sets the Docker image
func (c *DockerExecutionConfig) WithImage(image string) *DockerExecutionConfig {
	c.DockerImage = image
	return c
}

// WithName sets the container name
func (c *DockerExecutionConfig) WithName(name string) *DockerExecutionConfig {
	c.ContainerName = name
	return c
}

// WithUniqueName sets whether to use a unique container name
func (c *DockerExecutionConfig) WithUniqueName(unique bool) *DockerExecutionConfig {
	c.UseUniqueContainerName = unique
	return c
}

// WithMount adds a mount point
func (c *DockerExecutionConfig) WithMount(hostPath, containerPath string) *DockerExecutionConfig {
	c.AdditionalMounts[hostPath] = containerPath
	return c
}

// WithEnv adds an environment variable
func (c *DockerExecutionConfig) WithEnv(key, value string) *DockerExecutionConfig {
	c.Environment[key] = value
	return c
}

// WithWorkDir sets the working directory
func (c *DockerExecutionConfig) WithWorkDir(dir string) *DockerExecutionConfig {
	c.WorkingDir = dir
	return c
}

// WithPrivileged sets privileged mode
func (c *DockerExecutionConfig) WithPrivileged(privileged bool) *DockerExecutionConfig {
	c.Privileged = privileged
	return c
}

// WithCapability adds a capability
func (c *DockerExecutionConfig) WithCapability(cap string) *DockerExecutionConfig {
	c.Capabilities = append(c.Capabilities, cap)
	return c
}

// WithNetworkMode sets the network mode
func (c *DockerExecutionConfig) WithNetworkMode(mode string) *DockerExecutionConfig {
	c.NetworkMode = mode
	return c
}
