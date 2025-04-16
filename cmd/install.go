/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// Variables to store flag values for install command
var (
	installNodeID int
	imagePath     string
)

// installCmd represents the install command
var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Flash an OS image to a compute node's storage",
	Long: `Installs an operating system by flashing a disk image (.img, .raw) 
to the specified compute node's internal storage (e.g., eMMC).

Requires specifying the target node (--node) and the image file path (--image).
This process can take several minutes depending on the image size.
The node might be power-cycled automatically during the process.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Validate node ID
		if installNodeID < 1 || installNodeID > 4 {
			fmt.Fprintf(os.Stderr, "Error: Invalid node ID %d. Please specify a node between 1 and 4 using --node.\n", installNodeID)
			cmd.Usage()
			os.Exit(1)
		}

		// Basic validation for image path (just check if provided)
		if imagePath == "" {
			fmt.Fprint(os.Stderr, "Error: Image file path cannot be empty. Please specify using --image.\n")
			cmd.Usage()
			os.Exit(1)
		}
		// Note: We don't check if the file exists here, 'tpi' command will handle that.

		// Construct arguments for the tpi command
		cmdArgs := []string{
			"flash",
			"--node", fmt.Sprintf("%d", installNodeID),
			"--image", imagePath,
			"--host", bmcHost, // Use persistent flag value
			"--user", bmcUser, // Use persistent flag value
		}
		if bmcPassword != "" { // Use persistent flag value
			cmdArgs = append(cmdArgs, "--password", bmcPassword)
		}

		// Prepare and execute the command
		execCmd := exec.Command("tpi", cmdArgs...)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr

		fmt.Println("Executing: tpi", cmdArgs)
		fmt.Println("Flashing process may take several minutes. Please wait...")
		err := execCmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing tpi flash command: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Flash command completed for node %d with image %s.\n", installNodeID, imagePath)
		fmt.Println("You may need to power cycle the node ('power off' then 'power on') to boot the new OS.")
	},
}

func init() {
	rootCmd.AddCommand(installCmd)

	// Add local flags specific to the install command
	installCmd.Flags().IntVarP(&installNodeID, "node", "n", 0, "Target node number (1-4) to flash (required)")
	installCmd.Flags().StringVarP(&imagePath, "image", "i", "", "Path to the OS image file (.img, .raw) (required)")

	// Mark flags as required
	if err := installCmd.MarkFlagRequired("node"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'node' as required: %v\n", err)
		os.Exit(1)
	}
	if err := installCmd.MarkFlagRequired("image"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'image' as required: %v\n", err)
		os.Exit(1)
	}
}
