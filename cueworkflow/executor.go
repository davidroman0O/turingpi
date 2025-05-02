package cueworkflow

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
	"github.com/davidroman0O/turingpi/bmc"
	"github.com/davidroman0O/turingpi/tools"
)

// LoadConfig loads a cluster configuration from a CUE file
func LoadConfig(ctx context.Context, filePath string) (*cue.Value, error) {
	cueCtx := cuecontext.New()

	// Convert to absolute path if it's not already
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path for %s: %w", filePath, err)
	}

	// Set up the load configuration
	loadConfig := &load.Config{
		Dir: filepath.Dir(absPath),
	}

	// Build the CUE instance - use just the filename without path in the instance
	fileName := filepath.Base(absPath)
	instances := load.Instances([]string{fileName}, loadConfig)
	if len(instances) == 0 {
		return nil, fmt.Errorf("no CUE instances found in %s", absPath)
	}

	if instances[0].Err != nil {
		return nil, fmt.Errorf("error loading CUE file %s: %w", absPath, instances[0].Err)
	}

	// Build and evaluate the CUE value
	value := cueCtx.BuildInstance(instances[0])
	if value.Err() != nil {
		return nil, fmt.Errorf("error building CUE instance: %w", value.Err())
	}

	// Look for the cluster configuration
	clusterValue := value.LookupPath(cue.ParsePath("cluster"))
	if !clusterValue.Exists() {
		return nil, fmt.Errorf("no cluster configuration found in %s", absPath)
	}

	// Validate the configuration
	if err := clusterValue.Validate(); err != nil {
		return nil, fmt.Errorf("cluster configuration validation failed: %w", err)
	}

	return &clusterValue, nil
}

// ExecuteWorkflow executes a workflow using the given configuration
func ExecuteWorkflow(ctx context.Context, workflow *cue.Value, config *cue.Value) error {
	// Extract workflow information
	title, _ := workflow.LookupPath(cue.ParsePath("title")).String()
	description, _ := workflow.LookupPath(cue.ParsePath("description")).String()

	log.Printf("Starting workflow: %s - %s", title, description)

	// Process each stage in the workflow
	stagesValue := workflow.LookupPath(cue.ParsePath("stages"))
	stagesIter, err := stagesValue.List()
	if err != nil {
		return fmt.Errorf("error iterating workflow stages: %w", err)
	}

	stageIndex := 1
	for stagesIter.Next() {
		stageValue := stagesIter.Value()

		// Extract stage information
		stageName, _ := stageValue.LookupPath(cue.ParsePath("name")).String()
		stageTitle, _ := stageValue.LookupPath(cue.ParsePath("title")).String()
		// We're getting the description but not using it yet - that's ok
		_, _ = stageValue.LookupPath(cue.ParsePath("description")).String()

		log.Printf("Executing stage %d: %s - %s", stageIndex, stageName, stageTitle)

		// Process each action in the stage
		actionsValue := stageValue.LookupPath(cue.ParsePath("actions"))
		actionsIter, err := actionsValue.List()
		if err != nil {
			return fmt.Errorf("error iterating stage actions: %w", err)
		}

		actionIndex := 1
		for actionsIter.Next() {
			actionValue := actionsIter.Value()

			// Extract action information
			actionType, err := actionValue.LookupPath(cue.ParsePath("type")).String()
			if err != nil {
				return fmt.Errorf("error getting action type: %w", err)
			}

			log.Printf("Executing action %d: %s", actionIndex, actionType)

			// Process the action based on its type
			startTime := time.Now()
			result, err := executeAction(ctx, actionType, &actionValue, config)
			duration := time.Since(startTime)

			if err != nil {
				return fmt.Errorf("error executing action %d (%s): %w", actionIndex, actionType, err)
			}

			log.Printf("Action %d completed in %v with result: %s", actionIndex, duration, result)
			actionIndex++
		}

		stageIndex++
	}

	log.Printf("Workflow completed successfully: %s", title)
	return nil
}

// getBMCTool creates a new BMCTool instance from the config
func getBMCTool(ctx context.Context, config *cue.Value) (tools.BMCTool, error) {
	// Extract BMC configuration from the config
	bmcIP, err := config.LookupPath(cue.ParsePath("bmc.ip")).String()
	if err != nil {
		return nil, fmt.Errorf("error getting BMC IP: %w", err)
	}

	bmcUser, err := config.LookupPath(cue.ParsePath("bmc.user")).String()
	if err != nil {
		return nil, fmt.Errorf("error getting BMC user: %w", err)
	}

	bmcPassword, err := config.LookupPath(cue.ParsePath("bmc.password")).String()
	if err != nil {
		return nil, fmt.Errorf("error getting BMC password: %w", err)
	}

	// Create a new BMC executor
	bmcExecutor := bmc.NewSSHExecutor(bmcIP, 22, bmcUser, bmcPassword)

	// Create a new BMC instance
	bmcInstance := bmc.New(bmcExecutor)

	// Wrap it in a BMCToolAdapter
	bmcTool := tools.NewBMCToolAdapter(bmcInstance)

	return bmcTool, nil
}

// executeAction executes a single action of the given type
func executeAction(ctx context.Context, actionType string, actionValue, config *cue.Value) (string, error) {
	// Parse out action parameters
	paramsValue := actionValue.LookupPath(cue.ParsePath("params"))

	// Get or create BMC tool if needed for BMC actions
	var bmcTool tools.BMCTool
	var err error
	if strings.HasPrefix(actionType, "bmc:") {
		bmcTool, err = getBMCTool(ctx, config)
		if err != nil {
			return "", fmt.Errorf("error initializing BMC tool: %w", err)
		}
	}

	// Handle different action types
	switch actionType {
	case "common:wait":
		// Extract wait duration
		seconds, err := paramsValue.LookupPath(cue.ParsePath("seconds")).Int64()
		if err != nil {
			return "", fmt.Errorf("invalid wait duration: %w", err)
		}

		// Perform the wait
		time.Sleep(time.Duration(seconds) * time.Second)
		return fmt.Sprintf("waited for %d seconds", seconds), nil

	case "bmc:get-power-status":
		// Extract node ID
		nodeIDValue, err := paramsValue.LookupPath(cue.ParsePath("nodeID")).Int64()
		if err != nil {
			return "", fmt.Errorf("invalid node ID: %w", err)
		}
		nodeID := int(nodeIDValue)

		// Get BMC connection details for logging
		bmcIP, _ := config.LookupPath(cue.ParsePath("bmc.ip")).String()
		bmcUser, _ := config.LookupPath(cue.ParsePath("bmc.user")).String()
		log.Printf("Querying power status for node %d (BMC: %s, user: %s)", nodeID, bmcIP, bmcUser)

		// Use BMC tool to get actual power status
		powerStatus, err := bmcTool.GetPowerStatus(ctx, nodeID)
		if err != nil {
			return "", fmt.Errorf("error getting power status for node %d: %w", nodeID, err)
		}

		statusStr := "unknown"
		if powerStatus != nil {
			// Check the State field from the PowerStatus struct
			if powerStatus.State == bmc.PowerStateOn {
				statusStr = "on"
			} else if powerStatus.State == bmc.PowerStateOff {
				statusStr = "off"
			}
		}

		return fmt.Sprintf("Power status for node %d: %s", nodeID, statusStr), nil

	case "bmc:power-on":
		// Extract node ID
		nodeIDValue, err := paramsValue.LookupPath(cue.ParsePath("nodeID")).Int64()
		if err != nil {
			return "", fmt.Errorf("invalid node ID: %w", err)
		}
		nodeID := int(nodeIDValue)

		// First check current power status
		powerStatus, err := bmcTool.GetPowerStatus(ctx, nodeID)
		if err != nil {
			return "", fmt.Errorf("failed to get power status before powering on: %w", err)
		}

		// Check if already powered on
		if powerStatus != nil && powerStatus.State == bmc.PowerStateOn {
			return fmt.Sprintf("Node %d is already powered on, skipping power-on command", nodeID), nil
		}

		// Get BMC connection details for logging
		bmcIP, _ := config.LookupPath(cue.ParsePath("bmc.ip")).String()
		log.Printf("Powering on node %d via BMC at %s", nodeID, bmcIP)

		// Use BMC tool to power on the node
		if err := bmcTool.PowerOn(ctx, nodeID); err != nil {
			return "", fmt.Errorf("error powering on node %d: %w", nodeID, err)
		}

		// Verify the power status after operation
		time.Sleep(3 * time.Second) // Give it a moment to take effect
		verifyStatus, err := bmcTool.GetPowerStatus(ctx, nodeID)
		if err != nil {
			return "", fmt.Errorf("failed to verify power status after powering on: %w", err)
		}

		if verifyStatus != nil && verifyStatus.State == bmc.PowerStateOn {
			return fmt.Sprintf("Powered on node %d successfully", nodeID), nil
		} else {
			return "", fmt.Errorf("failed to power on node %d: status check shows it's still off", nodeID)
		}

	case "bmc:power-off":
		// Extract node ID
		nodeIDValue, err := paramsValue.LookupPath(cue.ParsePath("nodeID")).Int64()
		if err != nil {
			return "", fmt.Errorf("invalid node ID: %w", err)
		}
		nodeID := int(nodeIDValue)

		// First check current power status
		powerStatus, err := bmcTool.GetPowerStatus(ctx, nodeID)
		if err != nil {
			return "", fmt.Errorf("failed to get power status before powering off: %w", err)
		}

		// Check if already powered off
		if powerStatus != nil && powerStatus.State == bmc.PowerStateOff {
			return fmt.Sprintf("Node %d is already powered off, skipping power-off command", nodeID), nil
		}

		// Get BMC connection details for logging
		bmcIP, _ := config.LookupPath(cue.ParsePath("bmc.ip")).String()
		log.Printf("Powering off node %d via BMC at %s", nodeID, bmcIP)

		// Use BMC tool to power off the node
		if err := bmcTool.PowerOff(ctx, nodeID); err != nil {
			return "", fmt.Errorf("error powering off node %d: %w", nodeID, err)
		}

		// Verify the power status after operation
		time.Sleep(3 * time.Second) // Give it a moment to take effect
		verifyStatus, err := bmcTool.GetPowerStatus(ctx, nodeID)
		if err != nil {
			return "", fmt.Errorf("failed to verify power status after powering off: %w", err)
		}

		if verifyStatus != nil && verifyStatus.State == bmc.PowerStateOff {
			return fmt.Sprintf("Powered off node %d successfully", nodeID), nil
		} else {
			return "", fmt.Errorf("failed to power off node %d: status check shows it's still on", nodeID)
		}

	case "bmc:reset":
		// Extract node ID
		nodeIDValue, err := paramsValue.LookupPath(cue.ParsePath("nodeID")).Int64()
		if err != nil {
			return "", fmt.Errorf("invalid node ID: %w", err)
		}
		nodeID := int(nodeIDValue)

		// Get BMC connection details for logging
		bmcIP, _ := config.LookupPath(cue.ParsePath("bmc.ip")).String()
		log.Printf("Resetting node %d via BMC at %s", nodeID, bmcIP)

		// Use BMC tool to reset the node
		if err := bmcTool.Reset(ctx, nodeID); err != nil {
			return "", fmt.Errorf("error resetting node %d: %w", nodeID, err)
		}

		// Wait a bit for the reset to complete
		time.Sleep(5 * time.Second)

		// Verify the power status after reset (should be on)
		verifyStatus, err := bmcTool.GetPowerStatus(ctx, nodeID)
		if err != nil {
			return "", fmt.Errorf("failed to verify power status after reset: %w", err)
		}

		if verifyStatus != nil && verifyStatus.State == bmc.PowerStateOn {
			return fmt.Sprintf("Successfully reset node %d", nodeID), nil
		} else {
			return "", fmt.Errorf("reset may have failed for node %d: status check shows it's off", nodeID)
		}

	case "bmc:set-node-mode":
		// Extract node ID
		nodeIDValue, err := paramsValue.LookupPath(cue.ParsePath("nodeID")).Int64()
		if err != nil {
			return "", fmt.Errorf("invalid node ID: %w", err)
		}
		nodeID := int(nodeIDValue)

		// Extract mode
		modeStr, err := paramsValue.LookupPath(cue.ParsePath("mode")).String()
		if err != nil {
			return "", fmt.Errorf("invalid mode: %w", err)
		}

		// Convert mode string to NodeMode
		var mode bmc.NodeMode
		switch strings.ToLower(modeStr) {
		case "normal":
			mode = bmc.NodeModeNormal
		case "msd":
			mode = bmc.NodeModeMSD
		default:
			return "", fmt.Errorf("invalid mode: %s (supported: normal, msd)", modeStr)
		}

		// Get BMC connection details for logging
		bmcIP, _ := config.LookupPath(cue.ParsePath("bmc.ip")).String()
		log.Printf("Setting node %d to mode %s via BMC at %s", nodeID, modeStr, bmcIP)

		// Use BMC tool to set the node mode
		if err := bmcTool.SetNodeMode(ctx, nodeID, mode); err != nil {
			return "", fmt.Errorf("error setting node %d mode to %s: %w", nodeID, modeStr, err)
		}

		return fmt.Sprintf("Set node %d to mode %s successfully", nodeID, modeStr), nil

	case "bmc:flash-node":
		// Extract node ID
		nodeIDValue, err := paramsValue.LookupPath(cue.ParsePath("nodeID")).Int64()
		if err != nil {
			return "", fmt.Errorf("invalid node ID: %w", err)
		}
		nodeID := int(nodeIDValue)

		// Extract image path
		imagePath, err := paramsValue.LookupPath(cue.ParsePath("imagePath")).String()
		if err != nil {
			return "", fmt.Errorf("invalid image path: %w", err)
		}

		// Get BMC connection details for logging
		bmcIP, _ := config.LookupPath(cue.ParsePath("bmc.ip")).String()
		log.Printf("Flashing node %d with image %s via BMC at %s", nodeID, imagePath, bmcIP)

		// Use BMC tool to flash the node
		if err := bmcTool.FlashNode(ctx, nodeID, imagePath); err != nil {
			return "", fmt.Errorf("error flashing node %d with image %s: %w", nodeID, imagePath, err)
		}

		return fmt.Sprintf("Flashed node %d with image %s successfully", nodeID, imagePath), nil

	default:
		return "", fmt.Errorf("unsupported action type: %s", actionType)
	}
}
