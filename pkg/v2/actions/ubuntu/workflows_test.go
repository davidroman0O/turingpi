package ubuntu

import (
	"fmt"
	"testing"

	"github.com/davidroman0O/gostate"
	"github.com/davidroman0O/gostate/store"
	"github.com/stretchr/testify/assert"
)

// MockAction is a simple mock action for testing
type MockAction struct {
	gostate.BaseAction
	executed bool
}

func NewMockAction(name string) *MockAction {
	return &MockAction{
		BaseAction: gostate.NewBaseAction(name, "Mock action for testing"),
		executed:   false,
	}
}

func (a *MockAction) Execute(ctx *gostate.ActionContext) error {
	a.executed = true
	return nil
}

func TestCreateRK1DeploymentWorkflow(t *testing.T) {
	// Create workflow options with custom settings
	nodeID := 3
	osVersion := "22.04"
	password := "securepassword"

	opts := DefaultWorkflowOptions(nodeID, osVersion)
	opts.SetNodePassword(password)

	// Add custom network config
	networkConfig := NetworkConfig{
		Hostname:   "node3",
		IPCIDR:     "192.168.1.3/24",
		Gateway:    "192.168.1.1",
		DNSServers: []string{"8.8.8.8", "1.1.1.1"},
	}
	opts.SetNetworkConfig(networkConfig)

	// Add custom actions at hook points
	beforeUnmountAction := NewMockAction("BeforeUnmountAction")
	afterPostInstallAction := NewMockAction("AfterPostInstallAction")

	opts.AddActionBeforeUnmount(beforeUnmountAction)
	opts.AddActionAfterPostInstall(afterPostInstallAction)

	// Add custom store value
	opts.AddStoreValue("customKey", "customValue")

	// Create the workflow
	workflow, err := CreateRK1DeploymentWorkflow(opts)

	// Verify workflow creation
	assert.NoError(t, err)
	assert.NotNil(t, workflow)

	// Check workflow name and basic properties
	assert.Contains(t, workflow.Name, fmt.Sprintf("Ubuntu %s Deployment for RK1 Node %d", osVersion, nodeID))

	// Check workflow store values
	nodeIDValue, err := store.Get[int](workflow.Store, "nodeID")
	assert.NoError(t, err)
	assert.Equal(t, nodeID, nodeIDValue)

	passwordValue, err := store.Get[string](workflow.Store, "nodePassword")
	assert.NoError(t, err)
	assert.Equal(t, password, passwordValue)

	customValue, err := store.Get[string](workflow.Store, "customKey")
	assert.NoError(t, err)
	assert.Equal(t, "customValue", customValue)

	// Check network config
	networkConfigValue, err := store.Get[NetworkConfig](workflow.Store, "networkConfig")
	assert.NoError(t, err)
	assert.Equal(t, networkConfig, networkConfigValue)

	// Verify stages
	assert.Equal(t, 3, len(workflow.Stages))

	// Check stage IDs
	stageNames := []string{}
	for _, stage := range workflow.Stages {
		stageNames = append(stageNames, stage.ID)
	}
	assert.Contains(t, stageNames, StageIDImagePreparation)
	assert.Contains(t, stageNames, StageIDOSInstallation)
	assert.Contains(t, stageNames, StageIDPostInstallation)

	// TODO: When actually implementing the actions and execution logic,
	// we can add more detailed tests to verify that the custom actions
	// are properly added at the hook points and executed in the right order.
}
