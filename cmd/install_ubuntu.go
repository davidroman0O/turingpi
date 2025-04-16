package cmd

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/state" // Correct import path

	"github.com/davidroman0O/firm-go"
	"github.com/pkg/sftp" // Need this for SCP
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

// Variables to store flag values for install-ubuntu command
var (
	ubuntuNodeID    int
	ubuntuImagePath string
)

// --- SSH/SFTP Helper Functions ---

func getSSHClientConfig() *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User: bmcUser, // Use global flag value
		Auth: []ssh.AuthMethod{
			ssh.Password(bmcPassword), // Use global flag value
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         20 * time.Second, // Increased timeout
	}
}

// executeSSHCommand executes a command on the BMC and returns output/error
func executeSSHCommand(command string) (string, string, error) {
	config := getSSHClientConfig()
	addr := fmt.Sprintf("%s:22", bmcHost)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", "", fmt.Errorf("ssh dial failed: %w", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("ssh session failed: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	log.Printf("[SSH EXEC] %s", command)
	err = session.Run(command) // Use Run for commands that terminate

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if stdoutStr != "" {
		log.Printf("[SSH STDOUT]:\n%s", stdoutStr)
	}
	if stderrStr != "" {
		log.Printf("[SSH STDERR]:\n%s", stderrStr)
	}

	if err != nil {
		// Combine error and stderr for better context
		return stdoutStr, stderrStr, fmt.Errorf("command '%s' failed: %w. Stderr: %s", command, err, stderrStr)
	}

	log.Printf("[SSH EXEC] Command '%s' completed successfully.", command)
	return stdoutStr, stderrStr, nil
}

// checkRemoteFileExists checks if a file exists on the remote host using ls
func checkRemoteFileExists(remotePath string) (bool, error) {
	// Use 'ls' and check exit code / stderr. ls exits 0 if file exists,
	// non-zero (often 1 or 2) if not found.
	// We redirect stderr to stdout to capture potential "No such file" messages in one place.
	cmdStr := fmt.Sprintf("ls %s 2>&1", remotePath)
	stdout, _, err := executeSSHCommand(cmdStr)

	if err == nil {
		// Command succeeded (exit code 0), file exists
		log.Printf("[SSH LS] File %s exists.", remotePath)
		return true, nil
	}

	// Command failed, check if it's because the file doesn't exist
	// Note: Exact error message might vary. Common indicators:
	if strings.Contains(stdout, "No such file or directory") ||
		strings.Contains(stdout, "cannot access") ||
		(err != nil && strings.Contains(err.Error(), "Process exited with status")) { // Check common error patterns

		// Determine if it's specifically a 'not found' error based on exit code or message
		if exitErr, ok := err.(*ssh.ExitError); ok {
			// Common exit codes for ls not found: 1 or 2 depending on implementation
			if exitErr.ExitStatus() == 1 || exitErr.ExitStatus() == 2 {
				log.Printf("[SSH LS] File %s does not exist (ls exit code %d).", remotePath, exitErr.ExitStatus())
				return false, nil // File not found is not an execution error
			}
		}
		// If we couldn't confirm exit code, rely on stdout message check
		if strings.Contains(stdout, "No such file or directory") {
			log.Printf("[SSH LS] File %s does not exist (stderr message).", remotePath)
			return false, nil
		}
	}

	// If the error wasn't clearly a "not found" error, report it as a failure
	log.Printf("[SSH LS] Error checking file %s: %v. Output: %s", remotePath, err, stdout)
	return false, fmt.Errorf("failed to check remote file %s: %w. Output: %s", remotePath, err, stdout)
}

// scpUploadFile uploads a local file to a remote path using SFTP
func scpUploadFile(localPath, remotePath string) error {
	config := getSSHClientConfig()
	addr := fmt.Sprintf("%s:22", bmcHost)

	log.Printf("[SCP UPLOAD] Connecting to %s...", addr)
	conn, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("ssh dial for sftp failed: %w", err)
	}
	defer conn.Close()

	log.Println("[SCP UPLOAD] Creating SFTP client...")
	client, err := sftp.NewClient(conn)
	if err != nil {
		return fmt.Errorf("sftp client creation failed: %w", err)
	}
	defer client.Close()

	remoteDir := filepath.Dir(remotePath)
	log.Printf("[SCP UPLOAD] Ensuring remote directory exists: %s", remoteDir)
	// MkdirAll creates parent directories as needed.
	if err := client.MkdirAll(remoteDir); err != nil {
		// Ignore error if directory already exists, handle others
		// Stat returns an error if path doesn't exist
		if _, statErr := client.Stat(remoteDir); os.IsNotExist(statErr) {
			return fmt.Errorf("failed to create remote directory %s: %w", remoteDir, err)
		}
		// If Stat succeeded, directory exists, ignore MkdirAll error
		log.Printf("[SCP UPLOAD] Remote directory %s likely already exists.", remoteDir)
	} else {
		log.Printf("[SCP UPLOAD] Created remote directory %s.", remoteDir)
	}

	log.Printf("[SCP UPLOAD] Opening local file: %s", localPath)
	srcFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file %s: %w", localPath, err)
	}
	defer srcFile.Close()

	log.Printf("[SCP UPLOAD] Creating remote file: %s", remotePath)
	dstFile, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s: %w", remotePath, err)
	}
	defer dstFile.Close()

	log.Printf("[SCP UPLOAD] Copying data...")
	bytesCopied, err := io.Copy(dstFile, srcFile)
	if err != nil {
		// Attempt to remove partially uploaded file on error
		_ = client.Remove(remotePath)
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	log.Printf("[SCP UPLOAD] Successfully copied %d bytes to %s", bytesCopied, remotePath)
	return nil
}

// --- installUbuntuCmd ---
var installUbuntuCmd = &cobra.Command{
	Use:   "install-ubuntu",
	Short: "Install Ubuntu OS onto a target node",
	Long: `Prepares the BMC by transferring and decompressing a prepared Ubuntu image,
then flashes it onto the target compute node (1-4) and power-cycles the node.

Requires:
- Node ID (--node)
- Local path to the *prepared* Ubuntu image (--image-path, e.g., rk1-node1.img.xz).
  This image should have been created using the 'prepare' command.
- BMC credentials (global flags).

Workflow:
1. Check if uncompressed image exists on BMC in /root/imgs/<node_id>/.
2. If not, transfer the compressed image (.img.xz) to BMC via SCP.
3. Decompress the image on BMC using 'unxz'.
4. Flash the image using 'tpi flash'.
5. Power cycle the node using 'tpi power off/on'.
6. Updates node status in the local state file.`,
	Run: func(cmd *cobra.Command, args []string) {
		// --- Input Validation ---
		if ubuntuNodeID < 1 || ubuntuNodeID > 4 {
			fmt.Fprintf(os.Stderr, "Error: Invalid node ID %d. Use --node with a value between 1 and 4.\n", ubuntuNodeID)
			cmd.Usage()
			os.Exit(1)
		}
		if ubuntuImagePath == "" {
			fmt.Fprint(os.Stderr, "Error: Prepared image file path cannot be empty. Use --image-path.\n")
			cmd.Usage()
			os.Exit(1)
		}
		// Ensure the *local* prepared image exists
		if _, err := os.Stat(ubuntuImagePath); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error: Local prepared image file not found at: %s\n", ubuntuImagePath)
			os.Exit(1)
		}
		// Validate it's an .img.xz file
		if !strings.HasSuffix(strings.ToLower(ubuntuImagePath), ".img.xz") {
			fmt.Fprintf(os.Stderr, "Error: Image path must point to a prepared '.img.xz' file.\n")
			os.Exit(1)
		}

		localAbsImagePath, _ := filepath.Abs(ubuntuImagePath)
		fmt.Printf("Using local prepared image: %s for node %d\n", localAbsImagePath, ubuntuNodeID)

		// Extract filenames
		imageXZName := filepath.Base(localAbsImagePath)
		imageName := strings.TrimSuffix(imageXZName, ".xz") // e.g., rk1-node1.img

		// --- Define Remote Paths ---
		nodeStr := fmt.Sprintf("%d", ubuntuNodeID)
		remoteBaseDir := "/root/imgs"
		remoteNodeDir := filepath.Join(remoteBaseDir, nodeStr) // Use Join for cross-platform compatibility if Go runs elsewhere
		remoteImgPath := filepath.Join(remoteNodeDir, imageName)
		remoteXZPath := filepath.Join(remoteNodeDir, imageXZName)

		// --- firm-go Setup ---
		cleanup, wait := firm.Root(func(owner *firm.Owner) firm.CleanUp {
			log.Println("Setting up firm-go root for Ubuntu installation...")

			// Define states
			const (
				StateIdle           = "idle"
				StateCheckingRemote = "checking_remote"
				StateTransferring   = "transferring"
				StateDecompressing  = "decompressing"
				StateFlashing       = "flashing"
				StatePoweringOff    = "powering_off"
				StatePoweringOn     = "powering_on"
				StateDone           = "done"
				StateError          = "error"
			)

			// State signals
			stateSignal := firm.Signal(owner, StateIdle)
			errorSignal := firm.Signal[error](owner, nil)
			statusMsgSignal := firm.Signal(owner, "Initializing...") // Signal for user feedback

			// --- Effect for State Machine ---
			firm.Effect(owner, func() firm.CleanUp {
				currentState := stateSignal.Get()
				log.Printf("Install effect triggered. Current state: %s\n", currentState)
				statusMsgSignal.Set(fmt.Sprintf("Current step: %s", currentState)) // Update user feedback

				switch currentState {
				case StateCheckingRemote:
					log.Printf("Checking for existing image on BMC: %s\n", remoteImgPath)
					owner.TrackPendingOp()
					go func() {
						defer owner.CompletePendingOp()
						exists, err := checkRemoteFileExists(remoteImgPath)
						firm.Batch(owner, func() {
							if err != nil {
								log.Printf("Error checking remote file: %v", err)
								errorSignal.Set(fmt.Errorf("failed to check BMC file status: %w", err))
								stateSignal.Set(StateError)
							} else if exists {
								log.Printf("Uncompressed image %s found on BMC. Skipping transfer and decompression.", remoteImgPath)
								statusMsgSignal.Set("Image found on BMC.")
								stateSignal.Set(StateFlashing) // Skip to flashing
							} else {
								log.Printf("Uncompressed image %s not found on BMC.", remoteImgPath)
								statusMsgSignal.Set("Image not found on BMC, preparing transfer...")
								stateSignal.Set(StateTransferring) // Need to transfer
							}
						})
					}()

				case StateTransferring:
					log.Printf("Transferring %s to BMC:%s\n", localAbsImagePath, remoteXZPath)
					owner.TrackPendingOp()
					go func() {
						defer owner.CompletePendingOp()
						err := scpUploadFile(localAbsImagePath, remoteXZPath)
						firm.Batch(owner, func() {
							if err != nil {
								log.Printf("SCP upload failed: %v", err)
								errorSignal.Set(fmt.Errorf("failed to upload image to BMC: %w", err))
								stateSignal.Set(StateError)
							} else {
								log.Println("SCP upload successful.")
								statusMsgSignal.Set("Image transferred.")
								stateSignal.Set(StateDecompressing)
							}
						})
					}()

				case StateDecompressing:
					log.Printf("Decompressing image on BMC: %s\n", remoteXZPath)
					owner.TrackPendingOp()
					go func() {
						defer owner.CompletePendingOp()
						// Use -f to force overwrite if .img exists from prior failed attempt
						cmdStr := fmt.Sprintf("unxz -f %s", remoteXZPath)
						_, _, err := executeSSHCommand(cmdStr)
						firm.Batch(owner, func() {
							if err != nil {
								log.Printf("Decompression failed on BMC: %v", err)
								errorSignal.Set(fmt.Errorf("failed to decompress image on BMC: %w", err))
								stateSignal.Set(StateError)
							} else {
								log.Println("Decompression successful on BMC.")
								statusMsgSignal.Set("Image decompressed.")
								stateSignal.Set(StateFlashing)
							}
						})
					}()

				case StateFlashing:
					log.Println("Starting flash process using tpi...")
					owner.TrackPendingOp()
					go func() {
						defer owner.CompletePendingOp()
						// Update node status in state file before starting flash
						_ = state.UpdateNodeState(ubuntuNodeID, func(s *state.NodeStatus) { s.Status = "installing"; s.OS = "ubuntu"; s.Error = "" })

						cmdStr := fmt.Sprintf("tpi flash --node %s -i %s", nodeStr, remoteImgPath)
						_, _, err := executeSSHCommand(cmdStr)
						firm.Batch(owner, func() {
							if err != nil {
								log.Printf("Flashing failed: %v", err)
								errorSignal.Set(err)
								_ = state.UpdateNodeState(ubuntuNodeID, func(s *state.NodeStatus) { s.Status = "install_failed"; s.Error = err.Error() })
								stateSignal.Set(StateError)
							} else {
								log.Println("Flashing completed successfully.")
								statusMsgSignal.Set("Flashing complete.")
								stateSignal.Set(StatePoweringOff)
							}
						})
					}()

				case StatePoweringOff:
					log.Println("Powering off node...")
					owner.TrackPendingOp()
					go func() {
						defer owner.CompletePendingOp()
						time.Sleep(2 * time.Second) // Short delay before power off
						cmdStr := fmt.Sprintf("tpi power off --node %s", nodeStr)
						_, _, err := executeSSHCommand(cmdStr)
						firm.Batch(owner, func() {
							if err != nil {
								log.Printf("Power off failed: %v", err)
								// Don't necessarily fail the whole install for power off failure? Maybe just log warning?
								// For now, treat as error.
								errorSignal.Set(fmt.Errorf("power off failed: %w", err))
								_ = state.UpdateNodeState(ubuntuNodeID, func(s *state.NodeStatus) {
									s.Status = "install_failed"
									s.Error = fmt.Sprintf("power off failed: %v", err)
								})
								stateSignal.Set(StateError)
							} else {
								log.Println("Power off successful.")
								statusMsgSignal.Set("Node powered off.")
								stateSignal.Set(StatePoweringOn)
							}
						})
					}()

				case StatePoweringOn:
					log.Println("Powering on node...")
					owner.TrackPendingOp()
					go func() {
						defer owner.CompletePendingOp()
						time.Sleep(2 * time.Second) // Wait after power off
						cmdStr := fmt.Sprintf("tpi power on --node %s", nodeStr)
						_, _, err := executeSSHCommand(cmdStr)
						firm.Batch(owner, func() {
							if err != nil {
								log.Printf("Power on failed: %v", err)
								errorSignal.Set(fmt.Errorf("power on failed: %w", err))
								_ = state.UpdateNodeState(ubuntuNodeID, func(s *state.NodeStatus) {
									s.Status = "install_failed"
									s.Error = fmt.Sprintf("power on failed: %v", err)
								})
								stateSignal.Set(StateError)
							} else {
								log.Println("Power on successful.")
								statusMsgSignal.Set("Node powered on.")
								_ = state.UpdateNodeState(ubuntuNodeID, func(s *state.NodeStatus) { s.Status = "installed"; s.OS = "ubuntu"; s.Error = "" })
								stateSignal.Set(StateDone)
							}
						})
					}()

				case StateDone:
					finalMsg := fmt.Sprintf("Ubuntu installation process for node %d completed successfully!\n", ubuntuNodeID)
					finalMsg += "Node should be booting Ubuntu with the prepared configuration.\n"
					finalMsg += "You might need to run the 'post-install' command for OS-specific first-boot steps (like setting password)."
					statusMsgSignal.Set(finalMsg)
					fmt.Println(finalMsg)
					// No further automatic transitions

				case StateError:
					errMsg := fmt.Sprintf("Ubuntu installation process failed for node %d. Last error: %v\n", ubuntuNodeID, errorSignal.Peek())
					statusMsgSignal.Set(errMsg)
					fmt.Fprint(os.Stderr, errMsg)
					// Update state file one last time if error wasn't already set
					_ = state.UpdateNodeState(ubuntuNodeID, func(s *state.NodeStatus) {
						if s.Status != "install_failed" { // Avoid overwriting specific error if already set
							s.Status = "install_failed"
							if err := errorSignal.Peek(); err != nil {
								s.Error = err.Error()
							} else {
								s.Error = "Unknown installation error"
							}
						}
					})
					// No further automatic transitions

				default:
					log.Printf("Entered unknown state: %s\n", currentState)
					errorSignal.Set(fmt.Errorf("entered unknown state %s", currentState))
					stateSignal.Set(StateError)
				}

				return func() { /* No specific cleanup needed per state change */ }
			}, []firm.Reactive{stateSignal}) // Effect reacts to state changes

			// --- User Feedback Effect ---
			firm.Effect(owner, func() firm.CleanUp {
				fmt.Printf("\rStatus: %s", statusMsgSignal.Get()) // Print status updates
				return nil
			}, []firm.Reactive{statusMsgSignal})

			// --- Initial Trigger ---
			log.Println("Triggering initial state check...")
			stateSignal.Set(StateCheckingRemote) // Start the process

			log.Println("Finished setting up firm-go root for installation.")
			return func() {
				fmt.Println() // Newline after final status message
				log.Println("Running main firm-go cleanup for installation.")
			}
		})

		fmt.Println("Starting Ubuntu installation process...")
		wait() // Wait for all async operations in firm-go scope to complete
		cleanup()
		fmt.Println("Installation command finished.")
	},
}

func init() {
	rootCmd.AddCommand(installUbuntuCmd)

	installUbuntuCmd.Flags().IntVarP(&ubuntuNodeID, "node", "n", 0, "Target node number (1-4) (required)")
	installUbuntuCmd.Flags().StringVarP(&ubuntuImagePath, "image-path", "i", "", "Local path to the prepared Ubuntu '.img.xz' image file (required)")

	if err := installUbuntuCmd.MarkFlagRequired("node"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'node' as required: %v\n", err)
		os.Exit(1)
	}
	if err := installUbuntuCmd.MarkFlagRequired("image-path"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'image-path' as required: %v\n", err)
		os.Exit(1)
	}
}
