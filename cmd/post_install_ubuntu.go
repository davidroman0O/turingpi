package cmd

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/node" // Import the new package
	"github.com/spf13/cobra"
	// "golang.org/x/crypto/ssh" // No longer needed directly here
	// "bufio"
	// "bytes"
	// "io"
)

var (
	postInstallNodeIP      string
	postInstallInitialUser string
	postInstallInitialPass string
	postInstallNewPass     string
)

// postInstallUbuntuCmd represents the post-install-ubuntu command
var postInstallUbuntuCmd = &cobra.Command{
	Use:   "post-install-ubuntu",
	Short: "Perform initial setup (password change) for Ubuntu nodes",
	Long: `Connects directly to a freshly installed Ubuntu node via SSH 
and automates the mandatory initial password change process.

Requires the node to be booted and reachable via the provided IP address.
Uses the default initial credentials (ubuntu/ubuntu) unless overridden.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting Ubuntu post-installation steps...")

		// --- Input Validation ---
		if postInstallNodeIP == "" {
			fmt.Fprintln(os.Stderr, "Error: --node-ip is required.")
			os.Exit(1)
		}
		if postInstallNewPass == "" {
			fmt.Fprintln(os.Stderr, "Error: --new-password is required.")
			os.Exit(1)
		}

		// Create node adapter
		adapter := node.NewNodeAdapter(node.SSHConfig{
			Host:     postInstallNodeIP,
			User:     postInstallInitialUser,
			Password: postInstallInitialPass,
			Timeout:  30 * time.Second,
		})

		// Define the interaction steps for Ubuntu password change
		steps := []node.InteractionStep{
			{Expect: "Current password:", Send: postInstallInitialPass, LogMsg: "Sending initial password..."},
			{Expect: "New password:", Send: postInstallNewPass, LogMsg: "Sending new password..."},
			{Expect: "Retype new password:", Send: postInstallNewPass, LogMsg: "Retyping new password..."},
		}

		// Execute the interaction
		finalOutput, err := adapter.ExpectAndSend(steps, 30*time.Second)

		// --- Verification ---
		// Check for errors first
		if err != nil {
			// Check for specific known errors based on the error message (a bit brittle)
			if strings.Contains(err.Error(), "timeout waiting for target string") || strings.Contains(err.Error(), "EOF reached before finding target") {
				fmt.Fprintf(os.Stderr, "\nPost-installation failed: Timed out or connection closed while waiting for prompt. Is the node responsive at %s? Did it already complete setup?\nError: %v\n", postInstallNodeIP, err)
			} else {
				fmt.Fprintf(os.Stderr, "\nPost-installation failed: %v\n", err)
			}
			// Log final output for debugging
			log.Printf("Final output before error:\n%s", finalOutput)
			os.Exit(1)
		}

		// Check final output content for success or failure messages
		if strings.Contains(finalOutput, "passwd: password updated successfully") {
			fmt.Println("\nPost-installation interaction sequence completed successfully.")
			fmt.Println("Password change confirmed by output.")
			fmt.Println("You should now be able to SSH into the node with the new password.")
		} else if strings.Contains(finalOutput, "You must choose a longer password") {
			fmt.Fprintf(os.Stderr, "\nPost-installation failed: Password rejected by node: too short.\n")
			log.Printf("Final output:\n%s", finalOutput)
			os.Exit(1)
		} else if strings.Contains(finalOutput, "Sorry, passwords do not match") {
			fmt.Fprintf(os.Stderr, "\nPost-installation failed: Password rejected by node: passwords do not match.\n")
			log.Printf("Final output:\n%s", finalOutput)
			os.Exit(1)
		} else {
			// Add more known error checks if needed
			fmt.Fprintf(os.Stderr, "\nPost-installation completed, but confirmation message 'password updated successfully' not found.\n")
			log.Printf("Final output:\n%s", finalOutput)
			// Decide if this should be a hard failure (os.Exit(1)) or just a warning
			os.Exit(1) // Treat as failure for now
		}
	},
}

// Removed runUbuntuPasswordChange function as logic is moved to pkg/node
// Removed readUntil and getLastLines as they are in pkg/node

func init() {
	rootCmd.AddCommand(postInstallUbuntuCmd)

	postInstallUbuntuCmd.Flags().StringVar(&postInstallNodeIP, "node-ip", "", "IP address of the target node (required)")
	postInstallUbuntuCmd.Flags().StringVar(&postInstallInitialUser, "initial-user", "ubuntu", "Initial username")
	postInstallUbuntuCmd.Flags().StringVar(&postInstallInitialPass, "initial-password", "ubuntu", "Initial password")
	postInstallUbuntuCmd.Flags().StringVar(&postInstallNewPass, "new-password", "", "New password to set (required)")

	if err := postInstallUbuntuCmd.MarkFlagRequired("node-ip"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'node-ip' as required: %v\n", err)
		os.Exit(1)
	}
	if err := postInstallUbuntuCmd.MarkFlagRequired("new-password"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'new-password' as required: %v\n", err)
		os.Exit(1)
	}
}
