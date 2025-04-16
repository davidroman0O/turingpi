package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// powerResetCmd represents the power reset command
var powerResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset (power cycle) a specific compute node",
	Long:  `Performs a hard reset (power cycle) on the specified compute node (1-4). Requires the --node flag.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Validate node ID
		if nodeID < 1 || nodeID > 4 {
			fmt.Fprintf(os.Stderr, "Error: Invalid node ID %d. Please specify a node between 1 and 4 using --node.\n", nodeID)
			cmd.Usage()
			os.Exit(1)
		}

		// Construct arguments for the tpi command
		cmdArgs := []string{
			"power", "reset", // Changed action to "reset"
			"--node", fmt.Sprintf("%d", nodeID),
			"--host", bmcHost,
			"--user", bmcUser,
		}
		if bmcPassword != "" {
			cmdArgs = append(cmdArgs, "--password", bmcPassword)
		}

		// Prepare and execute the command
		execCmd := exec.Command("tpi", cmdArgs...)
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr

		fmt.Println("Executing: tpi", cmdArgs)
		err := execCmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing tpi command: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Node %d power reset command sent.\n", nodeID)
	},
}

func init() {
	powerCmd.AddCommand(powerResetCmd)
	// No specific flags for 'reset', uses persistent flags from parent 'power' command.
}
