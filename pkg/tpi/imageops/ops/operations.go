package ops

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteToFile writes content to a file within the mounted image
func WriteToFile(mountDir, relativePath string, content []byte, perm fs.FileMode) error {
	filePath := filepath.Join(mountDir, relativePath)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return fmt.Errorf("failed to create parent directories: %w", err)
	}
	return os.WriteFile(filePath, content, perm)
}

// CopyFile copies a local file into the mounted image
func CopyFile(mountDir, localSourcePath, relativeDestPath string) error {
	content, err := os.ReadFile(localSourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}
	return WriteToFile(mountDir, relativeDestPath, content, 0644)
}

// MkdirAll creates a directory within the mounted image
func MkdirAll(mountDir, relativePath string, perm fs.FileMode) error {
	dirPath := filepath.Join(mountDir, relativePath)
	return os.MkdirAll(dirPath, perm)
}

// Chmod changes permissions of a file/directory within the mounted image
func Chmod(mountDir, relativePath string, perm fs.FileMode) error {
	path := filepath.Join(mountDir, relativePath)
	return os.Chmod(path, perm)
}

// Operation implementations

func (w WriteOperation) Type() string {
	return "write"
}

func (w WriteOperation) Execute(mountDir string) error {
	return WriteToFile(mountDir, w.Path, w.Content, w.FileMode)
}

func (w WriteOperation) Verify(mountDir string) error {
	path := filepath.Join(mountDir, w.Path)
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file for verification: %w", err)
	}
	if string(content) != string(w.Content) {
		return fmt.Errorf("content mismatch in file %s", w.Path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file for verification: %w", err)
	}
	if info.Mode().Perm() != w.FileMode.Perm() {
		return fmt.Errorf("file mode mismatch in file %s: got %v, want %v", w.Path, info.Mode().Perm(), w.FileMode.Perm())
	}
	return nil
}

func (c CopyOperation) Type() string {
	return "copy"
}

func (c CopyOperation) Execute(mountDir string) error {
	return CopyFile(mountDir, c.SourcePath, c.DestPath)
}

func (c CopyOperation) Verify(mountDir string) error {
	sourceContent, err := os.ReadFile(c.SourcePath)
	if err != nil {
		return fmt.Errorf("failed to read source file for verification: %w", err)
	}
	destPath := filepath.Join(mountDir, c.DestPath)
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		return fmt.Errorf("failed to read destination file for verification: %w", err)
	}
	if string(sourceContent) != string(destContent) {
		return fmt.Errorf("content mismatch between source and destination files")
	}
	return nil
}

func (m MkdirOperation) Type() string {
	return "mkdir"
}

func (m MkdirOperation) Execute(mountDir string) error {
	return MkdirAll(mountDir, m.Path, m.FileMode)
}

func (m MkdirOperation) Verify(mountDir string) error {
	path := filepath.Join(mountDir, m.Path)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat directory for verification: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %s is not a directory", m.Path)
	}
	if info.Mode().Perm() != m.FileMode.Perm() {
		return fmt.Errorf("directory mode mismatch: got %v, want %v", info.Mode().Perm(), m.FileMode.Perm())
	}
	return nil
}

func (c ChmodOperation) Type() string {
	return "chmod"
}

func (c ChmodOperation) Execute(mountDir string) error {
	return Chmod(mountDir, c.Path, c.FileMode)
}

func (c ChmodOperation) Verify(mountDir string) error {
	path := filepath.Join(mountDir, c.Path)
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("failed to stat file for verification: %w", err)
	}
	if info.Mode().Perm() != c.FileMode.Perm() {
		return fmt.Errorf("file mode mismatch: got %v, want %v", info.Mode().Perm(), c.FileMode.Perm())
	}
	return nil
}

// Execute executes a batch of file operations
func Execute(params ExecuteParams) error {
	if params.MountDir == "" {
		return fmt.Errorf("mount directory is required")
	}

	for _, op := range params.Operations {
		if err := op.Execute(params.MountDir); err != nil {
			return fmt.Errorf("failed to execute operation %s: %w", op.Type(), err)
		}

		if params.VerifyWrite {
			if err := op.Verify(params.MountDir); err != nil {
				return fmt.Errorf("failed to verify operation %s: %w", op.Type(), err)
			}
		}
	}

	return nil
}
