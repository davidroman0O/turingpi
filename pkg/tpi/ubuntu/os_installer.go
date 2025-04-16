package ubuntu

import (
	"crypto/sha256"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/bmc"
	"github.com/davidroman0O/turingpi/pkg/tpi" // Base tpi types
	// TODO: Need bmc package for actual flashing
	// "github.com/davidroman0O/turingpi/pkg/bmc"
)

// UbuntuOSInstallerBuilder defines the configuration for Phase 2: OS Installation for Ubuntu.
type UbuntuOSInstallerBuilder struct {
	nodeID        tpi.NodeID              // The specific node to install on
	installConfig tpi.UbuntuInstallConfig // OS-specific config
	genericConfig *tpi.OSInstallConfig    // Optional generic config
	imageResult   *tpi.ImageResult        // Result from Phase 1
}

// NewOSInstaller creates a new builder for installing Ubuntu.
// It requires the node ID and Ubuntu-specific configuration.
func NewOSInstaller(nodeID tpi.NodeID, config tpi.UbuntuInstallConfig) *UbuntuOSInstallerBuilder {
	return &UbuntuOSInstallerBuilder{
		nodeID:        nodeID,
		installConfig: config,
	}
}

// UsingImage specifies the customized image to be installed (result from Phase 1). REQUIRED.
func (b *UbuntuOSInstallerBuilder) UsingImage(result *tpi.ImageResult) *UbuntuOSInstallerBuilder {
	b.imageResult = result
	return b
}

// WithGenericConfig adds generic OS installation options (e.g., SSH keys).
func (b *UbuntuOSInstallerBuilder) WithGenericConfig(config tpi.OSInstallConfig) *UbuntuOSInstallerBuilder {
	b.genericConfig = &config
	return b
}

// calculateInputHash generates a hash representing the inputs to this phase.
func (b *UbuntuOSInstallerBuilder) calculateInputHash() (string, error) {
	h := sha256.New()

	// Hash Image Result (path and input hash from phase 1)
	if b.imageResult != nil {
		imgResultString := fmt.Sprintf("%s:%s", b.imageResult.ImagePath, b.imageResult.InputHash)
		if _, err := h.Write([]byte(imgResultString)); err != nil {
			return "", err
		}
	} else {
		return "", fmt.Errorf("cannot calculate hash without image result")
	}

	// Hash UbuntuInstallConfig
	uicString := fmt.Sprintf("%+v", b.installConfig)
	if _, err := h.Write([]byte(uicString)); err != nil {
		return "", err
	}

	// Hash Generic Config (if present)
	if b.genericConfig != nil {
		gcString := fmt.Sprintf("%+v", *b.genericConfig)
		if _, err := h.Write([]byte(gcString)); err != nil {
			return "", err
		}
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Run executes the OS installation phase.
func (b *UbuntuOSInstallerBuilder) Run(ctx tpi.Context, cluster tpi.Cluster) error {
	phaseName := "OSInstallation"
	log.Printf("--- Starting Phase 2: %s for Node %d (Ubuntu) ---", phaseName, b.nodeID)

	// --- Validate Builder Config ---
	if b.imageResult == nil || b.imageResult.ImagePath == "" {
		return fmt.Errorf("phase 2 validation failed: UsingImage result from Phase 1 is required")
	}
	nodeConfig := cluster.GetNodeConfig(b.nodeID)
	if nodeConfig == nil {
		return fmt.Errorf("internal error: node config not found for Node %d", b.nodeID)
	}
	if nodeConfig.Board != b.imageResult.Board {
		return fmt.Errorf("phase 2 validation failed: board type mismatch between node config (%s) and image result (%s)", nodeConfig.Board, b.imageResult.Board)
	}

	// --- Calculate Input Hash ---
	inputHash, err := b.calculateInputHash()
	if err != nil {
		return fmt.Errorf("failed to calculate input hash: %w", err)
	}
	log.Printf("Calculated input hash: %s", inputHash)

	// --- Check State ---
	stateMgr := cluster.GetStateManager()
	currentState := stateMgr.GetNodeState(b.nodeID).OSInstallation

	if currentState.Status == tpi.StatusCompleted && currentState.InputHash == inputHash {
		log.Printf("Phase 2 already completed with matching inputs. Skipping execution.")
		return nil
	}
	if currentState.Status == tpi.StatusRunning {
		return fmt.Errorf("phase 2 is already marked as running for node %d (state timestamp: %s). Manual intervention might be required", b.nodeID, currentState.Timestamp)
	}

	// --- Mark State as Running ---
	err = stateMgr.UpdatePhaseState(b.nodeID, phaseName, tpi.StatusRunning, inputHash, "", nil)
	if err != nil {
		return fmt.Errorf("failed to update state to running: %w", err)
	}

	// --- Execute Installation --- //
	log.Printf("Starting OS installation for Node %d using image %s", b.nodeID, b.imageResult.ImagePath)

	var installErr error
	switch nodeConfig.Board {
	case tpi.RK1:
		log.Println("Executing RK1 flashing procedure...")
		installErr = b.flashRK1(ctx, cluster)
	case tpi.CM4:
		log.Println("Executing CM4 flashing procedure...")
		installErr = fmt.Errorf("CM4 flashing not yet implemented")
	default:
		installErr = fmt.Errorf("unsupported board type for installation: %s", nodeConfig.Board)
	}

	// --- Update State ---
	if installErr != nil {
		return b.failPhase(cluster, installErr)
	}

	err = stateMgr.UpdatePhaseState(b.nodeID, phaseName, tpi.StatusCompleted, inputHash, "", nil)
	if err != nil {
		log.Printf("Warning: Failed to update state to completed, but phase finished: %v", err)
	}

	log.Printf("--- Finished Phase 2: %s for Node %d ---", phaseName, b.nodeID)
	return nil
}

// flashRK1 implements the flashing process specifically for RK1 boards.
func (b *UbuntuOSInstallerBuilder) flashRK1(ctx tpi.Context, cluster tpi.Cluster) error {
	log.Println("[flashRK1] Starting RK1 flashing process...")
	bmcSSHConfig := cluster.GetBMCSSHConfig()
	nodeStr := fmt.Sprintf("%d", b.nodeID)

	localImagePath := b.imageResult.ImagePath // Path to the *.img.xz file in the local cache
	imageXZName := filepath.Base(localImagePath)
	imageName := strings.TrimSuffix(imageXZName, ".xz") // Target uncompressed name

	// Define remote paths on BMC
	remoteBaseDir := "/root/imgs" // Standard cache location on BMC
	remoteNodeDir := filepath.Join(remoteBaseDir, nodeStr)
	remoteXZPath := filepath.Join(remoteNodeDir, imageXZName)
	remoteImgPath := filepath.Join(remoteNodeDir, imageName) // Path to the required uncompressed image

	var err error

	// 1. Check if uncompressed image exists on BMC
	log.Printf("[flashRK1] Checking for existing uncompressed image on BMC: %s", remoteImgPath)
	imgExists, err := bmc.CheckRemoteFileExists(bmcSSHConfig, remoteImgPath)
	if err != nil {
		// Treat check failure as potentially fatal, as we can't determine state
		return fmt.Errorf("failed to check for existing uncompressed image %s on BMC: %w", remoteImgPath, err)
	}

	if !imgExists {
		log.Printf("[flashRK1] Uncompressed image %s not found on BMC.", remoteImgPath)

		// 2. Check/Transfer compressed image
		log.Printf("[flashRK1] Checking/Uploading compressed image %s to BMC:%s", localImagePath, remoteXZPath)
		// TODO: Add check for existing XZ? For now, always upload if uncompressed is missing.
		err = bmc.UploadFile(bmcSSHConfig, localImagePath, remoteXZPath)
		if err != nil {
			return fmt.Errorf("failed to upload image %s to %s: %w", localImagePath, remoteXZPath, err)
		}
		log.Printf("[flashRK1] Upload successful.")

		// 3. Decompress on BMC
		log.Printf("[flashRK1] Decompressing %s on BMC...", remoteXZPath)
		// Use -f to force overwrite if xz somehow exists but img doesn't.
		// Use -k to keep the .xz file after decompression?
		cmdStr := fmt.Sprintf("unxz -f %s", remoteXZPath)
		_, _, err = bmc.ExecuteCommand(bmcSSHConfig, cmdStr)
		if err != nil {
			// Attempt cleanup? Remove XZ? Difficult state.
			return fmt.Errorf("failed to decompress image %s on BMC: %w", remoteXZPath, err)
		}
		log.Printf("[flashRK1] Decompression successful on BMC.")

	} else {
		log.Printf("[flashRK1] Uncompressed image %s found on BMC. Skipping upload and decompression.", remoteImgPath)
	}

	// 4. Flash the (now definitely existing) uncompressed image
	log.Printf("[flashRK1] Starting flash command: tpi flash -n %s -i %s", nodeStr, remoteImgPath)
	// TODO: Consider timeout for flashing?
	flashCmdStr := fmt.Sprintf("tpi flash --node %s -i %s", nodeStr, remoteImgPath)
	_, _, err = bmc.ExecuteCommand(bmcSSHConfig, flashCmdStr)
	if err != nil {
		return fmt.Errorf("tpi flash command failed for node %s with image %s: %w", nodeStr, remoteImgPath, err)
	}
	log.Println("[flashRK1] Flash command completed successfully.")

	// 5. Power cycle the node
	log.Println("[flashRK1] Power cycling node...")
	powerOffCmd := fmt.Sprintf("tpi power off --node %s", nodeStr)
	_, _, err = bmc.ExecuteCommand(bmcSSHConfig, powerOffCmd)
	if err != nil {
		// Log warning but maybe proceed? Power on might still work.
		log.Printf("Warning: Power off command failed for node %s: %v", nodeStr, err)
	}

	// Add a small delay between off and on
	time.Sleep(2 * time.Second)

	powerOnCmd := fmt.Sprintf("tpi power on --node %s", nodeStr)
	_, _, err = bmc.ExecuteCommand(bmcSSHConfig, powerOnCmd)
	if err != nil {
		// This is more critical - if power on fails, node is left off.
		return fmt.Errorf("power on command failed for node %s: %w", nodeStr, err)
	}

	log.Println("[flashRK1] Node power cycled.")
	log.Println("[flashRK1] RK1 flashing process finished.")
	return nil
}

// failPhase is a helper to update state on failure and return the error.
func (b *UbuntuOSInstallerBuilder) failPhase(cluster tpi.Cluster, err error) error {
	phaseName := "OSInstallation"
	log.Printf("--- Error in Phase 2: %s for Node %d ---", phaseName, b.nodeID)
	log.Printf("Error details: %v", err)
	_ = cluster.GetStateManager().UpdatePhaseState(b.nodeID, phaseName, tpi.StatusFailed, "", "", err)
	return err
}
