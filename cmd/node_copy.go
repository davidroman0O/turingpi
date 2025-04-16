package cmd

import (
	"fmt"
	"os"

	"github.com/davidroman0O/turingpi/pkg/node"
	"github.com/spf13/cobra"
)

var (
	nodeCopyIP         string
	nodeCopyUser       string
	nodeCopyPassword   string
	nodeCopySource     string
	nodeCopyDest       string
	nodeCopyFromRemote bool
	// nodeCopyRecursive bool // Future enhancement
)

// nodeCopyCmd represents the node copy command
var nodeCopyCmd = &cobra.Command{
	Use:   "copy",
	Short: "Copy files between the local machine and a compute node using SFTP",
	Long: `Connects directly to the specified compute node using its IP address 
and copies a file either to or from the node.

Requires node IP, credentials, source path, and destination path. 
Use --from-remote to copy from the node to the local machine.`,
	Run: func(cmd *cobra.Command, args []string) {
		// --- Input Validation ---
		if nodeCopyIP == "" {
			fmt.Fprintln(os.Stderr, "Error: --node-ip is required.")
			os.Exit(1)
		}
		if nodeCopySource == "" {
			fmt.Fprintln(os.Stderr, "Error: --source is required.")
			os.Exit(1)
		}
		if nodeCopyDest == "" {
			fmt.Fprintln(os.Stderr, "Error: --dest is required.")
			os.Exit(1)
		}
		if nodeCopyUser == "" {
			fmt.Fprintln(os.Stderr, "Error: --user is required.")
			os.Exit(1)
		}
		if nodeCopyPassword == "" {
			fmt.Fprintln(os.Stderr, "Error: --password is required.")
			os.Exit(1)
		}

		toRemote := !nodeCopyFromRemote
		localPath := nodeCopySource
		remotePath := nodeCopyDest
		if !toRemote {
			localPath = nodeCopyDest
			remotePath = nodeCopySource
		}

		fmt.Printf("Copying file on node %s...\n  Direction: %s\n  Source: %s\n  Destination: %s\n",
			nodeCopyIP,
			map[bool]string{true: "Local -> Remote", false: "Remote -> Local"}[toRemote],
			nodeCopySource, // Always show user-provided source
			nodeCopyDest,   // Always show user-provided destination
		)

		err := node.CopyFile(
			nodeCopyIP,
			nodeCopyUser,
			nodeCopyPassword,
			localPath,  // Actual local path for the function
			remotePath, // Actual remote path for the function
			toRemote,
		)

		if err != nil {
			fmt.Fprintf(os.Stderr, "\nFile copy failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\nFile copy completed successfully.")
	},
}

func init() {
	// Add nodeCopyCmd to the existing nodeCmd
	nodeCmd.AddCommand(nodeCopyCmd)

	nodeCopyCmd.Flags().StringVar(&nodeCopyIP, "node-ip", "", "IP address of the target node (required)")
	nodeCopyCmd.Flags().StringVarP(&nodeCopyUser, "user", "u", "", "Username for node SSH connection (required)")
	nodeCopyCmd.Flags().StringVarP(&nodeCopyPassword, "password", "p", "", "Password for node SSH connection (required)")
	nodeCopyCmd.Flags().StringVar(&nodeCopySource, "source", "", "Source file path (local or remote) (required)")
	nodeCopyCmd.Flags().StringVar(&nodeCopyDest, "dest", "", "Destination file path (local or remote) (required)")
	nodeCopyCmd.Flags().BoolVar(&nodeCopyFromRemote, "from-remote", false, "Copy from remote node to local machine (default: local to remote)")
	// nodeCopyCmd.Flags().BoolVarP(&nodeCopyRecursive, "recursive", "r", false, "Recursively copy directories (TODO)")

	// Mark required flags
	_ = nodeCopyCmd.MarkFlagRequired("node-ip")
	_ = nodeCopyCmd.MarkFlagRequired("user")
	_ = nodeCopyCmd.MarkFlagRequired("password")
	_ = nodeCopyCmd.MarkFlagRequired("source")
	_ = nodeCopyCmd.MarkFlagRequired("dest")
}
