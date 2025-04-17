package imageops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewImageOpsAdapter(t *testing.T) {
	adapter := NewImageOpsAdapter()
	if adapter == nil {
		t.Fatal("Expected non-nil adapter")
	}
}

func TestImageOpsAdapter_InitDockerConfig(t *testing.T) {
	adapter := NewImageOpsAdapter()

	// Create temporary directories for testing
	sourceDir, err := os.MkdirTemp("", "turingpi-test-source-*")
	if err != nil {
		t.Fatalf("Failed to create temp source dir: %v", err)
	}
	defer os.RemoveAll(sourceDir)

	tempDir, err := os.MkdirTemp("", "turingpi-test-temp-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	outputDir, err := os.MkdirTemp("", "turingpi-test-output-*")
	if err != nil {
		t.Fatalf("Failed to create temp output dir: %v", err)
	}
	defer os.RemoveAll(outputDir)

	err = adapter.InitDockerConfig(sourceDir, tempDir, outputDir)
	if err != nil {
		t.Fatalf("InitDockerConfig failed: %v", err)
	}
}

func TestImageOpsAdapter_PrepareImage(t *testing.T) {
	adapter := NewImageOpsAdapter()

	// Create temporary directories for testing
	tempDir, err := os.MkdirTemp("", "turingpi-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a dummy source image
	sourceImgPath := filepath.Join(tempDir, "source.img")
	if err := os.WriteFile(sourceImgPath, []byte("dummy image data"), 0644); err != nil {
		t.Fatalf("Failed to create dummy source image: %v", err)
	}

	// Compress the dummy image
	sourceImgXZPath := sourceImgPath + ".xz"
	if err := RecompressImageXZ(sourceImgPath, sourceImgXZPath); err != nil {
		t.Fatalf("Failed to compress dummy image: %v", err)
	}

	// Initialize Docker config
	if err := adapter.InitDockerConfig(tempDir, tempDir, tempDir); err != nil {
		t.Fatalf("InitDockerConfig failed: %v", err)
	}

	// Test PrepareImage
	opts := PrepareImageOptions{
		SourceImgXZ:  sourceImgXZPath,
		NodeNum:      1,
		IPAddress:    "192.168.1.100",
		IPCIDRSuffix: "/24",
		Hostname:     "node1",
		Gateway:      "192.168.1.1",
		DNSServers:   []string{"8.8.8.8", "8.8.4.4"},
		OutputDir:    tempDir,
		TempDir:      tempDir,
	}

	outputPath, err := adapter.PrepareImage(opts)
	if err != nil {
		t.Fatalf("PrepareImage failed: %v", err)
	}

	if _, err := os.Stat(outputPath); err != nil {
		t.Fatalf("Output file not found: %v", err)
	}
}
