package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/state"
	"github.com/davidroman0O/turingpi/pkg/tpi/bmc" // Import the new package

	"github.com/spf13/cobra"
	// No longer needed: github.com/davidroman0O/firm-go
	// No longer needed: github.com/pkg/sftp
	// No longer needed: golang.org/x/crypto/ssh
	// No longer needed: bytes
	// No longer needed: io
)

// Variables to store flag values for install-ubuntu command
var (
	ubuntuNodeID    int
	ubuntuImagePath string
)

// --- installUbuntuCmd ---
var installUbuntuCmd = &cobra.Command{
	Use:   "install-ubuntu",
	Short: "Install Ubuntu OS onto a target node using a prepared image",
	Long: `Transfers a prepared Ubuntu image (.img.xz) to the BMC, decompresses it,
flashes it onto the target compute node (1-4), and power-cycles the node.

Requires:
- Node ID (--node)
- Local path to the *prepared* Ubuntu image (--image-path, e.g., rk1-node1.img.xz).
  This image should have been created using the 'prepare' command.
- BMC credentials (global flags).

Workflow:
1. Check if uncompressed image exists on BMC in /root/imgs/<node_id>/.
2. If not, transfer the compressed image (.img.xz) to BMC via SCP.
3. Decompress the image on BMC using 'unxz'.
4. Flash the image using 'tpi flash'.
5. Power cycle the node using 'tpi power off/on'.
6. Updates node status in the local state file.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting Ubuntu installation process...")

		// --- Input Validation ---
		if ubuntuNodeID < 1 || ubuntuNodeID > 4 {
			fmt.Fprintf(os.Stderr, "Error: Invalid node ID %d. Use --node with a value between 1 and 4.\n", ubuntuNodeID)
			cmd.Usage()
			os.Exit(1)
		}
		if ubuntuImagePath == "" {
			fmt.Fprint(os.Stderr, "Error: Prepared image file path cannot be empty. Use --image-path.\n")
			cmd.Usage()
			os.Exit(1)
		}
		if _, err := os.Stat(ubuntuImagePath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Local prepared image file not found at: %s\n", ubuntuImagePath)
			os.Exit(1)
		}
		if !strings.HasSuffix(strings.ToLower(ubuntuImagePath), ".img.xz") {
			fmt.Fprintf(os.Stderr, "Error: Image path must point to a prepared '.img.xz' file.\n")
			os.Exit(1)
		}

		localAbsImagePath, _ := filepath.Abs(ubuntuImagePath)
		fmt.Printf("Using local prepared image: %s for node %d\n", localAbsImagePath, ubuntuNodeID)

		// Extract filenames and define remote paths
		imageXZName := filepath.Base(localAbsImagePath)
		imageName := strings.TrimSuffix(imageXZName, ".xz")
		nodeStr := fmt.Sprintf("%d", ubuntuNodeID)
		remoteBaseDir := "/root/imgs"
		remoteNodeDir := filepath.Join(remoteBaseDir, nodeStr)
		remoteImgPath := filepath.Join(remoteNodeDir, imageName)
		remoteXZPath := filepath.Join(remoteNodeDir, imageXZName)

		// Create BMC SSH Config from global flags
		bmcSSHConfig := bmc.SSHConfig{
			Host:     bmcHost,
			User:     bmcUser,
			Password: bmcPassword,
			Timeout:  20 * time.Second, // Or make configurable
		}

		// Create BMC adapter
		bmcAdapter := bmc.NewBMCAdapter(bmcSSHConfig)

		// --- Installation Sequence ---
		var err error

		// 1. Check if uncompressed image exists on BMC
		log.Printf("Checking for existing image on BMC: %s\n", remoteImgPath)
		imgExists, err := bmcAdapter.CheckFileExists(remoteImgPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError checking remote file: %v\n", err)
			os.Exit(1)
		}

		if !imgExists {
			log.Printf("Uncompressed image %s not found on BMC. Proceeding with transfer and decompression.\n", remoteImgPath)

			// 2. Transfer compressed image if needed
			log.Printf("Transferring %s to BMC:%s\n", localAbsImagePath, remoteXZPath)
			err = bmcAdapter.UploadFile(localAbsImagePath, remoteXZPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nError uploading image: %v\n", err)
				os.Exit(1)
			}
			log.Println("SCP upload successful.")

			// 3. Decompress on BMC
			log.Printf("Decompressing image on BMC: %s\n", remoteXZPath)
			cmdStr := fmt.Sprintf("unxz -f %s", remoteXZPath) // -f forces overwrite
			stdout, stderr, err := bmcAdapter.ExecuteCommand(cmdStr)
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nError decompressing image on BMC: %v\nStderr: %s\n", err, stderr)
				os.Exit(1)
			}
			if stderr != "" {
				log.Printf("Warning: stderr from decompression: %s", stderr)
			}
			if stdout != "" {
				log.Printf("Decompression output: %s", stdout)
			}
			log.Println("Decompression successful on BMC.")
		} else {
			log.Printf("Uncompressed image %s found on BMC. Skipping transfer and decompression.\n", remoteImgPath)
		}

		// 4. Flash the image
		log.Println("Starting flash process using tpi...")
		err = state.UpdateNodeState(ubuntuNodeID, func(s *state.NodeStatus) { s.Status = "installing"; s.OS = "ubuntu"; s.Error = "" })
		if err != nil {
			log.Printf("Warning: failed to update state to installing: %v", err)
		}

		flashCmdStr := fmt.Sprintf("tpi flash --node %s -i %s", nodeStr, remoteImgPath)
		stdout, stderr, err := bmcAdapter.ExecuteCommand(flashCmdStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError flashing node: %v\nStderr: %s\n", err, stderr)
			_ = state.UpdateNodeState(ubuntuNodeID, func(s *state.NodeStatus) { s.Status = "install_failed"; s.Error = err.Error() })
			os.Exit(1)
		}
		if stderr != "" {
			log.Printf("Warning: stderr from flash: %s", stderr)
		}
		if stdout != "" {
			log.Printf("Flash output: %s", stdout)
		}
		log.Println("Flashing completed successfully.")

		// 5. Power cycle the node
		log.Println("Powering off node...")
		time.Sleep(2 * time.Second)
		powerOffCmdStr := fmt.Sprintf("tpi power off --node %s", nodeStr)
		stdout, stderr, err = bmcAdapter.ExecuteCommand(powerOffCmdStr)
		if err != nil {
			// Log warning but proceed? Or fail?
			fmt.Fprintf(os.Stderr, "\nWarning: Power off command failed (proceeding anyway): %v\nStderr: %s\n", err, stderr)
			// Decide if this should be fatal or just a warning
			// os.Exit(1)
		} else {
			if stderr != "" {
				log.Printf("Warning: stderr from power off: %s", stderr)
			}
			if stdout != "" {
				log.Printf("Power off output: %s", stdout)
			}
			log.Println("Power off successful.")
		}

		log.Println("Powering on node...")
		time.Sleep(2 * time.Second)
		powerOnCmdStr := fmt.Sprintf("tpi power on --node %s", nodeStr)
		stdout, stderr, err = bmcAdapter.ExecuteCommand(powerOnCmdStr)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError powering on node: %v\nStderr: %s\n", err, stderr)
			_ = state.UpdateNodeState(ubuntuNodeID, func(s *state.NodeStatus) {
				s.Status = "install_failed"
				s.Error = fmt.Sprintf("power on failed: %v", err)
			})
			os.Exit(1)
		}
		if stderr != "" {
			log.Printf("Warning: stderr from power on: %s", stderr)
		}
		if stdout != "" {
			log.Printf("Power on output: %s", stdout)
		}
		log.Println("Power on successful.")

		// 6. Update state
		err = state.UpdateNodeState(ubuntuNodeID, func(s *state.NodeStatus) { s.Status = "installed"; s.OS = "ubuntu"; s.Error = "" })
		if err != nil {
			log.Printf("Warning: failed to update final state: %v", err)
		}

		fmt.Println("\nUbuntu installation process completed successfully!")
		fmt.Println("Node should be booting Ubuntu with the prepared configuration.")
		fmt.Println("Run 'post-install-ubuntu' command next if needed.")
	},
}

func init() {
	rootCmd.AddCommand(installUbuntuCmd)

	installUbuntuCmd.Flags().IntVarP(&ubuntuNodeID, "node", "n", 0, "Target node number (1-4) (required)")
	installUbuntuCmd.Flags().StringVarP(&ubuntuImagePath, "image-path", "i", "", "Local path to the prepared Ubuntu '.img.xz' image file (required)")

	if err := installUbuntuCmd.MarkFlagRequired("node"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'node' as required: %v\n", err)
		os.Exit(1)
	}
	if err := installUbuntuCmd.MarkFlagRequired("image-path"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'image-path' as required: %v\n", err)
		os.Exit(1)
	}
}
