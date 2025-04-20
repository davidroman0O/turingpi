package ubuntu

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi"
	"github.com/davidroman0O/turingpi/pkg/tpi/bmc"
	"github.com/davidroman0O/turingpi/pkg/tpi/node"
	osapi "github.com/davidroman0O/turingpi/pkg/tpi/os"
	"github.com/davidroman0O/turingpi/pkg/tpi/state"
)

// UbuntuOSInstallerBuilder defines the configuration for Phase 2: OS Installation for Ubuntu.
type UbuntuOSInstallerBuilder struct {
	nodeID        tpi.NodeID
	installConfig *InstallConfig
	imageResult   tpi.ImageResult
}

// InstallConfig holds configuration for installing Ubuntu.
type InstallConfig struct {
	BaseConfig
	NetworkingConfig
	RuntimeConfig

	// Target device where the OS will be installed
	TargetDevice string

	// User credentials
	Username string
	Password string
}

// CacheKey generates a unique key for caching installation state
func (c *InstallConfig) CacheKey() string {
	if c == nil {
		return ""
	}
	key := c.BaseConfig.CacheKey()
	if c.TargetDevice != "" {
		key += fmt.Sprintf("+dev:%s", filepath.Base(c.TargetDevice))
	}
	if c.Username != "" {
		key += fmt.Sprintf("+user:%s", c.Username)
	}
	return key
}

// NewOSInstaller creates a new builder for installing Ubuntu.
func NewOSInstaller(nodeID tpi.NodeID) *UbuntuOSInstallerBuilder {
	return &UbuntuOSInstallerBuilder{
		nodeID:        nodeID,
		installConfig: &InstallConfig{},
	}
}

// Configure accepts an OS-specific installation configuration.
func (b *UbuntuOSInstallerBuilder) Configure(config interface{}) error {
	installConfig, ok := config.(*InstallConfig)
	if !ok {
		return fmt.Errorf("expected *InstallConfig, got %T", config)
	}

	// Validate configuration
	if installConfig.TargetDevice == "" {
		return fmt.Errorf("target device must be specified")
	}
	if installConfig.Username == "" {
		return fmt.Errorf("username must be specified")
	}
	if installConfig.Password == "" {
		return fmt.Errorf("password must be specified")
	}

	b.installConfig = installConfig
	return nil
}

// UsingImage specifies the image to install.
func (b *UbuntuOSInstallerBuilder) UsingImage(result tpi.ImageResult) osapi.OSInstaller {
	b.imageResult = result
	return b
}

// calculateInputHash generates a hash representing the inputs to this phase.
func (b *UbuntuOSInstallerBuilder) calculateInputHash() (string, error) {
	h := sha256.New()

	// Hash Image Result
	imgResultString := fmt.Sprintf("%s:%s", b.imageResult.ImagePath, b.imageResult.InputHash)
	if _, err := h.Write([]byte(imgResultString)); err != nil {
		return "", err
	}

	// Hash InstallConfig
	if b.installConfig != nil {
		configString := fmt.Sprintf("%+v", *b.installConfig)
		if _, err := h.Write([]byte(configString)); err != nil {
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
	if b.imageResult.ImagePath == "" {
		return fmt.Errorf("phase 2 validation failed: UsingImage result from Phase 1 is required")
	}
	nodeConfig := cluster.GetNodeConfig(b.nodeID)
	if nodeConfig == nil {
		return fmt.Errorf("internal error: node config not found for Node %d", b.nodeID)
	}

	// --- Execute Installation --- //
	log.Printf("Starting OS installation for Node %d using image %s", b.nodeID, b.imageResult.ImagePath)

	var installErr error
	switch nodeConfig.Board {
	case state.RK1:
		log.Println("Executing RK1 flashing procedure...")
		installErr = b.flashRK1(ctx, cluster)
	case state.CM4:
		log.Println("Executing CM4 flashing procedure...")
		installErr = fmt.Errorf("CM4 flashing not yet implemented")
	default:
		installErr = fmt.Errorf("unsupported board type for installation: %s", nodeConfig.Board)
	}

	if installErr != nil {
		return fmt.Errorf("installation failed: %w", installErr)
	}

	log.Printf("--- Finished Phase 2: %s for Node %d ---", phaseName, b.nodeID)
	return nil
}

// flashRK1 implements the flashing process specifically for RK1 boards.
func (b *UbuntuOSInstallerBuilder) flashRK1(ctx tpi.Context, cluster tpi.Cluster) error {
	log.Println("[flashRK1] Starting RK1 flashing process...")
	bmcConfig := cluster.GetBMCSSHConfig()
	bmcAdapter := bmc.NewBMCAdapter(bmcConfig)
	nodeStr := fmt.Sprintf("%d", b.nodeID)

	localImagePath := b.imageResult.ImagePath
	imageXZName := filepath.Base(localImagePath)
	imageName := strings.TrimSuffix(imageXZName, ".xz")

	// Define remote paths on BMC
	remoteBaseDir := "/root/imgs"
	remoteNodeDir := filepath.Join(remoteBaseDir, nodeStr)
	remoteXZPath := filepath.Join(remoteNodeDir, imageXZName)
	remoteImgPath := filepath.Join(remoteNodeDir, imageName)

	// 1. Check if uncompressed image exists on BMC
	log.Printf("[flashRK1] Checking for existing uncompressed image on BMC: %s", remoteImgPath)
	imgExists, err := bmcAdapter.CheckFileExists(remoteImgPath)
	if err != nil {
		return fmt.Errorf("failed to check for existing uncompressed image %s on BMC: %w", remoteImgPath, err)
	}

	if !imgExists {
		log.Printf("[flashRK1] Uncompressed image %s not found on BMC.", remoteImgPath)

		// 2. Upload compressed image
		log.Printf("[flashRK1] Uploading compressed image %s to BMC:%s", localImagePath, remoteXZPath)
		err = bmcAdapter.UploadFile(localImagePath, remoteXZPath)
		if err != nil {
			return fmt.Errorf("failed to upload image %s to %s: %w", localImagePath, remoteXZPath, err)
		}
		log.Printf("[flashRK1] Upload successful.")

		// 3. Decompress on BMC
		log.Printf("[flashRK1] Decompressing %s on BMC...", remoteXZPath)
		cmdStr := fmt.Sprintf("unxz -f %s", remoteXZPath)
		_, _, err = bmcAdapter.ExecuteCommand(cmdStr)
		if err != nil {
			return fmt.Errorf("failed to decompress image %s on BMC: %w", remoteXZPath, err)
		}
		log.Printf("[flashRK1] Decompression successful on BMC.")
	} else {
		log.Printf("[flashRK1] Uncompressed image %s found on BMC. Skipping upload and decompression.", remoteImgPath)
	}

	// 4. Flash the image
	log.Printf("[flashRK1] Starting flash command: tpi flash -n %s -i %s", nodeStr, remoteImgPath)
	flashCmdStr := fmt.Sprintf("tpi flash --node %s -i %s", nodeStr, remoteImgPath)
	stdout, _, err := bmcAdapter.ExecuteCommand(flashCmdStr)
	if err != nil {
		// Look for specific BMC API failure pattern
		if strings.Contains(stdout, "127.0.0.1") &&
			(strings.Contains(stdout, "Connection refused") ||
				strings.Contains(stdout, "connect error")) {
			// This is a critical BMC API failure
			bmcErrorMsg := `
!!!!! CRITICAL BMC FAILURE DETECTED !!!!!

The Turing Pi BMC's internal API has failed with a connection refused error.
This indicates the BMC software is in an unrecoverable state.

REQUIRED ACTION:
1. Completely power off the Turing Pi board (unplug power)
2. Wait 10 seconds
3. Reconnect power and restart the board
4. Try the operation again after the BMC has fully restarted

This error cannot be fixed by software retries.
`
			log.Printf(bmcErrorMsg)
			return fmt.Errorf("BMC internal API failure detected: %s - %w\n\nThe BMC must be power cycled completely (unplug power)", stdout, err)
		}
		return fmt.Errorf("tpi flash command failed for node %s with image %s: %w", nodeStr, remoteImgPath, err)
	}
	log.Println("[flashRK1] Flash command completed successfully.")

	// 5. Power cycle the node
	log.Println("[flashRK1] Power cycling node...")
	powerOffCmd := fmt.Sprintf("tpi power off --node %s", nodeStr)
	_, _, err = bmcAdapter.ExecuteCommand(powerOffCmd)
	if err != nil {
		log.Printf("Warning: Power off command failed for node %s: %v", nodeStr, err)
	}

	time.Sleep(2 * time.Second)

	powerOnCmd := fmt.Sprintf("tpi power on --node %s", nodeStr)
	_, _, err = bmcAdapter.ExecuteCommand(powerOnCmd)
	if err != nil {
		return fmt.Errorf("power on command failed for node %s: %w", nodeStr, err)
	}

	log.Println("[flashRK1] Node power cycled.")

	// 6. Wait for node to boot by monitoring UART logs
	log.Printf("[flashRK1] Monitoring boot progress via UART...")
	// Make sure we're using normal mode before accessing UART
	normalModeCmd := fmt.Sprintf("tpi advanced --node %s normal", nodeStr)
	_, _, err = bmcAdapter.ExecuteCommand(normalModeCmd)
	if err != nil {
		log.Printf("Warning: Failed to set normal mode: %v", err)
	}

	// Get node IP for later SSH connection
	nodeConfig := cluster.GetNodeConfig(b.nodeID)
	if nodeConfig == nil {
		return fmt.Errorf("internal error: node config not found for Node %d", b.nodeID)
	}

	ipAddress := nodeConfig.IP
	if idx := strings.Index(ipAddress, "/"); idx != -1 {
		ipAddress = ipAddress[:idx]
	}

	// Create a timeout context for the boot monitoring
	bootCtx, bootCancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer bootCancel()

	// Track boot progress indicators
	var (
		systemdStarted     bool
		networkInitialized bool
		loginPromptFound   bool
	)

	// Monitor boot process through UART logs
	log.Printf("[flashRK1] Waiting for node to boot (IP: %s)...", ipAddress)

	// Wait for boot to complete by monitoring UART
	// We'll check every 5 seconds for up to 3 minutes total
	bootFailures := 0
	maxBootAttempts := 36 // 3 minutes total (36 * 5 seconds)

	for attempts := 0; attempts < maxBootAttempts; attempts++ {
		select {
		case <-bootCtx.Done():
			return fmt.Errorf("timeout waiting for node to boot")
		default:
			// Get UART logs
			uartCmd := fmt.Sprintf("tpi uart --node %s get", nodeStr)
			uartOutput, _, err := bmcAdapter.ExecuteCommand(uartCmd)
			if err != nil {
				log.Printf("[flashRK1] Warning: Failed to get UART logs: %v", err)
				bootFailures++
				if bootFailures > 5 {
					return fmt.Errorf("too many UART log retrieval failures")
				}
			} else {
				// Log the UART output with formatting
				formattedLines := strings.Split(uartOutput, "\n")
				for _, line := range formattedLines {
					if strings.TrimSpace(line) != "" {
						log.Printf("[flashRK1] UART: %s", line)
					}
				}

				// Look for key boot indicators
				if !systemdStarted && strings.Contains(uartOutput, "systemd[1]:") {
					log.Printf("[flashRK1] Boot progress: systemd started")
					systemdStarted = true
				}

				if !networkInitialized && (strings.Contains(uartOutput, "Reached target Network") ||
					strings.Contains(uartOutput, "eth0: Link is Up")) {
					log.Printf("[flashRK1] Boot progress: network initialization detected")
					networkInitialized = true
				}

				// Detect login prompt - most reliable indicator that system is ready
				if !loginPromptFound && (strings.Contains(uartOutput, "login:") ||
					strings.Contains(uartOutput, "turingpi-node1 login:")) {
					log.Printf("[flashRK1] Boot progress: login prompt detected! System is ready.")
					loginPromptFound = true
					// If login prompt is found, we can proceed to SSH
					break
				}

				// Check for common first-boot issues that are normal
				if strings.Contains(uartOutput, "journal corrupted or uncleanly shut down") {
					log.Printf("[flashRK1] Note: Journal corruption detected - this is normal for first boot")
				}
			}

			// Wait before checking again
			time.Sleep(5 * time.Second)
		}

		// If login prompt found, exit the loop early
		if loginPromptFound {
			break
		}
	}

	// Report boot status
	if !loginPromptFound {
		log.Printf("[flashRK1] Warning: Login prompt not detected in UART logs within timeout period")
	}

	// Always give the system some time to fully start SSH after login appears
	log.Printf("[flashRK1] Waiting 10 more seconds for SSH to fully initialize...")
	time.Sleep(10 * time.Second)

	// 7. Handle initial password setup
	log.Printf("[flashRK1] Attempting password change on %s@%s", b.installConfig.Username, ipAddress)

	// Create node adapter with retry for password change
	nodeAdapter := node.NewNodeAdapter(node.SSHConfig{
		Host:     ipAddress,
		User:     b.installConfig.Username,
		Password: "ubuntu", // Default initial password
		Timeout:  30 * time.Second,
	})

	// Change the default password
	steps := []node.InteractionStep{
		{Expect: "Current password:", Send: "ubuntu", LogMsg: "Sending initial password..."},
		{Expect: "New password:", Send: b.installConfig.Password, LogMsg: "Sending new password..."},
		{Expect: "Retype new password:", Send: b.installConfig.Password, LogMsg: "Retyping new password..."},
	}

	finalOutput, err := nodeAdapter.ExpectAndSend(steps, 30*time.Second)
	if err != nil {
		log.Printf("[flashRK1] Password change interaction failed: %v", err)
		log.Printf("[flashRK1] Final output: %s", finalOutput)

		// Check if the device is responding to ping
		// Try a simple SSH command from BMC - avoid using nc which isn't available
		sshCheckCmd := fmt.Sprintf("ssh -o ConnectTimeout=3 -o StrictHostKeyChecking=no %s@%s echo ALIVE",
			b.installConfig.Username, ipAddress)
		_, _, sshErr := bmcAdapter.ExecuteCommand(sshCheckCmd)
		if sshErr == nil {
			log.Printf("[flashRK1] Node is responding to SSH but password change failed")
		} else {
			log.Printf("[flashRK1] Node is not responding to SSH: %v", sshErr)
		}

		return fmt.Errorf("password change failed after flashing: %w", err)
	}

	if !strings.Contains(finalOutput, "passwd: password updated successfully") {
		log.Printf("[flashRK1] Password change did not report success. Output: %s", finalOutput)
		return fmt.Errorf("password change did not complete successfully after flashing")
	}

	log.Printf("[flashRK1] Password successfully changed for user %s", b.installConfig.Username)
	log.Printf("[flashRK1] Node is now accessible at %s", ipAddress)

	log.Println("[flashRK1] RK1 flashing process finished.")
	return nil
}

// Ensure OSInstaller implements os.OSInstaller interface
var _ osapi.OSInstaller = (*UbuntuOSInstallerBuilder)(nil)
