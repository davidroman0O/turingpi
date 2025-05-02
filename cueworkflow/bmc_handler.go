package cueworkflow

import (
	"context"
	"fmt"
	"os/exec"

	"cuelang.org/go/cue"
)

// BMCActionHandler implements the ActionHandler interface for BMC-related actions
type BMCActionHandler struct {
	// clusterConfig holds the cluster configuration with BMC details
	config *ClusterConfig
}

// NewBMCActionHandler creates a new BMC action handler with the given cluster configuration
func NewBMCActionHandler(config *ClusterConfig) *BMCActionHandler {
	return &BMCActionHandler{
		config: config,
	}
}

// ActionType returns the type prefix this handler can process
func (h *BMCActionHandler) ActionType() string {
	return "bmc:"
}

// Execute executes the BMC action and returns a result or error
func (h *BMCActionHandler) Execute(ctx context.Context, action cue.Value) (interface{}, error) {
	// Extract the action type
	actionType, err := action.LookupPath(cue.ParsePath("type")).String()
	if err != nil {
		return nil, fmt.Errorf("failed to get action type: %w", err)
	}

	// Extract parameters
	paramsValue := action.LookupPath(cue.ParsePath("params"))
	if !paramsValue.Exists() {
		return nil, fmt.Errorf("action does not define parameters")
	}

	// Execute the appropriate BMC action based on the type
	switch actionType {
	case "bmc:power-on":
		return h.executePowerOn(ctx, paramsValue)
	case "bmc:power-off":
		return h.executePowerOff(ctx, paramsValue)
	case "bmc:reset":
		return h.executeReset(ctx, paramsValue)
	case "bmc:get-power-status":
		return h.executeGetPowerStatus(ctx, paramsValue)
	case "bmc:flash-node":
		return h.executeFlashNode(ctx, paramsValue)
	case "bmc:set-node-mode":
		return h.executeSetNodeMode(ctx, paramsValue)
	default:
		return nil, fmt.Errorf("unsupported BMC action type: %s", actionType)
	}
}

// extractNodeID extracts the node ID from parameters
func (h *BMCActionHandler) extractNodeID(params cue.Value) (int, error) {
	nodeIDValue := params.LookupPath(cue.ParsePath("nodeID"))
	if !nodeIDValue.Exists() {
		return 0, fmt.Errorf("parameters do not include 'nodeID'")
	}

	nodeID, err := nodeIDValue.Int64()
	if err != nil {
		return 0, fmt.Errorf("nodeID parameter is not an integer: %w", err)
	}

	if nodeID < 1 || nodeID > 4 {
		return 0, fmt.Errorf("nodeID must be between 1 and 4, got: %d", nodeID)
	}

	return int(nodeID), nil
}

// executePowerOn powers on a node
func (h *BMCActionHandler) executePowerOn(ctx context.Context, params cue.Value) (interface{}, error) {
	nodeID, err := h.extractNodeID(params)
	if err != nil {
		return nil, err
	}

	// In a real implementation, this would use a library to communicate with the BMC
	// Here we'll simulate it with a command execution
	cmd := exec.CommandContext(ctx, "echo", fmt.Sprintf("Powering on node %d", nodeID))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to power on node %d: %w", nodeID, err)
	}

	return fmt.Sprintf("Powered on node %d: %s", nodeID, string(output)), nil
}

// executePowerOff powers off a node
func (h *BMCActionHandler) executePowerOff(ctx context.Context, params cue.Value) (interface{}, error) {
	nodeID, err := h.extractNodeID(params)
	if err != nil {
		return nil, err
	}

	// In a real implementation, this would use a library to communicate with the BMC
	cmd := exec.CommandContext(ctx, "echo", fmt.Sprintf("Powering off node %d", nodeID))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to power off node %d: %w", nodeID, err)
	}

	return fmt.Sprintf("Powered off node %d: %s", nodeID, string(output)), nil
}

// executeReset resets a node
func (h *BMCActionHandler) executeReset(ctx context.Context, params cue.Value) (interface{}, error) {
	nodeID, err := h.extractNodeID(params)
	if err != nil {
		return nil, err
	}

	// In a real implementation, this would use a library to communicate with the BMC
	cmd := exec.CommandContext(ctx, "echo", fmt.Sprintf("Resetting node %d", nodeID))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to reset node %d: %w", nodeID, err)
	}

	return fmt.Sprintf("Reset node %d: %s", nodeID, string(output)), nil
}

// executeGetPowerStatus gets the power status of a node
func (h *BMCActionHandler) executeGetPowerStatus(ctx context.Context, params cue.Value) (interface{}, error) {
	nodeID, err := h.extractNodeID(params)
	if err != nil {
		return nil, err
	}

	// In a real implementation, this would use a library to communicate with the BMC
	cmd := exec.CommandContext(ctx, "echo", fmt.Sprintf("Getting power status for node %d", nodeID))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to get power status for node %d: %w", nodeID, err)
	}

	// Simulate a power status (on/off)
	status := "on"
	if nodeID%2 == 0 { // Just for simulation purposes
		status = "off"
	}

	return fmt.Sprintf("Power status for node %d: %s (cmd output: %s)", nodeID, status, string(output)), nil
}

// executeFlashNode flashes a node with an image
func (h *BMCActionHandler) executeFlashNode(ctx context.Context, params cue.Value) (interface{}, error) {
	nodeID, err := h.extractNodeID(params)
	if err != nil {
		return nil, err
	}

	// Extract the image path
	imagePathValue := params.LookupPath(cue.ParsePath("imagePath"))
	if !imagePathValue.Exists() {
		return nil, fmt.Errorf("parameters do not include 'imagePath'")
	}

	imagePath, err := imagePathValue.String()
	if err != nil {
		return nil, fmt.Errorf("imagePath parameter is not a string: %w", err)
	}

	// In a real implementation, this would use a library to communicate with the BMC
	cmd := exec.CommandContext(ctx, "echo", fmt.Sprintf("Flashing node %d with image %s", nodeID, imagePath))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to flash node %d with image %s: %w", nodeID, imagePath, err)
	}

	return fmt.Sprintf("Flashed node %d with image %s: %s", nodeID, imagePath, string(output)), nil
}

// executeSetNodeMode sets the node to a specific mode
func (h *BMCActionHandler) executeSetNodeMode(ctx context.Context, params cue.Value) (interface{}, error) {
	nodeID, err := h.extractNodeID(params)
	if err != nil {
		return nil, err
	}

	// Extract the mode
	modeValue := params.LookupPath(cue.ParsePath("mode"))
	if !modeValue.Exists() {
		return nil, fmt.Errorf("parameters do not include 'mode'")
	}

	mode, err := modeValue.String()
	if err != nil {
		return nil, fmt.Errorf("mode parameter is not a string: %w", err)
	}

	// Validate the mode
	if mode != "normal" && mode != "msd" {
		return nil, fmt.Errorf("invalid mode: %s (supported: 'normal', 'msd')", mode)
	}

	// In a real implementation, this would use a library to communicate with the BMC
	cmd := exec.CommandContext(ctx, "echo", fmt.Sprintf("Setting node %d to mode %s", nodeID, mode))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to set node %d to mode %s: %w", nodeID, mode, err)
	}

	return fmt.Sprintf("Set node %d to mode %s: %s", nodeID, mode, string(output)), nil
}
