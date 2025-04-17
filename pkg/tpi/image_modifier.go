package tpi

import (
	"io/fs"

	"github.com/davidroman0O/turingpi/pkg/tpi/imageops"
)

// imageModifierImpl implements ImageModifier
type imageModifierImpl struct {
	operations []imageops.FileOperation // Internal list of staged operations
}

// WriteFile stages a file write operation
func (m *imageModifierImpl) WriteFile(relativePath string, data []byte, perm fs.FileMode) {
	op := imageops.WriteOperation{
		RelativePath: relativePath,
		Data:         data,
		Perm:         perm,
	}
	m.operations = append(m.operations, op)
}

// CopyLocalFile stages a local file copy operation
func (m *imageModifierImpl) CopyLocalFile(localSourcePath, relativeDestPath string) {
	op := imageops.CopyLocalOperation{
		LocalSourcePath:  localSourcePath,
		RelativeDestPath: relativeDestPath,
	}
	m.operations = append(m.operations, op)
}

// MkdirAll stages a directory creation operation
func (m *imageModifierImpl) MkdirAll(relativePath string, perm fs.FileMode) {
	op := imageops.MkdirOperation{
		RelativePath: relativePath,
		Perm:         perm,
	}
	m.operations = append(m.operations, op)
}

// Chmod stages a permission change operation
func (m *imageModifierImpl) Chmod(relativePath string, perm fs.FileMode) {
	op := imageops.ChmodOperation{
		RelativePath: relativePath,
		Perm:         perm,
	}
	m.operations = append(m.operations, op)
}

// NewImageModifierImpl creates a new ImageModifier implementation
func NewImageModifierImpl() ImageModifier {
	return &imageModifierImpl{
		operations: make([]imageops.FileOperation, 0),
	}
}

// GetOperations returns the list of staged operations
func (m *imageModifierImpl) GetOperations() []imageops.FileOperation {
	return m.operations
}
