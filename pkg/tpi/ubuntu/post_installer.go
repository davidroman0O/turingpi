package ubuntu

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi" // Base tpi types
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
	ubuntuPassword string // Default Ubuntu user password for SSH connection
}

// NewPostInstaller creates a new builder for post-installation setup of Ubuntu.
// It requires the node ID.
func NewPostInstaller(nodeID tpi.NodeID) *UbuntuPostInstallerBuilder {
	return &UbuntuPostInstallerBuilder{
		nodeID: nodeID,
	}
}

// RunActions specifies the callback function that will be executed
// during post-installation setup. It provides a LocalRuntime for local file/command operations
// and a UbuntuRuntime for remote SSH operations on the target node.
func (b *UbuntuPostInstallerBuilder) RunActions(actionsFunc func(local tpi.LocalRuntime, remote tpi.UbuntuRuntime) error) *UbuntuPostInstallerBuilder {
	b.actionsFunc = actionsFunc
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
	if b.ubuntuPassword == "" {
		return fmt.Errorf("phase 3 validation failed: WithPassword is required for SSH connection")
	}
	nodeConfig := cluster.GetNodeConfig(b.nodeID)
	if nodeConfig == nil {
		return fmt.Errorf("internal error: node config not found for Node %d", b.nodeID)
	}

	// --- Calculate Input Hash (or skip) ---
	inputHash, _ := b.calculateInputHash() // Ignore error for now

	// --- Check State ---
	stateMgr := cluster.GetStateManager()
	currentState := stateMgr.GetNodeState(b.nodeID).PostInstallation

	if currentState.Status == tpi.StatusCompleted /* && currentState.InputHash == inputHash */ {
		log.Printf("Phase 3 already completed. Skipping execution.")
		return nil
	}
	if currentState.Status == tpi.StatusRunning {
		return fmt.Errorf("phase 3 is already marked as running for node %d (state timestamp: %s). Manual intervention might be required", b.nodeID, currentState.Timestamp)
	}

	// --- Mark State as Running ---
	err := stateMgr.UpdatePhaseState(b.nodeID, phaseName, tpi.StatusRunning, inputHash, "", nil)
	if err != nil {
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
	remoteRuntime := newUbuntuRuntime(ipAddress, "ubuntu", b.ubuntuPassword)

	// Execute the user-provided function
	actionsErr := b.actionsFunc(localRuntime, remoteRuntime)

	// --- Update State ---
	if actionsErr != nil {
		return b.failPhase(cluster, actionsErr)
	}

	err = stateMgr.UpdatePhaseState(b.nodeID, phaseName, tpi.StatusCompleted, inputHash, "", nil)
	if err != nil {
		log.Printf("Warning: Failed to update state to completed, but phase finished: %v", err)
	}

	log.Printf("--- Finished Phase 3: %s for Node %d ---", phaseName, b.nodeID)
	return nil
}

// failPhase is a helper to update state on failure and return the error.
func (b *UbuntuPostInstallerBuilder) failPhase(cluster tpi.Cluster, err error) error {
	phaseName := "PostInstallation"
	log.Printf("--- Error in Phase 3: %s for Node %d ---", phaseName, b.nodeID)
	log.Printf("Error details: %v", err)
	_ = cluster.GetStateManager().UpdatePhaseState(b.nodeID, phaseName, tpi.StatusFailed, "", "", err)
	return err
}
