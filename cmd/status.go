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

// statusCmd represents the status command
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Get the power status of all compute nodes",
	Long: `Prints the current power status (on/off) for each of the four compute nodes
connected to the Turing Pi 2 BMC.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Construct arguments for the tpi command
		cmdArgs := []string{
			"power", "status",
			"--host", bmcHost,
			"--user", bmcUser,
		}
		// Only add password flag if it's provided
		if bmcPassword != "" {
			cmdArgs = append(cmdArgs, "--password", bmcPassword)
		}

		// Prepare the command
		execCmd := exec.Command("tpi", cmdArgs...)

		// Connect command's output and error streams to the main process's streams
		execCmd.Stdout = os.Stdout
		execCmd.Stderr = os.Stderr

		// Run the command
		fmt.Println("Executing: tpi", cmdArgs)
		err := execCmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error executing tpi command: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)

	// No specific flags for the status command itself, it uses the persistent ones.
}
