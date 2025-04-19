package imageops

import (
	"log"
	"os"
	"sync"

	"github.com/davidroman0O/turingpi/pkg/tpi/imageops/ops"
)

// ImageModifier provides an interface for modifying an OS image.
type ImageModifier interface {
	// WriteFile stages an operation to write data to a file.
	WriteFile(relativePath string, data []byte, perm os.FileMode)

	// CopyLocalFile stages an operation to copy a local file into the image.
	CopyLocalFile(localSourcePath string, relativeDestPath string)

	// MkdirAll stages an operation to create a directory.
	MkdirAll(relativePath string, perm os.FileMode)

	// Chmod stages an operation to change file permissions.
	Chmod(relativePath string, perm os.FileMode)

	// Value retrieves a value from the context map.
	Value(key string) interface{}

	// SetContextValue sets a value in the context map.
	SetContextValue(key string, value interface{})

	// GetOperations returns the staged operations.
	GetOperations() []ops.Operation
}

// imageModifierImpl implements the ImageModifier interface.
type imageModifierImpl struct {
	operations []ops.Operation        // Internal list of staged operations
	contextMap map[string]interface{} // Dynamic data storage
	mu         sync.RWMutex           // Protects contextMap
}

// NewImageModifier creates a new instance of the ImageModifier implementation.
func NewImageModifier() ImageModifier {
	return &imageModifierImpl{
		operations: make([]ops.Operation, 0),
		contextMap: make(map[string]interface{}),
	}
}

// WriteFile stages an operation to write data to a file.
func (m *imageModifierImpl) WriteFile(relativePath string, data []byte, perm os.FileMode) {
	op := ops.WriteOperation{
		Path:     relativePath,
		Content:  data,
		FileMode: perm,
	}
	m.operations = append(m.operations, op)
	log.Printf("[ImageModifier] Staged: Write %d bytes to %s (Mode: %o)", len(data), relativePath, perm)
}

// CopyLocalFile stages an operation to copy a local file into the image.
func (m *imageModifierImpl) CopyLocalFile(localSourcePath string, relativeDestPath string) {
	op := ops.CopyOperation{
		SourcePath: localSourcePath,
		DestPath:   relativeDestPath,
	}
	m.operations = append(m.operations, op)
	log.Printf("[ImageModifier] Staged: Copy local %s to %s", localSourcePath, relativeDestPath)
}

// MkdirAll stages an operation to create a directory.
func (m *imageModifierImpl) MkdirAll(relativePath string, perm os.FileMode) {
	op := ops.MkdirOperation{
		Path:     relativePath,
		FileMode: perm,
	}
	m.operations = append(m.operations, op)
	log.Printf("[ImageModifier] Staged: Mkdir %s (Mode: %o)", relativePath, perm)
}

// Chmod stages an operation to change file permissions.
func (m *imageModifierImpl) Chmod(relativePath string, perm os.FileMode) {
	op := ops.ChmodOperation{
		Path:     relativePath,
		FileMode: perm,
	}
	m.operations = append(m.operations, op)
	log.Printf("[ImageModifier] Staged: Chmod %s (Mode: %o)", relativePath, perm)
}

// Value retrieves a value from the context map.
func (m *imageModifierImpl) Value(key string) interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.contextMap[key]
}

// SetContextValue sets a value in the context map.
func (m *imageModifierImpl) SetContextValue(key string, value interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.contextMap[key] = value
}

// GetOperations returns the staged operations.
func (m *imageModifierImpl) GetOperations() []ops.Operation {
	return m.operations
}
