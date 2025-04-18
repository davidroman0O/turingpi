package imageops

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
)

// FileOperation defines the interface for file operations within the image
type FileOperation interface {
	Type() string
	Execute(mountDir string) error
}

// WriteOperation implements FileOperation for writing data to a file
type WriteOperation struct {
	RelativePath string
	Data         []byte
	Perm         os.FileMode
}

func (op WriteOperation) Type() string { return "write" }
func (op WriteOperation) Execute(mountDir string) error {
	filePath := filepath.Join(mountDir, op.RelativePath)
	fmt.Printf("Writing %d bytes to %s (Mode: %o)\n", len(op.Data), filePath, op.Perm)
	return WriteToImageFile(mountDir, op.RelativePath, op.Data, op.Perm)
}

// CopyLocalOperation implements FileOperation for copying files
type CopyLocalOperation struct {
	LocalSourcePath  string
	RelativeDestPath string
}

func (op CopyLocalOperation) Type() string { return "copyLocal" }
func (op CopyLocalOperation) Execute(mountDir string) error {
	return CopyFileToImage(mountDir, op.LocalSourcePath, op.RelativeDestPath)
}

// MkdirOperation implements FileOperation for creating directories
type MkdirOperation struct {
	RelativePath string
	Perm         os.FileMode
}

func (op MkdirOperation) Type() string { return "mkdir" }
func (op MkdirOperation) Execute(mountDir string) error {
	return MkdirInImage(mountDir, op.RelativePath, op.Perm)
}

// ChmodOperation implements FileOperation for changing permissions
type ChmodOperation struct {
	RelativePath string
	Perm         os.FileMode
}

func (op ChmodOperation) Type() string { return "chmod" }
func (op ChmodOperation) Execute(mountDir string) error {
	return ChmodInImage(mountDir, op.RelativePath, op.Perm)
}

// ExecuteFileOperationsParams contains parameters for ExecuteFileOperations
type ExecuteFileOperationsParams struct {
	MountDir   string
	Operations []FileOperation
}

// executeFileOperationsNative executes a batch of file operations using native Linux tools
func executeFileOperationsNative(params ExecuteFileOperationsParams) error {
	for i, op := range params.Operations {
		fmt.Printf("Operation %d/%d: %s\n", i+1, len(params.Operations), op.Type())
		if err := op.Execute(params.MountDir); err != nil {
			return fmt.Errorf("operation %d failed: %w", i+1, err)
		}
	}
	return nil
}

// ExecuteFileOperations implements ImageOpsAdapter.ExecuteFileOperations
func (a *imageOpsAdapter) ExecuteFileOperations(params ExecuteFileOperationsParams) error {
	if len(params.Operations) == 0 {
		fmt.Println("No file operations to execute.")
		return nil
	}

	fmt.Printf("Executing %d file operations...\n", len(params.Operations))

	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && a.isDockerInitialized() {
		fmt.Println("Using Docker for file operations...")

		// Just execute each operation directly since the filesystem is already mounted at /mnt
		for i, op := range params.Operations {
			fmt.Printf("Operation %d/%d: %s\n", i+1, len(params.Operations), op.Type())

			var dockerCmd string

			switch o := op.(type) {
			case WriteOperation:
				// Write file operation
				// Create temp file with content
				tempFile, err := os.CreateTemp("", "docker-file-op-*.tmp")
				if err != nil {
					return fmt.Errorf("failed to create temp file: %w", err)
				}
				tempPath := tempFile.Name()
				defer os.Remove(tempPath)

				if _, err := tempFile.Write(o.Data); err != nil {
					tempFile.Close()
					return fmt.Errorf("failed to write temp file: %w", err)
				}
				tempFile.Close()

				// Copy file to Docker container
				tempFileName := filepath.Base(tempPath)
				copyToDockerCmd := fmt.Sprintf("docker cp %s %s:/tmp/%s",
					tempPath,
					a.dockerAdapter.GetContainerName(),
					tempFileName)

				copyCmd := exec.Command("bash", "-c", copyToDockerCmd)
				if _, err := runCommand(copyCmd); err != nil {
					return fmt.Errorf("failed to copy file to Docker: %w", err)
				}

				// Move file to target in Docker
				dockerCmd = fmt.Sprintf("mkdir -p $(dirname /mnt/%s) && cp /tmp/%s /mnt/%s && chmod %o /mnt/%s",
					o.RelativePath, tempFileName, o.RelativePath, o.Perm, o.RelativePath)

			case MkdirOperation:
				// Mkdir operation
				dockerCmd = fmt.Sprintf("mkdir -p /mnt/%s && chmod %o /mnt/%s",
					o.RelativePath, o.Perm, o.RelativePath)

			case ChmodOperation:
				// Chmod operation
				dockerCmd = fmt.Sprintf("chmod %o /mnt/%s", o.Perm, o.RelativePath)

			case CopyLocalOperation:
				// CopyLocalOperation - need to copy file to container first
				content, err := os.ReadFile(o.LocalSourcePath)
				if err != nil {
					return fmt.Errorf("failed to read local file %s: %w", o.LocalSourcePath, err)
				}

				tempFile, err := os.CreateTemp("", "docker-copy-op-*.tmp")
				if err != nil {
					return fmt.Errorf("failed to create temp file: %w", err)
				}
				tempPath := tempFile.Name()
				defer os.Remove(tempPath)

				if _, err := tempFile.Write(content); err != nil {
					tempFile.Close()
					return fmt.Errorf("failed to write temp file: %w", err)
				}
				tempFile.Close()

				// Copy file to Docker container
				tempFileName := filepath.Base(tempPath)
				copyToDockerCmd := fmt.Sprintf("docker cp %s %s:/tmp/%s",
					tempPath,
					a.dockerAdapter.GetContainerName(),
					tempFileName)

				copyCmd := exec.Command("bash", "-c", copyToDockerCmd)
				if _, err := runCommand(copyCmd); err != nil {
					return fmt.Errorf("failed to copy file to Docker: %w", err)
				}

				// Move file to target in Docker
				dockerCmd = fmt.Sprintf("mkdir -p $(dirname /mnt/%s) && cp /tmp/%s /mnt/%s && chmod 0644 /mnt/%s",
					o.RelativeDestPath, tempFileName, o.RelativeDestPath, o.RelativeDestPath)
			}

			// Execute the operation in Docker
			output, err := a.executeDockerCommand(dockerCmd)
			if err != nil {
				return fmt.Errorf("Docker operation %d (%s) failed: %w\nOutput: %s", i+1, op.Type(), err, output)
			}

			fmt.Printf("Operation completed successfully: %s\n", output)
		}

		return nil
	}

	// Direct execution for Linux
	return executeFileOperationsNative(params)
}

// WriteToImageFile implements ImageOpsAdapter.WriteToImageFile
func (a *imageOpsAdapter) WriteToImageFile(mountDir, relativePath string, content []byte, perm fs.FileMode) error {
	filePath := filepath.Join(mountDir, relativePath)
	fmt.Printf("Writing to file: %s\n", filePath)

	// Create parent directories if they don't exist
	dirPath := filepath.Dir(filePath)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	return writeToFileAsRoot(filePath, content, perm)
}

// CopyFileToImage implements ImageOpsAdapter.CopyFileToImage
func (a *imageOpsAdapter) CopyFileToImage(mountDir, localSourcePath, relativeDestPath string) error {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && a.isDockerInitialized() {
		// This will be handled by ExecuteFileOperations
		// Just create a file operation and execute it
		op := CopyLocalOperation{
			LocalSourcePath:  localSourcePath,
			RelativeDestPath: relativeDestPath,
		}
		return a.ExecuteFileOperations(ExecuteFileOperationsParams{
			MountDir:   mountDir,
			Operations: []FileOperation{op},
		})
	}

	// Read the local file
	content, err := os.ReadFile(localSourcePath)
	if err != nil {
		return fmt.Errorf("failed to read local file %s: %w", localSourcePath, err)
	}

	// Write to the destination in the image
	destPath := filepath.Join(mountDir, relativeDestPath)
	destDir := filepath.Dir(destPath)

	// Create destination directory if it doesn't exist
	cmd := exec.Command("sudo", "mkdir", "-p", destDir)
	if _, err := runCommand(cmd); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destDir, err)
	}

	return writeToFileAsRoot(destPath, content, 0644)
}

// MkdirInImage implements ImageOpsAdapter.MkdirInImage
func (a *imageOpsAdapter) MkdirInImage(mountDir, relativePath string, perm fs.FileMode) error {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && a.isDockerInitialized() {
		// This will be handled by ExecuteFileOperations
		// Just create a file operation and execute it
		op := MkdirOperation{
			RelativePath: relativePath,
			Perm:         perm,
		}
		return a.ExecuteFileOperations(ExecuteFileOperationsParams{
			MountDir:   mountDir,
			Operations: []FileOperation{op},
		})
	}

	dirPath := filepath.Join(mountDir, relativePath)
	cmd := exec.Command("sudo", "mkdir", "-p", dirPath)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	// Set permissions
	chmodCmd := exec.Command("sudo", "chmod", fmt.Sprintf("%o", perm), dirPath)
	_, err = runCommand(chmodCmd)
	if err != nil {
		return fmt.Errorf("failed to set directory permissions: %w", err)
	}

	return nil
}

// ChmodInImage implements ImageOpsAdapter.ChmodInImage
func (a *imageOpsAdapter) ChmodInImage(mountDir, relativePath string, perm fs.FileMode) error {
	// For non-Linux platforms, use Docker
	if !platform.IsLinux() && a.isDockerInitialized() {
		// This will be handled by ExecuteFileOperations
		// Just create a file operation and execute it
		op := ChmodOperation{
			RelativePath: relativePath,
			Perm:         perm,
		}
		return a.ExecuteFileOperations(ExecuteFileOperationsParams{
			MountDir:   mountDir,
			Operations: []FileOperation{op},
		})
	}

	filePath := filepath.Join(mountDir, relativePath)
	cmd := exec.Command("sudo", "chmod", fmt.Sprintf("%o", perm), filePath)
	_, err := runCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to set permissions: %w", err)
	}

	return nil
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
