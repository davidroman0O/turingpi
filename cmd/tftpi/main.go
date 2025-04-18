// Package main implements the tftpi CLI tool
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidroman0O/turingpi/pkg/tpi"
	"github.com/davidroman0O/turingpi/pkg/tpi/state"
	"github.com/spf13/cobra"
)

var (
	rootCmd = &cobra.Command{
		Use:   "tftpi",
		Short: "Touch-Free Turing Pi - A tool for automating Turing Pi deployments",
		Long: `Touch-Free Turing Pi (tftpi) is a command-line tool for automating
the deployment of operating systems on Turing Pi compute modules.
It supports image preparation, OS installation, and post-installation configuration.`,
		SilenceUsage: true,
	}

	// Global flags
	configFile  string
	cacheDir    string
	verboseMode bool

	// State manager instance
	stateManager state.StateManager
)

func init() {
	cobra.OnInitialize(initState)

	// Global flags
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to config file")
	rootCmd.PersistentFlags().StringVar(&cacheDir, "cache-dir", getDefaultCacheDir(), "Path to cache directory")
	rootCmd.PersistentFlags().BoolVarP(&verboseMode, "verbose", "v", false, "Enable verbose output")

	// Add commands
	rootCmd.AddCommand(newStatusCommand())
	rootCmd.AddCommand(newPrepareCommand())
	rootCmd.AddCommand(newInstallCommand())
	rootCmd.AddCommand(newConfigureCommand())
}

func initState() {
	// Ensure cache directory exists
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating cache directory: %v\n", err)
		os.Exit(1)
	}

	// Initialize state manager
	var err error
	stateFilePath := filepath.Join(cacheDir, "tftpi_state.json")
	stateManager, err = state.NewFileStateManager(stateFilePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing state manager: %v\n", err)
		os.Exit(1)
	}
}

func getDefaultCacheDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/tftpi"
	}
	return filepath.Join(homeDir, ".tftpi")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// Status command
func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [nodeID]",
		Short: "Show status of nodes",
		Long:  "Show the current status of all nodes or a specific node",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				// Show status for specific node
				nodeID, err := parseNodeID(args[0])
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error: %v\n", err)
					return
				}
				showNodeStatus(nodeID)
			} else {
				// Show status for all nodes
				showAllNodesStatus()
			}
		},
	}
	return cmd
}

// Prepare command
func newPrepareCommand() *cobra.Command {
	var (
		baseImagePath string
		ipAddress     string
		hostname      string
		outputDir     string
	)

	cmd := &cobra.Command{
		Use:   "prepare-image [nodeID]",
		Short: "Prepare an OS image for a node",
		Long:  "Customize an OS image with network configuration for a specific node",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			nodeID, err := parseNodeID(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}

			// TODO: Implement image preparation logic
			fmt.Printf("Preparing image for node %d...\n", nodeID)
		},
	}

	cmd.Flags().StringVarP(&baseImagePath, "image", "i", "", "Path to base OS image (.img.xz)")
	cmd.Flags().StringVar(&ipAddress, "ip", "", "IP address with CIDR (e.g., 192.168.1.100/24)")
	cmd.Flags().StringVar(&hostname, "hostname", "", "Hostname for the node")
	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Directory to store the prepared image")

	cmd.MarkFlagRequired("image")
	cmd.MarkFlagRequired("ip")

	return cmd
}

// Install command
func newInstallCommand() *cobra.Command {
	var (
		imagePath       string
		initialPassword string
	)

	cmd := &cobra.Command{
		Use:   "install-os [nodeID]",
		Short: "Install OS on a node",
		Long:  "Install a prepared OS image onto a specific node",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			nodeID, err := parseNodeID(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}

			// TODO: Implement OS installation logic
			fmt.Printf("Installing OS on node %d...\n", nodeID)
		},
	}

	cmd.Flags().StringVarP(&imagePath, "image", "i", "", "Path to prepared OS image (.img.xz)")
	cmd.Flags().StringVar(&initialPassword, "password", "ubuntu", "Initial user password")

	cmd.MarkFlagRequired("image")

	return cmd
}

// Configure command
func newConfigureCommand() *cobra.Command {
	var (
		username string
		password string
	)

	cmd := &cobra.Command{
		Use:   "configure [nodeID]",
		Short: "Configure a node",
		Long:  "Perform post-installation configuration on a node",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			nodeID, err := parseNodeID(args[0])
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				return
			}

			// TODO: Implement configuration logic
			fmt.Printf("Configuring node %d...\n", nodeID)
		},
	}

	cmd.Flags().StringVarP(&username, "user", "u", "ubuntu", "Username for SSH connection")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Password for SSH connection")

	cmd.MarkFlagRequired("password")

	return cmd
}

// Helper functions

// parseNodeID converts a string to a NodeID
func parseNodeID(s string) (tpi.NodeID, error) {
	var nodeID int
	_, err := fmt.Sscanf(s, "%d", &nodeID)
	if err != nil {
		return 0, fmt.Errorf("invalid node ID: %s", s)
	}
	if nodeID < 1 || nodeID > 4 {
		return 0, fmt.Errorf("node ID must be between 1 and 4")
	}
	return tpi.NodeID(nodeID), nil
}

// showNodeStatus displays the status of a specific node
func showNodeStatus(nodeID tpi.NodeID) {
	nodeState, err := stateManager.GetNodeState(nodeID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving state for node %d: %v\n", nodeID, err)
		return
	}

	if nodeState == nil {
		fmt.Printf("No state information available for node %d\n", nodeID)
		return
	}

	fmt.Printf("Node: %d\n", nodeState.NodeID)
	fmt.Printf("Board: %s\n", nodeState.BoardType)
	fmt.Printf("OS: %s %s\n", nodeState.OSType, nodeState.OSVersion)
	fmt.Printf("IP Address: %s\n", nodeState.IPAddress)
	fmt.Printf("Hostname: %s\n", nodeState.Hostname)
	fmt.Printf("\n")

	fmt.Printf("Last Operation: %s\n", nodeState.LastOperation)
	fmt.Printf("Last Operation Time: %s\n", nodeState.LastOperationTime.Format("2006-01-02 15:04:05"))

	if nodeState.LastError != "" {
		fmt.Printf("Last Error: %s\n", nodeState.LastError)
	}

	fmt.Printf("\n")

	if nodeState.LastImagePath != "" {
		fmt.Printf("Image Information:\n")
		fmt.Printf("  Path: %s\n", nodeState.LastImagePath)
		fmt.Printf("  Prepared: %s\n", nodeState.LastImageTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("\n")
	}

	if !nodeState.LastInstallTime.IsZero() {
		fmt.Printf("OS Installation:\n")
		fmt.Printf("  Installed: %s\n", nodeState.LastInstallTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("\n")
	}

	if !nodeState.LastConfigTime.IsZero() {
		fmt.Printf("Configuration:\n")
		fmt.Printf("  Configured: %s\n", nodeState.LastConfigTime.Format("2006-01-02 15:04:05"))
		fmt.Printf("\n")
	}
}

// showAllNodesStatus displays the status of all nodes
func showAllNodesStatus() {
	nodes, err := stateManager.ListNodeStates()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error retrieving node states: %v\n", err)
		return
	}

	if len(nodes) == 0 {
		fmt.Println("No node state information available")
		return
	}

	// Print table header
	fmt.Printf("%-5s %-10s %-10s %-20s %-10s %-20s %-15s\n",
		"NODE", "BOARD", "OS", "LAST OPERATION", "RESULT", "LAST TIME", "IP ADDRESS")
	fmt.Printf("%-5s %-10s %-10s %-20s %-10s %-20s %-15s\n",
		"----", "-----", "--", "-------------", "------", "--------", "----------")

	// Print node info
	for _, node := range nodes {
		result := "Success"
		if node.LastError != "" {
			result = "Failed"
		}

		fmt.Printf("%-5d %-10s %-10s %-20s %-10s %-20s %-15s\n",
			node.NodeID,
			node.BoardType,
			node.OSType,
			node.LastOperation,
			result,
			node.LastOperationTime.Format("2006-01-02 15:04"),
			node.IPAddress)
	}
}
