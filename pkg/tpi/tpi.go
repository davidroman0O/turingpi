package tpi

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/bmc" // Import from new location
	"github.com/davidroman0O/turingpi/pkg/tpi/state"
)

// TuringPiExecutor holds the processed configuration and runtime state
// for executing Turing Pi workflows.
// It implements the Cluster interface to provide a cleaner API.
type TuringPiExecutor struct {
	config        TPIConfig     // The validated and defaulted configuration
	stateFilePath string        // Absolute path to the state file
	stateManager  state.Manager // Direct use of the state package manager
	// bmcClient     *bmc.Client   // Client for interacting with the BMC (Removed as bmc pkg is functional)
	// TODO: Add credential manager etc.
}

// NewTuringPi validates the provided configuration, sets defaults,
// initializes necessary components, and returns an executor instance.
func NewTuringPi(config TPIConfig) (*TuringPiExecutor, error) {
	log.Println("Initializing Turing Pi Executor...")

	// --- Validate Required Config --- //
	if config.IP == "" {
		return nil, fmt.Errorf("TPIConfig validation failed: missing required field 'IP' (BMC IP Address)")
	}
	// Basic validation for BMC credentials (can be enhanced)
	if config.BMCUser == "" {
		// Default to root if not provided?
		log.Println("BMCUser not specified, defaulting to 'root'")
		config.BMCUser = "root"
	}
	// Password might be empty if using other auth methods in the future, but required for now.
	if config.BMCPassword == "" {
		log.Println("Warning: BMCPassword is empty. BMC operations will likely fail.")
		// return nil, fmt.Errorf("TPIConfig validation failed: missing required field 'BMCPassword'")
	}

	configuredNodes := 0
	if config.Node1 != nil {
		configuredNodes++
		if err := validateNodeConfig(*config.Node1, Node1); err != nil {
			return nil, err
		}
	}
	if config.Node2 != nil {
		configuredNodes++
		if err := validateNodeConfig(*config.Node2, Node2); err != nil {
			return nil, err
		}
	}
	if config.Node3 != nil {
		configuredNodes++
		if err := validateNodeConfig(*config.Node3, Node3); err != nil {
			return nil, err
		}
	}
	if config.Node4 != nil {
		configuredNodes++
		if err := validateNodeConfig(*config.Node4, Node4); err != nil {
			return nil, err
		}
	}

	if configuredNodes == 0 {
		return nil, fmt.Errorf("TPIConfig validation failed: at least one Node (Node1-Node4) must be configured")
	}

	// --- Apply Defaults --- //

	// Default Cache Directory
	if config.CacheDir == "" {
		userCacheDir, err := os.UserCacheDir()
		if err != nil {
			// Fallback if user cache dir isn't found (e.g., minimal environment)
			config.CacheDir = ".tpi_cache" // Use local directory as fallback
			log.Printf("Warning: Could not find user cache directory (%v), using default relative path: %s", err, config.CacheDir)
		} else {
			config.CacheDir = filepath.Join(userCacheDir, "tpi")
		}
	}

	// Ensure Cache Directory exists (use absolute path for clarity)
	absCacheDir, err := filepath.Abs(config.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("failed to get absolute path for CacheDir '%s': %w", config.CacheDir, err)
	}
	config.CacheDir = absCacheDir // Store the absolute path
	if err := os.MkdirAll(config.CacheDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create cache directory '%s': %w", config.CacheDir, err)
	}
	log.Printf("Using cache directory: %s", config.CacheDir)

	// Default State File Name (relative to CacheDir)
	if config.StateFileName == "" {
		config.StateFileName = "tpi_state.json"
	}
	// Construct full state file path
	stateFilePath := filepath.Join(config.CacheDir, config.StateFileName)
	log.Printf("Using state file: %s", stateFilePath)

	// --- Initialize Internal Components --- //

	// Initialize State Manager - directly use the state package
	stateMgr, err := state.NewFileStateManager(stateFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize state manager: %w", err)
	}

	// Initialize BMC client
	// NOTE: We are not actually creating a bmc.Client struct here, as the bmc package
	// currently uses functions that take SSHConfig directly. We will store the config
	// in the executor for now and construct SSHConfig on the fly when needed.
	// If bmc package evolves to have a client struct, update this.
	log.Printf("BMC configured: Host=%s, User=%s", config.IP, config.BMCUser)

	// TODO: Initialize Credential Manager

	// --- Create Executor --- //
	executor := &TuringPiExecutor{
		config:        config, // Store the processed config
		stateFilePath: stateFilePath,
		stateManager:  stateMgr,
		// bmcClient: nil,
		// credentialManager: credManager,
	}

	log.Println("Turing Pi Executor initialized successfully.")
	return executor, nil
}

// validateNodeConfig checks if a specific node's configuration is valid.
func validateNodeConfig(nodeCfg NodeConfig, nodeID NodeID) error {
	if nodeCfg.IP == "" {
		return fmt.Errorf("TPIConfig validation failed: Node%d configuration is missing required field 'IP'", nodeID)
	}
	if nodeCfg.Board == "" {
		return fmt.Errorf("TPIConfig validation failed: Node%d configuration is missing required field 'Board'", nodeID)
	}
	// Add more validation as needed (e.g., IP format, BoardType validity)
	return nil
}

// --- Run Method --- //

// Run prepares the execution environment for the defined workflow template.
// It accepts a function `workflowTemplate` that defines the sequence of operations
// (image building, OS installation, post-installation) for a *single* node.
// It returns a new function that, when called with a specific `NodeID`, executes
// the defined workflow template for that node.
func (tpi *TuringPiExecutor) Run(workflowTemplate func(ctx Context, cluster Cluster, node Node) error) func(execCtx context.Context, nodeID NodeID) error {
	log.Println("Preparing node execution function...")

	// Return the function that will be called for each node
	return func(execCtx context.Context, nodeID NodeID) error {
		log.Printf("--- Initiating workflow execution for Node %d ---", nodeID)

		// 1. Get Node Configuration
		nodeCfg := tpi.GetNodeConfig(nodeID)
		if nodeCfg == nil {
			return fmt.Errorf("configuration for Node %d not found in TPIConfig", nodeID)
		}

		// 2. Populate Node Details with default network values
		// Extract the IP without CIDR suffix if it exists
		ipAddress := nodeCfg.IP
		if idx := strings.Index(ipAddress, "/"); idx != -1 {
			ipAddress = ipAddress[:idx]
		}

		// Create a default hostname if not specified
		hostname := fmt.Sprintf("tpi-node%d", nodeID)

		// Determine default gateway based on IP (using same subnet with .1)
		// This is a simple approach - in production, we might need more sophisticated logic
		ipParts := strings.Split(ipAddress, ".")
		gateway := "192.168.1.1" // Default fallback
		if len(ipParts) == 4 {
			// Construct gateway as first 3 octets + .1
			gateway = fmt.Sprintf("%s.%s.%s.1", ipParts[0], ipParts[1], ipParts[2])
		}

		nodeDetails := Node{
			ID:         nodeID,
			Config:     nodeCfg,
			IPAddress:  ipAddress,
			Hostname:   hostname,
			Gateway:    gateway,
			DNSServers: []string{"1.1.1.1", "8.8.8.8"}, // Common DNS servers
		}
		log.Printf("Node Details: ID=%d, IP=%s, Hostname=%s, Board=%s, Gateway=%s",
			nodeDetails.ID, nodeDetails.IPAddress, nodeDetails.Hostname,
			nodeCfg.Board, nodeDetails.Gateway)

		// 3. Create Execution Context using our new constructor
		tpiCtx := NewContext(execCtx)

		// 4. Execute the User's Workflow Template
		log.Println("Executing user-defined workflow template...")
		wfErr := workflowTemplate(tpiCtx, tpi, nodeDetails)

		// 5. Handle Result
		if wfErr != nil {
			log.Printf("--- Workflow execution FAILED for Node %d: %v ---", nodeID, wfErr)
			return fmt.Errorf("workflow for Node %d failed: %w", nodeID, wfErr)
		}

		log.Printf("--- Workflow execution COMPLETED successfully for Node %d ---", nodeID)
		return nil
	}
}

// TODO: Define Context interface and implementation more completely.
// TODO: Implement robust derivation logic for Node details (Gateway, DNS, Hostname).

// Helper to get node config - assumes executor is not nil
// Exported for use by OS-specific packages
func (e *TuringPiExecutor) GetNodeConfig(nodeID NodeID) *NodeConfig {
	switch nodeID {
	case Node1:
		return e.config.Node1
	case Node2:
		return e.config.Node2
	case Node3:
		return e.config.Node3
	case Node4:
		return e.config.Node4
	default:
		return nil
	}
}

// GetStateManager returns the state manager instance.
func (e *TuringPiExecutor) GetStateManager() state.Manager {
	return e.stateManager
}

// GetCacheDir returns the absolute path to the configured cache directory.
func (e *TuringPiExecutor) GetCacheDir() string {
	return e.config.CacheDir
}

// GetPrepImageDir returns the path to the configured preparation image directory.
// Returns an empty string if not configured, indicating the system temp directory should be used.
func (e *TuringPiExecutor) GetPrepImageDir() string {
	return e.config.PrepImageDir
}

// SetCacheDir updates the cache directory path.
func (e *TuringPiExecutor) SetCacheDir(path string) {
	e.config.CacheDir = path
	log.Printf("Cache directory updated to: %s", path)
}

// GetBMCSSHConfig constructs the SSHConfig needed for bmc operations.
func (e *TuringPiExecutor) GetBMCSSHConfig() bmc.SSHConfig {
	// TODO: Make timeout configurable?
	return bmc.SSHConfig{
		Host:     e.config.IP,
		User:     e.config.BMCUser,
		Password: e.config.BMCPassword,
		Timeout:  30 * time.Second, // Default timeout for BMC ops
	}
}

// TODO: Implement calculateInputHash more robustly, maybe hash file contents for CopyLocalFile?
