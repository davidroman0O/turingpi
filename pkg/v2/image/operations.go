package imageops

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// ImageOpsImpl implements the ImageOps interface
type ImageOpsImpl struct{}

// WriteToFile writes content to a file at the specified path within the mounted directory
func (i *ImageOpsImpl) WriteToFile(mountDir, path string, content []byte, perm fs.FileMode) error {
	fullPath := filepath.Join(mountDir, path)

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Write the file
	if err := os.WriteFile(fullPath, content, perm); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// CopyFile copies a file from sourcePath to destPath within the mounted directory
func (i *ImageOpsImpl) CopyFile(mountDir, sourcePath, destPath string) error {
	// Open source file
	src, err := os.Open(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer src.Close()

	// Get source file info for permissions
	srcInfo, err := src.Stat()
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}

	// Create destination path
	fullDestPath := filepath.Join(mountDir, destPath)

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullDestPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create destination file
	dst, err := os.OpenFile(fullDestPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dst.Close()

	// Copy the contents
	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	return nil
}

// MakeDirectory creates a directory at the specified path within the mounted directory
func (i *ImageOpsImpl) MakeDirectory(mountDir, path string, perm fs.FileMode) error {
	fullPath := filepath.Join(mountDir, path)
	if err := os.MkdirAll(fullPath, perm); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

// ChangePermissions changes the permissions of a file or directory at the specified path
func (i *ImageOpsImpl) ChangePermissions(mountDir, path string, perm fs.FileMode) error {
	fullPath := filepath.Join(mountDir, path)
	if err := os.Chmod(fullPath, perm); err != nil {
		return fmt.Errorf("failed to change permissions: %w", err)
	}
	return nil
}
