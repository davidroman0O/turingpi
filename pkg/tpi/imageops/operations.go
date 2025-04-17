package imageops

import (
	"fmt"
	"os"
	"path/filepath"
)

// FileOperation defines an interface for operations that can be performed on files
type FileOperation interface {
	Type() string
	Execute(mountDir string) error
}

// WriteOperation represents a file write operation
type WriteOperation struct {
	RelativePath string
	Data         []byte
	Perm         os.FileMode
}

func (op WriteOperation) Type() string { return "write" }

func (op WriteOperation) Execute(mountDir string) error {
	fullPath := filepath.Join(mountDir, op.RelativePath)
	return writeToFileAsRoot(fullPath, op.Data, op.Perm)
}

// CopyLocalOperation represents a local file copy operation
type CopyLocalOperation struct {
	LocalSourcePath  string
	RelativeDestPath string
}

func (op CopyLocalOperation) Type() string { return "copyLocal" }

func (op CopyLocalOperation) Execute(mountDir string) error {
	destPath := filepath.Join(mountDir, op.RelativeDestPath)
	return copyFile(op.LocalSourcePath, destPath)
}

// MkdirOperation represents a directory creation operation
type MkdirOperation struct {
	RelativePath string
	Perm         os.FileMode
}

func (op MkdirOperation) Type() string { return "mkdir" }

func (op MkdirOperation) Execute(mountDir string) error {
	fullPath := filepath.Join(mountDir, op.RelativePath)
	return os.MkdirAll(fullPath, op.Perm)
}

// ChmodOperation represents a permission change operation
type ChmodOperation struct {
	RelativePath string
	Perm         os.FileMode
}

func (op ChmodOperation) Type() string { return "chmod" }

func (op ChmodOperation) Execute(mountDir string) error {
	fullPath := filepath.Join(mountDir, op.RelativePath)
	return os.Chmod(fullPath, op.Perm)
}

// ExecuteFileOperationsParams contains parameters for executing file operations
type ExecuteFileOperationsParams struct {
	MountDir   string
	Operations []FileOperation
}

// ExecuteFileOperations executes a list of file operations on a mounted filesystem
func ExecuteFileOperations(params ExecuteFileOperationsParams) error {
	if params.MountDir == "" {
		return fmt.Errorf("mount directory is required")
	}

	if len(params.Operations) == 0 {
		return nil // No operations to perform
	}

	// Execute each operation in sequence
	for i, op := range params.Operations {
		if err := op.Execute(params.MountDir); err != nil {
			return fmt.Errorf("operation %d (%s) failed: %w", i, op.Type(), err)
		}
	}

	return nil
}
