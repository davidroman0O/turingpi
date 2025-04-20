package tools

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/davidroman0O/turingpi/pkg/v2/cache"
)

// FSToolImpl is the implementation of the FSTool interface
type FSToolImpl struct{}

// NewFSTool creates a new FSTool
func NewFSTool() FSTool {
	return &FSToolImpl{}
}

// CreateDir creates a directory
func (t *FSToolImpl) CreateDir(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

// WriteFile writes content to a file
func (t *FSToolImpl) WriteFile(path string, content []byte, perm fs.FileMode) error {
	// Ensure parent directory exists
	parentDir := filepath.Dir(path)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	return os.WriteFile(path, content, perm)
}

// ReadFile reads a file's content
func (t *FSToolImpl) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// FileExists checks if a file exists
func (t *FSToolImpl) FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// CopyFile copies a file
func (t *FSToolImpl) CopyFile(src, dst string) error {
	// Open source file
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy the contents
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Get source file mode
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to get source file info: %w", err)
	}

	// Set same mode on destination
	return os.Chmod(dst, srcInfo.Mode())
}

// RemoveFile removes a file
func (t *FSToolImpl) RemoveFile(path string) error {
	return os.Remove(path)
}

// CalculateFileHash computes a hash for a file
func (t *FSToolImpl) CalculateFileHash(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file for hashing: %w", err)
	}
	defer file.Close()

	return cache.GenerateContentHash(file)
}
