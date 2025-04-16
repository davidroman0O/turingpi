package cmd

import (
	"fmt"
	"os"

	"github.com/davidroman0O/turingpi/pkg/node"
	"github.com/spf13/cobra"
)

var (
	nodeExecIP       string
	nodeExecUser     string
	nodeExecPassword string
	nodeExecCommand  string
)

// nodeExecCmd represents the node exec command
var nodeExecCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execute a non-interactive command on a compute node via SSH",
	Long: `Connects directly to the specified compute node using its IP address 
and executes a given shell command. 

Requires the node IP, command, and credentials (user/password). 
Stdout and stderr from the remote command will be printed locally.`,
	Run: func(cmd *cobra.Command, args []string) {
		// --- Input Validation ---
		if nodeExecIP == "" {
			fmt.Fprintln(os.Stderr, "Error: --node-ip is required.")
			os.Exit(1)
		}
		if nodeExecCommand == "" {
			fmt.Fprintln(os.Stderr, "Error: --command is required.")
			os.Exit(1)
		}
		if nodeExecUser == "" {
			fmt.Fprintln(os.Stderr, "Error: --user is required.")
			os.Exit(1)
		}
		if nodeExecPassword == "" {
			// Consider adding interactive prompt later if password is empty
			fmt.Fprintln(os.Stderr, "Error: --password is required.")
			os.Exit(1)
		}

		fmt.Printf("Executing command on node %s...\n", nodeExecIP)

		stdout, stderr, err := node.ExecuteCommand(
			nodeExecIP,
			nodeExecUser,
			nodeExecPassword,
			nodeExecCommand,
		)

		// Print stdout/stderr regardless of error for context
		if stdout != "" {
			fmt.Println("--- stdout --- ")
			fmt.Print(stdout)
			fmt.Println("--------------")
		}
		if stderr != "" {
			fmt.Fprintln(os.Stderr, "--- stderr --- ")
			fmt.Fprint(os.Stderr, stderr)
			fmt.Fprintln(os.Stderr, "--------------")
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "\nCommand execution failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\nCommand executed successfully.")
	},
}

// We need a parent 'node' command for organization
var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Manage individual compute nodes directly",
	Long:  `Provides commands to interact directly with compute nodes via SSH, such as executing commands or copying files.`,
}

func init() {
	// Add nodeCmd to root, and nodeExecCmd to nodeCmd
	rootCmd.AddCommand(nodeCmd)
	nodeCmd.AddCommand(nodeExecCmd)

	nodeExecCmd.Flags().StringVar(&nodeExecIP, "node-ip", "", "IP address of the target node (required)")
	nodeExecCmd.Flags().StringVarP(&nodeExecUser, "user", "u", "", "Username for node SSH connection (required)")
	nodeExecCmd.Flags().StringVarP(&nodeExecPassword, "password", "p", "", "Password for node SSH connection (required)")
	nodeExecCmd.Flags().StringVarP(&nodeExecCommand, "command", "c", "", "Command to execute on the node (required)")

	if err := nodeExecCmd.MarkFlagRequired("node-ip"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'node-ip' as required: %v\n", err)
		os.Exit(1)
	}
	if err := nodeExecCmd.MarkFlagRequired("user"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'user' as required: %v\n", err)
		os.Exit(1)
	}
	if err := nodeExecCmd.MarkFlagRequired("password"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'password' as required: %v\n", err)
		os.Exit(1)
	}
	if err := nodeExecCmd.MarkFlagRequired("command"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'command' as required: %v\n", err)
		os.Exit(1)
	}
}
