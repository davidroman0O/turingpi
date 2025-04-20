package platform

import (
	"testing"
)

func TestNewDefaultDockerConfig(t *testing.T) {
	sourceDir := "/source"
	tempDir := "/temp"
	outputDir := "/output"

	config := NewDefaultDockerConfig(sourceDir, tempDir, outputDir)

	// Test default values
	if config.DockerImage != "ubuntu:22.04" {
		t.Errorf("Expected default image ubuntu:22.04, got %s", config.DockerImage)
	}
	if config.ContainerName != "turingpi-container" {
		t.Errorf("Expected default container name turingpi-container, got %s", config.ContainerName)
	}
	if !config.UseUniqueContainerName {
		t.Error("Expected UseUniqueContainerName to be true")
	}
	if config.SourceDir != sourceDir {
		t.Errorf("Expected source dir %s, got %s", sourceDir, config.SourceDir)
	}
	if config.TempDir != tempDir {
		t.Errorf("Expected temp dir %s, got %s", tempDir, config.TempDir)
	}
	if config.OutputDir != outputDir {
		t.Errorf("Expected output dir %s, got %s", outputDir, config.OutputDir)
	}
	if config.WorkingDir != "/workspace" {
		t.Errorf("Expected working dir /workspace, got %s", config.WorkingDir)
	}
	if config.Privileged {
		t.Error("Expected Privileged to be false")
	}
	if len(config.Capabilities) != 0 {
		t.Errorf("Expected empty capabilities, got %v", config.Capabilities)
	}
	if config.NetworkMode != "bridge" {
		t.Errorf("Expected network mode bridge, got %s", config.NetworkMode)
	}
}

func TestDockerConfigBuilderPattern(t *testing.T) {
	config := NewDefaultDockerConfig("/source", "/temp", "/output")

	// Test builder pattern methods
	config.WithImage("custom:latest").
		WithName("custom-container").
		WithUniqueName(false).
		WithMount("/host", "/container").
		WithEnv("KEY", "VALUE").
		WithWorkDir("/custom").
		WithPrivileged(true).
		WithCapability("SYS_ADMIN").
		WithNetworkMode("host")

	// Verify modifications
	tests := []struct {
		name   string
		got    interface{}
		want   interface{}
		errMsg string
	}{
		{"Image", config.DockerImage, "custom:latest", "Wrong image"},
		{"Name", config.ContainerName, "custom-container", "Wrong container name"},
		{"UniqueName", config.UseUniqueContainerName, false, "Wrong unique name setting"},
		{"Mount", config.AdditionalMounts["/host"], "/container", "Wrong mount point"},
		{"Env", config.Environment["KEY"], "VALUE", "Wrong environment variable"},
		{"WorkDir", config.WorkingDir, "/custom", "Wrong working directory"},
		{"Privileged", config.Privileged, true, "Wrong privileged setting"},
		{"NetworkMode", config.NetworkMode, "host", "Wrong network mode"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("%s: got %v, want %v", tt.errMsg, tt.got, tt.want)
			}
		})
	}

	// Test capabilities
	if len(config.Capabilities) != 1 || config.Capabilities[0] != "SYS_ADMIN" {
		t.Errorf("Wrong capabilities: got %v, want [SYS_ADMIN]", config.Capabilities)
	}
}

func TestDockerConfigMultipleMounts(t *testing.T) {
	config := NewDefaultDockerConfig("/source", "/temp", "/output")

	// Add multiple mounts
	mounts := map[string]string{
		"/host1": "/container1",
		"/host2": "/container2",
		"/host3": "/container3",
	}

	for host, container := range mounts {
		config.WithMount(host, container)
	}

	// Verify all mounts are present
	if len(config.AdditionalMounts) != len(mounts) {
		t.Errorf("Expected %d mounts, got %d", len(mounts), len(config.AdditionalMounts))
	}

	for host, container := range mounts {
		if got := config.AdditionalMounts[host]; got != container {
			t.Errorf("Mount %s: got %s, want %s", host, got, container)
		}
	}
}

func TestDockerConfigMultipleCapabilities(t *testing.T) {
	config := NewDefaultDockerConfig("/source", "/temp", "/output")

	// Add multiple capabilities
	capabilities := []string{"SYS_ADMIN", "NET_ADMIN", "SYS_PTRACE"}
	for _, cap := range capabilities {
		config.WithCapability(cap)
	}

	// Verify all capabilities are present
	if len(config.Capabilities) != len(capabilities) {
		t.Errorf("Expected %d capabilities, got %d", len(capabilities), len(config.Capabilities))
	}

	for i, cap := range capabilities {
		if config.Capabilities[i] != cap {
			t.Errorf("Capability %d: got %s, want %s", i, config.Capabilities[i], cap)
		}
	}
}

func TestDockerConfigMultipleEnvVars(t *testing.T) {
	config := NewDefaultDockerConfig("/source", "/temp", "/output")

	// Add multiple environment variables
	envVars := map[string]string{
		"KEY1": "VALUE1",
		"KEY2": "VALUE2",
		"KEY3": "VALUE3",
	}

	for key, value := range envVars {
		config.WithEnv(key, value)
	}

	// Verify all environment variables are present
	if len(config.Environment) != len(envVars) {
		t.Errorf("Expected %d environment variables, got %d", len(envVars), len(config.Environment))
	}

	for key, value := range envVars {
		if got := config.Environment[key]; got != value {
			t.Errorf("Environment variable %s: got %s, want %s", key, got, value)
		}
	}
}
