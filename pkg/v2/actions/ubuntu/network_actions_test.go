package ubuntu

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfigureNetworkAction(t *testing.T) {
	// Create a new ConfigureNetworkAction
	nodeID := 3
	action := NewConfigureNetworkAction(nodeID)

	// Test that the action is properly initialized
	assert.Equal(t, "ConfigureNetwork", action.Name())
	assert.Equal(t, "Configure network settings for node 3", action.Description())
	assert.Equal(t, nodeID, action.nodeID)
}
