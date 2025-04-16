package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// powerOffCmd represents the power off command
var powerOffCmd = &cobra.Command{
	Use:   "off",
	Short: "Power off (hard shutdown) a specific compute node",
	Long:  `Cuts power to the specified compute node (1-4). This is a hard shutdown. Requires the --node flag.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Validate node ID
		if nodeID < 1 || nodeID > 4 {
			fmt.Fprintf(os.Stderr, "Error: Invalid node ID %d. Please specify a node between 1 and 4 using --node.\n", nodeID)
			cmd.Usage()
			os.Exit(1)
		}

		// Construct arguments for the tpi command
		cmdArgs := []string{
			"power", "off", // Changed action to "off"
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
		fmt.Printf("Node %d power off command sent.\n", nodeID)
	},
}

func init() {
	powerCmd.AddCommand(powerOffCmd)
	// No specific flags for 'off', uses persistent flags from parent 'power' command.
}
