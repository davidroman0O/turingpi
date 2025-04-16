/*
Copyright © 2025 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"github.com/spf13/cobra"
)

// Variable to store the node ID flag value for power commands
var nodeID int

// powerCmd represents the power command
var powerCmd = &cobra.Command{
	Use:   "power",
	Short: "Manage power state (on, off, reset) for compute nodes",
	Long: `Provides subcommands to control the power state of individual compute nodes 
(1-4) connected to the Turing Pi 2 BMC. 

Requires specifying the target node using the --node flag for actions like 'on', 'off', or 'reset'.`,
	// No Run function, as this is a parent command
}

func init() {
	rootCmd.AddCommand(powerCmd)

	// Add persistent flag --node to the power command, required by subcommands
	powerCmd.PersistentFlags().IntVarP(&nodeID, "node", "n", 0, "Target node number (1-4) (required for on/off/reset)")
	// Mark the node flag as required for the subcommands (will be enforced by subcommands)
	// We don't mark it required here directly, as the base 'power' command itself doesn't need it.
	// Subcommands will check if nodeID is valid (1-4).

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// powerCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
