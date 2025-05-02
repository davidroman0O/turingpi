package cueworkflow

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// LoadClusterConfig loads a cluster configuration from a CUE file
func LoadClusterConfig(ctx context.Context, filePath string) (*ClusterConfig, error) {
	cueCtx := cuecontext.New()

	// Convert to absolute path if it's not already
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute path for %s: %w", filePath, err)
	}

	// Setup the load configuration
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

	// Look for the cluster field
	clusterValue := value.LookupPath(cue.ParsePath("cluster"))
	if !clusterValue.Exists() {
		return nil, fmt.Errorf("no 'cluster' field found in %s", absPath)
	}

	// Decode into our Go structure
	var clusterConfig ClusterConfig
	if err := clusterValue.Decode(&clusterConfig); err != nil {
		return nil, fmt.Errorf("error decoding cluster config: %w", err)
	}

	return &clusterConfig, nil
}

// LoadWorkflow loads a workflow definition from a CUE file
func LoadWorkflow(ctx context.Context, filePath, workflowPath string, inputParams map[string]interface{}) (*cue.Value, error) {
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

	// Look for the workflow at the specified path
	workflowValue := value
	if workflowPath != "" && workflowPath != "." {
		workflowValue = value.LookupPath(cue.ParsePath(workflowPath))
		if !workflowValue.Exists() {
			return nil, fmt.Errorf("no workflow found at path '%s' in %s", workflowPath, absPath)
		}
	}

	// If we have input parameters, apply them
	if len(inputParams) > 0 {
		// First, build a CUE value with all the parameters
		paramStrings := make([]string, 0, len(inputParams))
		for k, v := range inputParams {
			var valueStr string
			switch val := v.(type) {
			case string:
				valueStr = fmt.Sprintf("%q", val)
			case int:
				valueStr = fmt.Sprintf("%d", val)
			case float64:
				valueStr = fmt.Sprintf("%g", val)
			case bool:
				valueStr = fmt.Sprintf("%t", val)
			default:
				return nil, fmt.Errorf("unsupported parameter type for %s: %T", k, v)
			}
			paramStrings = append(paramStrings, fmt.Sprintf("%s: %s", k, valueStr))
		}

		// Combine all parameters into a single CUE object
		paramsObj := fmt.Sprintf("params: {%s}", strings.Join(paramStrings, ", "))
		paramsValue := cueCtx.CompileString(paramsObj)
		if paramsValue.Err() != nil {
			return nil, fmt.Errorf("error compiling parameters: %w", paramsValue.Err())
		}

		// Fill the workflow with the parameters
		workflowValue = workflowValue.Fill(paramsValue)
		if workflowValue.Err() != nil {
			return nil, fmt.Errorf("error filling workflow with parameters: %w", workflowValue.Err())
		}
	}

	// Check if all required parameters are provided
	paramsValue := workflowValue.LookupPath(cue.ParsePath("params"))
	if paramsValue.Exists() {
		if err := paramsValue.Validate(cue.Concrete(true)); err != nil {
			return nil, fmt.Errorf("missing or invalid parameters: %w", err)
		}
	}

	// Validate the entire workflow
	if err := workflowValue.Validate(); err != nil {
		return nil, fmt.Errorf("workflow validation failed: %w", err)
	}

	return &workflowValue, nil
}

// findProjectRoot finds the project root by looking for cue.mod directory
func findProjectRoot(path string) (string, error) {
	// Start from the current directory and move up until we find cue.mod
	dir := filepath.Dir(path)
	for {
		cueModPath := filepath.Join(dir, "cue.mod")
		if _, err := os.Stat(cueModPath); err == nil {
			// Found the cue.mod directory
			return dir, nil
		}

		// If we've reached the root directory, stop
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find cue.mod directory in any parent of %s", path)
		}
		dir = parent
	}
}

// SaveCUETemplate saves a CUE template for a workflow or configuration to the specified file
func SaveCUETemplate(ctx context.Context, templateType, filePath string) error {
	var content string

	switch templateType {
	case "workflow":
		content = `package workflow

// Define a workflow for resetting a node
reset: {
	// Metadata about this workflow
	title:       "node-reset"
	description: "Resets a specific node"
	
	// Parameters required by this workflow
	params: {
		// Node ID must be between 1 and 4
		nodeID: int & >=1 & <=4
	}
	
	// Use the nodeID parameter throughout the workflow
	let targetNode = params.nodeID
	
	// Stages of the workflow
	stages: [{
		name:        "reset-sequence"
		title:       "Sequence of operations to reset the node"
		description: "Gets the power status, powers off, waits, powers on, then gets status again"
		
		// Actions to execute in this stage
		actions: [
			// First check the current power status
			{
				type: "bmc:get-power-status"
				params: {
					nodeID: targetNode
				}
			},
			// Power off the node
			{
				type: "bmc:power-off"
				params: {
					nodeID: targetNode
				}
			},
			// Wait for 5 seconds
			{
				type: "common:wait"
				params: {
					seconds: 5
				}
			},
			// Power on the node
			{
				type: "bmc:power-on"
				params: {
					nodeID: targetNode
				}
			},
			// Wait for 10 seconds to allow the node to boot
			{
				type: "common:wait"
				params: {
					seconds: 10
				}
			},
			// Check the power status again
			{
				type: "bmc:get-power-status"
				params: {
					nodeID: targetNode
				}
			},
		]
	}]
}`
	case "cluster":
		content = `package config

// Define the cluster configuration
cluster: {
	// BMC connection details
	bmc: {
		ip:       "127.0.0.1"
		user:     "admin"
		password: "admin"
	}
	
	// Optional cache paths
	cache?: {
		local?: string
		temp?:  string
	}
}`
	default:
		return fmt.Errorf("unknown template type: %s", templateType)
	}

	return os.WriteFile(filePath, []byte(content), 0644)
}
