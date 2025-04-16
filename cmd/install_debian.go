package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/davidroman0O/turingpi/pkg/state" // Correct import path

	"github.com/davidroman0O/firm-go"
	"github.com/spf13/cobra"
)

// Variables to store flag values for install-debian command
var (
	debianNodeID    int
	debianImagePath string
)

// Helper function to execute tpi commands
// Returns an error if the command fails
func executeTpiCommand(args ...string) error {
	baseArgs := []string{
		"--host", bmcHost, // Use persistent flag value
		"--user", bmcUser, // Use persistent flag value
	}
	if bmcPassword != "" {
		baseArgs = append(baseArgs, "--password", bmcPassword)
	}
	fullArgs := append(args, baseArgs...)

	fmt.Printf("Executing: tpi %v\n", fullArgs)
	cmd := exec.Command("tpi", fullArgs...)
	cmd.Stdout = os.Stdout // Show command output directly
	cmd.Stderr = os.Stderr // Show command errors directly
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to execute tpi %v: %w", args, err)
	}
	return nil
}

// installDebianCmd represents the install-debian command
var installDebianCmd = &cobra.Command{
	Use:   "install-debian",
	Short: "Install Debian OS onto a discovered node",
	Long: `Flashes a specified Debian image file onto the target compute node (1-4)
and then power-cycles the node.

This command assumes the node's IP has already been discovered using the 'discover' command.
It reads the IP from the state file (~/.config/turingpi-cli/state.json).

Sequence:
1. Load node IP from state file.
2. Flash image ('tpi flash ...')
3. Power off node ('tpi power off ...')
4. Power on node ('tpi power on ...')

Requires:
- Node ID (--node)
- Local path to Debian image (--image-path)
- The node must have been previously discovered.`,
	Run: func(cmd *cobra.Command, args []string) {
		// --- Input Validation ---
		if debianNodeID < 1 || debianNodeID > 4 {
			fmt.Fprintf(os.Stderr, "Error: Invalid node ID %d. Use --node with a value between 1 and 4.\n", debianNodeID)
			cmd.Usage()
			os.Exit(1)
		}
		if debianImagePath == "" {
			fmt.Fprint(os.Stderr, "Error: Image file path cannot be empty. Use --image-path.\n")
			cmd.Usage()
			os.Exit(1)
		}
		if _, err := os.Stat(debianImagePath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Image file not found at: %s\n", debianImagePath)
			os.Exit(1)
		}
		absImagePath, _ := filepath.Abs(debianImagePath)
		fmt.Printf("Using image: %s for node %d\n", absImagePath, debianNodeID)

		// --- Load State ---
		currentState, err := state.LoadState()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading state file: %v\nConsider running 'discover' first.\n", err)
			os.Exit(1)
		}
		nodeInfo, exists := currentState.Nodes[debianNodeID]
		if !exists || nodeInfo.Status != "discovered" || nodeInfo.IPAddress == "" {
			fmt.Fprintf(os.Stderr, "Error: Node %d not found in state file or not marked as 'discovered'.\nPlease run 'discover --node %d' first.\n", debianNodeID, debianNodeID)
			os.Exit(1)
		}
		fmt.Printf("Found discovered IP %s for node %d.\n", nodeInfo.IPAddress, debianNodeID)

		// --- firm-go Setup ---
		cleanup, wait := firm.Root(func(owner *firm.Owner) firm.CleanUp {
			log.Println("Setting up firm-go root for installation...")
			// State signal: idle -> flashing -> powering_off -> powering_on -> done/error
			stateSignal := firm.Signal(owner, "idle")
			errorSignal := firm.Signal[error](owner, nil)

			// Effect to manage the installation sequence
			firm.Effect(owner, func() firm.CleanUp {
				currentInternalState := stateSignal.Get()
				nodeStr := fmt.Sprintf("%d", debianNodeID)
				log.Printf("Install effect triggered. Current state: %s\n", currentInternalState)

				switch currentInternalState {
				case "flashing":
					log.Println("Processing 'flashing' state...")
					fmt.Println("Starting Debian OS flash process (this may take several minutes)...")
					owner.TrackPendingOp()
					go func() {
						defer owner.CompletePendingOp()
						// Update node status in state file before starting flash
						_ = state.UpdateNodeState(debianNodeID, func(s *state.NodeStatus) { s.Status = "installing"; s.OS = "debian"; s.Error = "" })
						_, err := executeTpiCommandWithOutput("flash", "--node", nodeStr, "--image-path", absImagePath)
						firm.Batch(owner, func() {
							if err != nil {
								fmt.Fprintf(os.Stderr, "Flashing failed: %v\n", err)
								errorSignal.Set(err)
								_ = state.UpdateNodeState(debianNodeID, func(s *state.NodeStatus) { s.Status = "install_failed"; s.Error = err.Error() })
								stateSignal.Set("error")
							} else {
								fmt.Println("Flashing completed successfully.")
								stateSignal.Set("powering_off")
							}
						})
					}()

				case "powering_off":
					log.Println("Processing 'powering_off' state...")
					fmt.Println("Powering off node before rebooting...")
					time.Sleep(2 * time.Second)
					owner.TrackPendingOp()
					go func() {
						defer owner.CompletePendingOp()
						_, err := executeTpiCommandWithOutput("power", "off", "--node", nodeStr)
						firm.Batch(owner, func() {
							if err != nil {
								fmt.Fprintf(os.Stderr, "Power off failed: %v\n", err)
								errorSignal.Set(err)
								_ = state.UpdateNodeState(debianNodeID, func(s *state.NodeStatus) { s.Status = "install_failed"; s.Error = err.Error() })
								stateSignal.Set("error")
							} else {
								stateSignal.Set("powering_on")
							}
						})
					}()

				case "powering_on":
					log.Println("Processing 'powering_on' state...")
					fmt.Println("Powering on node with new OS...")
					time.Sleep(2 * time.Second)
					owner.TrackPendingOp()
					go func() {
						defer owner.CompletePendingOp()
						_, err := executeTpiCommandWithOutput("power", "on", "--node", nodeStr)
						firm.Batch(owner, func() {
							if err != nil {
								fmt.Fprintf(os.Stderr, "Power on failed: %v\n", err)
								errorSignal.Set(err)
								_ = state.UpdateNodeState(debianNodeID, func(s *state.NodeStatus) { s.Status = "install_failed"; s.Error = err.Error() })
								stateSignal.Set("error")
							} else {
								_ = state.UpdateNodeState(debianNodeID, func(s *state.NodeStatus) { s.Status = "installed"; s.Error = "" })
								stateSignal.Set("done")
							}
						})
					}()

				case "done":
					log.Println("Processing 'done' state...")
					fmt.Printf("Debian installation process for node %d completed successfully!\n", debianNodeID)
					fmt.Printf("Node should be booting Debian. You might be able to SSH using IP %s (check state file or run 'status' command later).\n", nodeInfo.IPAddress)
				case "error":
					log.Println("Processing 'error' state...")
					fmt.Fprintf(os.Stderr, "Debian installation process failed for node %d. Last error: %v\n", debianNodeID, errorSignal.Peek())
				default:
					log.Printf("Entered unknown state: %s\n", currentInternalState)
				}

				return func() { /* No specific cleanup needed per state change */ }
			}, []firm.Reactive{stateSignal})

			// Initial trigger inside Root scope
			log.Println("Triggering initial state change to 'flashing' inside Root...")
			stateSignal.Set("flashing")

			log.Println("Finished setting up firm-go root for installation.")
			return func() { log.Println("Running main firm-go cleanup for installation.") }
		})

		fmt.Println("Waiting for installation process to complete...")
		wait()
		cleanup()
		fmt.Println("Installation command finished.")
	},
}

func init() {
	rootCmd.AddCommand(installDebianCmd)

	installDebianCmd.Flags().IntVarP(&debianNodeID, "node", "n", 0, "Target node number (1-4) to install Debian on (required)")
	installDebianCmd.Flags().StringVarP(&debianImagePath, "image-path", "i", "", "Local path to the Debian .img/.raw image file (required)")

	if err := installDebianCmd.MarkFlagRequired("node"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'node' as required: %v\n", err)
		os.Exit(1)
	}
	if err := installDebianCmd.MarkFlagRequired("image-path"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'image-path' as required: %v\n", err)
		os.Exit(1)
	}
}
