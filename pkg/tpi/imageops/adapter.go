package imageops

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/docker"
	"github.com/davidroman0O/turingpi/pkg/tpi/imageops/ops"
	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
	"github.com/docker/docker/client"
)

// ImageOpsAdapter defines the interface for image operations
type ImageOpsAdapter interface {
	// PrepareImage prepares an image with the given options
	PrepareImage(ctx context.Context, opts ops.PrepareImageOptions) error

	// ExecuteFileOperations executes a series of file operations on the image
	ExecuteFileOperations(ctx context.Context, params ops.ExecuteParams) error

	// Cleanup performs any necessary cleanup
	Cleanup(ctx context.Context) error

	// Internal methods for image operations
	MapPartitions(imgPathAbs string) (string, error)
	CleanupPartitions(imgPathAbs string) error
	MountFilesystem(partitionDevice, mountDir string) error
	UnmountFilesystem(mountDir string) error
	ApplyNetworkConfig(mountDir string, hostname string, ipCIDR string, gateway string, dnsServers []string) error
	DecompressImageXZ(sourceImgXZAbs, tmpDir string) (string, error)
	RecompressImageXZ(modifiedImgPath, finalXZPath string) error
	WriteToImageFile(mountDir, relativePath string, content []byte, perm fs.FileMode) error
	CopyFileToImage(mountDir, localSourcePath, relativeDestPath string) error
	MkdirInImage(mountDir, relativePath string, perm fs.FileMode) error
	ChmodInImage(mountDir, relativePath string, perm fs.FileMode) error
}

// imageOpsAdapter implements the ImageOpsAdapter interface
type imageOpsAdapter struct {
	dockerClient  *client.Client
	dockerAdapter *docker.DockerAdapter
	dockerConfig  *ops.DockerConfig
	startTime     time.Time
	sourceDir     string
	tempDir       string
	outputDir     string
}

// NewImageOpsAdapter creates a new adapter for image operations
func NewImageOpsAdapter(sourceDir, tempDir, outputDir string) (ImageOpsAdapter, error) {
	adapter := &imageOpsAdapter{
		startTime: time.Now(),
		sourceDir: sourceDir,
		tempDir:   tempDir,
		outputDir: outputDir,
	}

	// Initialize Docker for non-Linux platforms
	if !platform.IsLinux() {
		if err := adapter.initDocker(sourceDir, tempDir, outputDir); err != nil {
			return nil, err
		}
	}

	return adapter, nil
}

func (a *imageOpsAdapter) ExecuteFileOperations(ctx context.Context, params ops.ExecuteParams) error {
	return ops.Execute(params)
}

func (a *imageOpsAdapter) Cleanup(ctx context.Context) error {
	if a.dockerAdapter != nil {
		return a.dockerAdapter.Close()
	}
	return nil
}

func (a *imageOpsAdapter) WriteToImageFile(mountDir, relativePath string, content []byte, perm fs.FileMode) error {
	filePath := filepath.Join(mountDir, relativePath)
	return writeToFileAsRoot(filePath, content, perm)
}

func (a *imageOpsAdapter) CopyFileToImage(mountDir, localSourcePath, relativeDestPath string) error {
	content, err := os.ReadFile(localSourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}
	return a.WriteToImageFile(mountDir, relativeDestPath, content, 0644)
}

func (a *imageOpsAdapter) MkdirInImage(mountDir, relativePath string, perm fs.FileMode) error {
	dirPath := filepath.Join(mountDir, relativePath)
	cmd := exec.Command("sudo", "mkdir", "-p", dirPath)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Set permissions
	return a.ChmodInImage(mountDir, relativePath, perm)
}

func (a *imageOpsAdapter) ChmodInImage(mountDir, relativePath string, perm fs.FileMode) error {
	path := filepath.Join(mountDir, relativePath)
	cmd := exec.Command("sudo", "chmod", fmt.Sprintf("%o", perm), path)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}
	return nil
}

// runCommand runs a command and returns its output
func runCommand(cmd *exec.Cmd) ([]byte, error) {
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// writeToFileAsRoot writes content to a file as root
func writeToFileAsRoot(filePath string, content []byte, perm fs.FileMode) error {
	// Create temp file
	tempFile, err := os.CreateTemp(filepath.Dir(filePath), "tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	// Write content to temp file
	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	tempFile.Close()

	// Set permissions on temp file
	if err := os.Chmod(tempPath, perm); err != nil {
		return fmt.Errorf("failed to set temp file permissions: %w", err)
	}

	// Move temp file to target path (requires root)
	cmd := exec.Command("sudo", "mv", tempPath, filePath)
	_, err = runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	return nil
}
