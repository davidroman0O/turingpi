package ubuntu

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi" // Base tpi types
	"github.com/davidroman0O/turingpi/pkg/tpi/state"
)

// localRuntimeImpl implements the tpi.LocalRuntime interface
type localRuntimeImpl struct {
	cacheDir string
}

// CopyFile implements tpi.LocalRuntime
func (r *localRuntimeImpl) CopyFile(localPath, remotePath string, toRemote bool) error {
	// Implementation would go here - for now return not implemented error
	return fmt.Errorf("not implemented")
}

// ReadFile implements tpi.LocalRuntime
func (r *localRuntimeImpl) ReadFile(localPath string) ([]byte, error) {
	return os.ReadFile(localPath)
}

// WriteFile implements tpi.LocalRuntime
func (r *localRuntimeImpl) WriteFile(localPath string, data []byte, perm os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory for file: %w", err)
	}
	return os.WriteFile(localPath, data, perm)
}

// RunCommand implements tpi.LocalRuntime
func (r *localRuntimeImpl) RunCommand(command string, timeout time.Duration) (stdout, stderr string, err error) {
	// Implementation would go here - for now return not implemented error
	return "", "", fmt.Errorf("not implemented")
}

// UbuntuPostInstallerBuilder defines the configuration for Phase 3: Post-Installation Setup for Ubuntu.
type UbuntuPostInstallerBuilder struct {
	nodeID         tpi.NodeID
	actionsFunc    func(local tpi.LocalRuntime, remote tpi.UbuntuRuntime) error
	ubuntuUser     string // Default Ubuntu username for SSH connection
	ubuntuPassword string // Default Ubuntu user password for SSH connection
}

// NewPostInstaller creates a new builder for post-installation setup of Ubuntu.
// It requires the node ID.
func NewPostInstaller(nodeID tpi.NodeID) *UbuntuPostInstallerBuilder {
	return &UbuntuPostInstallerBuilder{
		nodeID:     nodeID,
		ubuntuUser: "ubuntu", // Default username
	}
}

// RunActions specifies the callback function that will be executed
// during post-installation setup. It provides a LocalRuntime for local file/command operations
// and a UbuntuRuntime for remote SSH operations on the target node.
func (b *UbuntuPostInstallerBuilder) RunActions(actionsFunc func(local tpi.LocalRuntime, remote tpi.UbuntuRuntime) error) *UbuntuPostInstallerBuilder {
	b.actionsFunc = actionsFunc
	return b
}

// WithUser sets the username for SSH connection
func (b *UbuntuPostInstallerBuilder) WithUser(username string) *UbuntuPostInstallerBuilder {
	b.ubuntuUser = username
	return b
}

// WithPassword sets the password for the default Ubuntu user for SSH connection
func (b *UbuntuPostInstallerBuilder) WithPassword(password string) *UbuntuPostInstallerBuilder {
	b.ubuntuPassword = password
	return b
}

// calculateInputHash generates a hash representing the inputs to this phase.
func (b *UbuntuPostInstallerBuilder) calculateInputHash() (string, error) {
	// Placeholder - Cannot reliably hash the actionsFunc
	return "no-hash-for-actions", nil
}

// Run executes the post-installation setup phase.
func (b *UbuntuPostInstallerBuilder) Run(ctx tpi.Context, cluster tpi.Cluster) error {
	phaseName := "PostInstallation"
	log.Printf("--- Starting Phase 3: %s for Node %d (Ubuntu) ---", phaseName, b.nodeID)

	// --- Validate Builder Config ---
	if b.actionsFunc == nil {
		return fmt.Errorf("phase 3 validation failed: RunActions function is required")
	}
	if b.ubuntuUser == "" {
		return fmt.Errorf("phase 3 validation failed: WithUser is required for SSH connection")
	}
	if b.ubuntuPassword == "" {
		return fmt.Errorf("phase 3 validation failed: WithPassword is required for SSH connection")
	}
	nodeConfig := cluster.GetNodeConfig(b.nodeID)
	if nodeConfig == nil {
		return fmt.Errorf("internal error: node config not found for Node %d", b.nodeID)
	}

	// --- Check State ---
	stateManager := cluster.GetStateManager()

	nodeState, err := stateManager.GetNodeState(state.NodeID(b.nodeID))
	if err == nil && nodeState != nil {
		// Check if this phase was already completed with the same hash
		if !nodeState.LastConfigTime.IsZero() &&
			nodeState.LastError == "" {
			log.Printf("Phase 3 already completed. Skipping execution.")
			return nil
		}

		// Check if already running
		if nodeState.LastOperation == fmt.Sprintf("Start%s", phaseName) {
			// This is an approximation - in a real solution we'd have better phase tracking
			return fmt.Errorf("phase 3 appears to be already running for node %d. Manual intervention might be required", b.nodeID)
		}
	}

	// Mark as running
	properties := map[string]interface{}{
		"LastOperation":     fmt.Sprintf("Start%s", phaseName),
		"LastOperationTime": time.Now(),
	}
	if err := stateManager.UpdateNodeProperties(state.NodeID(b.nodeID), properties); err != nil {
		return fmt.Errorf("failed to update state to running: %w", err)
	}

	// --- Execute Actions --- //
	log.Printf("Executing post-installation actions for Node %d (%s)...", b.nodeID, nodeConfig.IP)

	// Setup the local runtime helper for the actions function
	localRuntime := &localRuntimeImpl{
		cacheDir: cluster.GetCacheDir(),
	}

	// Get the node's IP information for SSH connection
	ipAddress := nodeConfig.IP
	if idx := strings.Index(ipAddress, "/"); idx != -1 {
		ipAddress = ipAddress[:idx] // Remove CIDR notation if present
	}

	// Create runtime instances
	remoteRuntime := newUbuntuRuntime(ipAddress, b.ubuntuUser, b.ubuntuPassword)

	// Execute the user-provided function
	actionsErr := b.actionsFunc(localRuntime, remoteRuntime)

	// --- Update State ---
	if actionsErr != nil {
		return b.failPhase(cluster, actionsErr)
	}

	// Update completion state
	properties = map[string]interface{}{
		"LastOperation":     fmt.Sprintf("Complete%s", phaseName),
		"LastOperationTime": time.Now(),
		"LastConfigTime":    time.Now(),
		"LastError":         "",
	}
	if err := stateManager.UpdateNodeProperties(state.NodeID(b.nodeID), properties); err != nil {
		log.Printf("Warning: Failed to update state to completed, but phase finished: %v", err)
	}

	log.Printf("--- Finished Phase 3: %s for Node %d ---", phaseName, b.nodeID)
	return nil
}

// failPhase updates the state and returns the error
func (b *UbuntuPostInstallerBuilder) failPhase(cluster tpi.Cluster, err error) error {
	phaseName := "PostInstallation"
	log.Printf("Phase %s failed: %v", phaseName, err)

	stateManager := cluster.GetStateManager()
	properties := map[string]interface{}{
		"LastOperation":     fmt.Sprintf("Failed%s", phaseName),
		"LastOperationTime": time.Now(),
		"LastError":         err.Error(),
	}
	if updateErr := stateManager.UpdateNodeProperties(state.NodeID(b.nodeID), properties); updateErr != nil {
		log.Printf("Warning: Failed to update state after failure: %v", updateErr)
	}

	return err
}
