package ubuntu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCompressImageAction(t *testing.T) {
	// Create a new CompressImageAction
	action := NewCompressImageAction()

	// Test that the action is properly initialized
	assert.Equal(t, "CompressImage", action.Name())
	assert.Equal(t, "Compress the customized Ubuntu image", action.Description())
}
