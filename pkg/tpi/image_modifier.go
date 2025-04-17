package tpi

import (
	"log"
	"os"

	"github.com/davidroman0O/turingpi/pkg/tpi/internal/imageops" // Internal helpers
)

// --- ImageModifier Implementation ---

// imageModifierImpl implements the tpi.ImageModifier interface.
// It collects file operations to be executed later.
type imageModifierImpl struct {
	operations []imageops.FileOperation // Internal list of staged operations
}

// WriteFile stages an operation to write data to a file.
func (m *imageModifierImpl) WriteFile(relativePath string, data []byte, perm os.FileMode) {
	op := imageops.WriteOperation{
		RelativePath: relativePath,
		Data:         data,
		Perm:         perm,
	}
	m.operations = append(m.operations, op)
	log.Printf("[ImageModifier] Staged: Write %d bytes to %s (Mode: %o)", len(data), relativePath, perm)
}

// CopyLocalFile stages an operation to copy a local file into the image.
func (m *imageModifierImpl) CopyLocalFile(localSourcePath string, relativeDestPath string) {
	// TODO: Add permission setting? Currently relies on default cp perms + subsequent Chmod.
	op := imageops.CopyLocalOperation{
		LocalSourcePath:  localSourcePath,
		RelativeDestPath: relativeDestPath,
	}
	m.operations = append(m.operations, op)
	log.Printf("[ImageModifier] Staged: Copy local %s to %s", localSourcePath, relativeDestPath)
}

// MkdirAll stages an operation to create a directory.
func (m *imageModifierImpl) MkdirAll(relativePath string, perm os.FileMode) {
	op := imageops.MkdirOperation{
		RelativePath: relativePath,
		Perm:         perm,
	}
	m.operations = append(m.operations, op)
	log.Printf("[ImageModifier] Staged: Mkdir %s (Mode: %o)", relativePath, perm)
}

// Chmod stages an operation to change file permissions.
func (m *imageModifierImpl) Chmod(relativePath string, perm os.FileMode) {
	op := imageops.ChmodOperation{
		RelativePath: relativePath,
		Perm:         perm,
	}
	m.operations = append(m.operations, op)
	log.Printf("[ImageModifier] Staged: Chmod %s (Mode: %o)", relativePath, perm)
}

// NewImageModifierImpl creates a new instance of the ImageModifier implementation.
func NewImageModifierImpl() *imageModifierImpl {
	return &imageModifierImpl{
		operations: make([]imageops.FileOperation, 0),
	}
}

// GetOperations returns the staged operations.
// This needs to be called by the builder Run method.
func (m *imageModifierImpl) GetOperations() []imageops.FileOperation {
	return m.operations
}
