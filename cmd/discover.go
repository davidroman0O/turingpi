package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/state" // Correct import path

	"github.com/davidroman0O/firm-go"
	"golang.org/x/crypto/ssh"

	"github.com/spf13/cobra"
)

// Variables to store flag values for discover command
var (
	discoverNodeID int
)

// --- SSH Helper ---
// (executeSSHCommand function as defined previously - connects to BMC)
func executeSSHCommandOnBMC(command string) (string, error) {
	sshConfig := &ssh.ClientConfig{
		User: bmcUser, // Use global flag value
		Auth: []ssh.AuthMethod{
			ssh.Password(bmcPassword), // Use global flag value
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second, // Increased timeout for potentially longer commands
	}

	addr := fmt.Sprintf("%s:22", bmcHost) // Use global flag value
	log.Printf("[BMC SSH] Attempting connection to %s as user %s...", addr, bmcUser)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return "", fmt.Errorf("[BMC SSH] failed to dial %s: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("[BMC SSH] failed to create session: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	log.Printf("[BMC SSH] Executing: %s\n", command)
	err = session.Run(command)
	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	// Log output regardless of error for debugging
	if stdoutStr != "" {
		log.Printf("[BMC SSH stdout]:\n%s", stdoutStr)
	}
	if stderrStr != "" {
		log.Printf("[BMC SSH stderr]:\n%s", stderrStr)
	}

	if err != nil {
		// Combine error and stderr for better context
		return stdoutStr, fmt.Errorf("[BMC SSH] command '%s' failed: %w. Stderr: %s", command, err, stderrStr)
	}

	log.Printf("[BMC SSH] command '%s' executed successfully.", command)
	return stdoutStr, nil
}

// --- TPI Helper ---
// (executeTpiCommandWithOutput as defined previously - runs tpi locally)
func executeTpiCommandWithOutput(args ...string) (string, error) {
	baseArgs := []string{
		"--host", bmcHost,
		"--user", bmcUser,
	}
	if bmcPassword != "" {
		baseArgs = append(baseArgs, "--password", bmcPassword)
	}
	fullArgs := append(args, baseArgs...)

	fmt.Printf("Executing Local TPI: tpi %v\n", fullArgs)
	cmd := exec.Command("tpi", fullArgs...)

	outputBytes, err := cmd.CombinedOutput() // Capture stdout and stderr
	output := string(outputBytes)

	if err != nil {
		fmt.Printf("Local TPI command failed. Output:\n%s\n", output) // Print output on error
		return output, fmt.Errorf("failed to execute local tpi %v: %w", args, err)
	}
	// fmt.Printf("Local TPI command output:\n%s\n", output) // Optional: print output on success too
	return output, nil
}

// discoverCmd represents the discover command implementing the Alpine guide steps
var discoverCmd = &cobra.Command{
	Use:   "discover",
	Short: "Provision Alpine via BMC SSH and discover IP via UART",
	Long: `Connects to the BMC via SSH and executes the steps to partition,
install a base Alpine system, install RK1 specifics, and flash U-Boot
to the target node's eMMC via MSD mode.

After provisioning Alpine, it power cycles the node and polls UART to find
the IP address, saving it to the state file. Uses a firm-go sequential effects approach.

Requires:
- Node ID (--node)
- BMC credentials (global flags)
- Network connectivity for the BMC to download Alpine packages/keys.

WARNING: This will erase the target node's eMMC.`,
	Run: func(cmd *cobra.Command, args []string) {
		// --- Input Validation ---
		if discoverNodeID < 1 || discoverNodeID > 4 {
			fmt.Fprintf(os.Stderr, "Error: Invalid node ID %d. Use --node with a value between 1 and 4.\n", discoverNodeID)
			cmd.Usage()
			os.Exit(1)
		}
		log.Printf("Starting Alpine provisioning and discovery for node %d...", discoverNodeID)
		nodeStr := fmt.Sprintf("%d", discoverNodeID)
		// Assuming /dev/sda corresponds to the node's eMMC when in MSD mode on BMC.
		msdDevice := "/dev/sda" // This might need adjustment
		rootPartition := msdDevice + "2"
		mountPoint := "/mnt/alpine_node" + nodeStr // Unique mount point
		apkToolsDir := "/tmp/apk_node" + nodeStr   // Unique temp dir

		// --- firm-go Setup ---
		var finalError error
		cleanup, wait := firm.Root(func(owner *firm.Owner) firm.CleanUp {
			log.Println("Setting up firm-go root with sequential effects discovery...")

			// --- State Signals ---
			discoveryError := firm.Signal[error](owner, nil) // Central error signal
			processComplete := firm.Signal(owner, false)     // Overall completion

			// Step completion signals
			msdDone := firm.Signal(owner, false)
			partitionDone := firm.Signal(owner, false)
			baseInstallDone := firm.Signal(owner, false)
			rk1InstallDone := firm.Signal(owner, false)
			ubootFlashDone := firm.Signal(owner, false)
			finalizeDone := firm.Signal(owner, false)
			powerCycleDone := firm.Signal(owner, false)
			uartPollDone := firm.Signal(owner, false)    // Indicates polling finished (success or timeout)
			stateUpdateDone := firm.Signal(owner, false) // Final success state

			// UART Polling specific state
			discoveredIP := firm.Signal(owner, "")
			uartTimeout := firm.Signal(owner, false)

			// --- Chain of Effects ---

			// 1. Enable MSD
			firm.Effect(owner, func() firm.CleanUp {
				log.Println("[Step 1/9] Enabling MSD mode...")
				go func() {
					cmdStr := fmt.Sprintf("tpi advanced -n %s msd", nodeStr)
					_, err := executeSSHCommandOnBMC(cmdStr)
					if err != nil {
						log.Printf("Failed to enable MSD: %v", err)
						firm.Batch(owner, func() { discoveryError.Set(err) })
						return
					}
					log.Println("MSD mode enabled. Waiting longer for device...")
					time.Sleep(15 * time.Second)
					firm.Batch(owner, func() { msdDone.Set(true) })
				}()
				return func() {}
			}, nil) // Start immediately

			// 2. Partition Disk
			firm.Effect(owner, func() firm.CleanUp {
				if !msdDone.Get() {
					return func() {}
				} // Wait for previous step
				log.Println("[Step 2/9] Partitioning disk...")
				go func() {
					commands := []string{
						fmt.Sprintf("sgdisk -Z %s", msdDevice),
						fmt.Sprintf("sgdisk -n 1:16384:+2M -c 1:uboot %s", msdDevice),
						fmt.Sprintf("sgdisk -n 2:: -c 2:alpine -A 2:set:2 %s", msdDevice),
						fmt.Sprintf("mkfs.ext4 %s", rootPartition),
						fmt.Sprintf("mkdir -p %s", mountPoint),
						fmt.Sprintf("mount %s %s", rootPartition, mountPoint),
					}
					for _, cmdStr := range commands {
						_, err := executeSSHCommandOnBMC(cmdStr)
						if err != nil {
							log.Printf("Partitioning command failed ('%s'): %v", cmdStr, err)
							firm.Batch(owner, func() { discoveryError.Set(err) })
							return // Stop if any command fails
						}
					}
					log.Println("Partitioning and mounting complete.")
					firm.Batch(owner, func() { partitionDone.Set(true) })
				}()
				return func() {}
			}, []firm.Reactive{msdDone})

			// 3. Install Alpine Base
			firm.Effect(owner, func() firm.CleanUp {
				if !partitionDone.Get() {
					return func() {}
				}
				log.Println("[Step 3/9] Installing Alpine base...")
				go func() {
					repoURL := "https://dl-cdn.alpinelinux.org/alpine/latest-stable"
					apkCmd := fmt.Sprintf("%s/sbin/apk.static -U --allow-untrusted --initdb --no-scripts --arch aarch64 -X %s/main -p %s add alpine-base", apkToolsDir, repoURL, mountPoint)
					repoCmd := fmt.Sprintf("echo '%s/main\n%s/community' > %s/etc/apk/repositories", repoURL, repoURL, mountPoint)
					commands := []string{
						fmt.Sprintf("mkdir -p %s", apkToolsDir),
						fmt.Sprintf("curl -L https://dl-cdn.alpinelinux.org/alpine/v3.19/main/armv7/apk-tools-static-2.14.0-r5.apk | tar -xz -C %s", apkToolsDir),
						apkCmd,
						repoCmd,
					}
					for _, cmdStr := range commands {
						_, err := executeSSHCommandOnBMC(cmdStr)
						if err != nil {
							log.Printf("Alpine base install command failed ('%s'): %v", cmdStr, err)
							firm.Batch(owner, func() { discoveryError.Set(err) })
							return
						}
					}
					log.Println("Alpine base install complete.")
					firm.Batch(owner, func() { baseInstallDone.Set(true) })
				}()
				return func() {}
			}, []firm.Reactive{partitionDone})

			// 4. Install RK1 Packages
			firm.Effect(owner, func() firm.CleanUp {
				if !baseInstallDone.Get() {
					return func() {}
				}
				log.Println("[Step 4/9] Installing RK1 packages...")
				go func() {
					rk1Repo := "https://alpine-rk1.cfs.works/packages/main"
					apkInstallCmd := fmt.Sprintf("%s/sbin/apk.static -p %s -U add --no-scripts linux-firmware-none linux-turing u-boot-turing", apkToolsDir, mountPoint)
					commands := []string{
						fmt.Sprintf("echo '%s' >> %s/etc/apk/repositories", rk1Repo, mountPoint),
						fmt.Sprintf("wget -P %s/etc/apk/keys/ http://alpine-rk1.cfs.works/packages/cfsworks@gmail.com-6549341f.rsa.pub", mountPoint),
						apkInstallCmd,
					}
					for _, cmdStr := range commands {
						_, err := executeSSHCommandOnBMC(cmdStr)
						if err != nil {
							log.Printf("RK1 package install command failed ('%s'): %v", cmdStr, err)
							firm.Batch(owner, func() { discoveryError.Set(err) })
							return
						}
					}
					log.Println("RK1 packages install complete.")
					firm.Batch(owner, func() { rk1InstallDone.Set(true) })
				}()
				return func() {}
			}, []firm.Reactive{baseInstallDone})

			// 5. Flash U-Boot
			firm.Effect(owner, func() firm.CleanUp {
				if !rk1InstallDone.Get() {
					return func() {}
				}
				log.Println("[Step 5/9] Flashing U-Boot...")
				go func() {
					ubootPath := fmt.Sprintf("%s/boot", mountPoint)
					commands := []string{
						fmt.Sprintf("dd if=%s/idbloader.img of=%s bs=512 seek=64", ubootPath, msdDevice),
						fmt.Sprintf("dd if=%s/u-boot.itb of=%s bs=512 seek=16384", ubootPath, msdDevice),
					}
					for _, cmdStr := range commands {
						_, err := executeSSHCommandOnBMC(cmdStr)
						if err != nil {
							log.Printf("U-Boot flash command failed ('%s'): %v", cmdStr, err)
							firm.Batch(owner, func() { discoveryError.Set(err) })
							return
						}
					}
					log.Println("U-Boot flashing complete.")
					firm.Batch(owner, func() { ubootFlashDone.Set(true) })
				}()
				return func() {}
			}, []firm.Reactive{rk1InstallDone})

			// 6. Finalize (Unmount, Cleanup)
			firm.Effect(owner, func() firm.CleanUp {
				if !ubootFlashDone.Get() {
					return func() {}
				}
				log.Println("[Step 6/9] Finalizing (unmount, cleanup)...")
				go func() {
					commands := []string{
						fmt.Sprintf("umount %s", mountPoint),
						fmt.Sprintf("rm -rf %s", mountPoint),  // Optional cleanup
						fmt.Sprintf("rm -rf %s", apkToolsDir), // Optional cleanup
					}
					// Don't set global error for cleanup failures, just log
					for _, cmdStr := range commands {
						_, err := executeSSHCommandOnBMC(cmdStr)
						if err != nil {
							log.Printf("Cleanup command ignored error ('%s'): %v", cmdStr, err)
						}
					}
					log.Println("Unmount and cleanup complete.")
					firm.Batch(owner, func() { finalizeDone.Set(true) })
				}()
				return func() {}
			}, []firm.Reactive{ubootFlashDone})

			// 7. Power Cycle Node
			firm.Effect(owner, func() firm.CleanUp {
				if !finalizeDone.Get() {
					return func() {}
				}
				log.Println("[Step 7/9] Power cycling node...")
				go func() {
					fmt.Println("Powering off node after provisioning...")
					_, err := executeTpiCommandWithOutput("power", "off", "--node", nodeStr)
					if err != nil {
						log.Printf("Failed to power off node: %v", err)
						firm.Batch(owner, func() { discoveryError.Set(err) })
						return
					}
					time.Sleep(2 * time.Second)

					fmt.Println("Powering on node with Alpine...")
					_, err = executeTpiCommandWithOutput("power", "on", "--node", nodeStr)
					if err != nil {
						log.Printf("Failed to power on node: %v", err)
						firm.Batch(owner, func() { discoveryError.Set(err) })
						return
					}

					fmt.Println("Node powered on. Waiting for boot before polling UART...")
					time.Sleep(10 * time.Second) // Initial delay for boot
					firm.Batch(owner, func() { powerCycleDone.Set(true) })
				}()
				return func() {}
			}, []firm.Reactive{finalizeDone})

			// 8. Poll UART for IP Address
			pollingAttempt := firm.Signal(owner, 0)
			firm.Effect(owner, func() firm.CleanUp {
				if !powerCycleDone.Get() {
					return func() {}
				} // Wait for power cycle

				// Check if already done (IP found or timed out)
				if uartPollDone.Get() {
					return func() {}
				}

				maxAttempts := 20
				currentAttempt := pollingAttempt.Get()

				if currentAttempt >= maxAttempts {
					errMsg := fmt.Errorf("could not find IP address in UART output for node %s after %d attempts", nodeStr, maxAttempts)
					log.Println(errMsg)
					firm.Batch(owner, func() {
						discoveryError.Set(errMsg)
						uartTimeout.Set(true)
						uartPollDone.Set(true) // Mark polling as finished (due to timeout)
					})
					return func() {} // Stop polling
				}

				// Only launch goroutine if it's time for a new attempt (attempt number changed)
				log.Printf("[Step 8/9] Polling UART (Attempt %d/%d)...", currentAttempt+1, maxAttempts)
				go func(attempt int) {
					// Call base function directly
					uartOutput, err := executeTpiCommandWithOutput("uart", "get", "--node", nodeStr)
					if err != nil {
						log.Printf("Warning: failed to get UART output (will retry): %v", err)
						// Schedule retry
						time.Sleep(6 * time.Second)
						firm.Batch(owner, func() { pollingAttempt.Update(func(c int) int { return c + 1 }) })
						return
					}

					// Check output for IP
					ipRegex := regexp.MustCompile(`inet (\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})`)
					scanner := bufio.NewScanner(strings.NewReader(uartOutput))
					ipFound := false
					for scanner.Scan() {
						line := scanner.Text()
						matches := ipRegex.FindStringSubmatch(line)
						if len(matches) > 1 {
							ip := matches[1]
							if strings.HasPrefix(ip, "192.168.") || strings.HasPrefix(ip, "10.") || strings.HasPrefix(ip, "172.") {
								log.Printf("IP %s found. UART Polling successful.", ip)
								firm.Batch(owner, func() {
									discoveredIP.Set(ip)
									uartPollDone.Set(true) // Mark polling as finished (success)
								})
								ipFound = true
								break
							}
						}
					}

					// If not found, schedule retry
					if !ipFound {
						log.Println("IP not found in UART output, retrying...")
						time.Sleep(6 * time.Second)
						firm.Batch(owner, func() { pollingAttempt.Update(func(c int) int { return c + 1 }) })
					}
				}(currentAttempt)

				return func() {}
			}, []firm.Reactive{powerCycleDone, pollingAttempt}) // React to power cycle AND attempt changes

			// 9. Update State File (if IP found)
			firm.Effect(owner, func() firm.CleanUp {
				ip := discoveredIP.Get()
				// Run only once when a valid IP is discovered
				if ip == "" || !uartPollDone.Get() || stateUpdateDone.Get() {
					return func() {}
				}

				log.Println("[Step 9/9] Updating state file...")
				err := state.UpdateNodeState(discoverNodeID, func(status *state.NodeStatus) {
					status.IPAddress = ip
					status.Status = "provisioned"
					status.OS = "alpine"
					status.Error = ""
				})

				if err != nil {
					errMsg := fmt.Errorf("failed to update state file for node %d: %w", discoverNodeID, err)
					log.Println(errMsg)
					firm.Batch(owner, func() { discoveryError.Set(errMsg) })
				} else {
					fmt.Printf("Node %d state updated successfully with IP %s.\n", discoverNodeID, ip)
					firm.Batch(owner, func() { stateUpdateDone.Set(true) }) // Mark final success
				}
				return func() {}
			}, []firm.Reactive{discoveredIP, uartPollDone})

			// --- Handle Overall Completion/Error ---
			firm.Effect(owner, func() firm.CleanUp {
				// Check for success
				if stateUpdateDone.Get() {
					if !processComplete.Get() {
						log.Println("Overall discovery process completed successfully.")
						processComplete.Set(true)
					}
					return func() {}
				}

				// Check for error or timeout
				err := discoveryError.Get()
				timedOut := uartTimeout.Get()
				if err != nil || timedOut {
					if !processComplete.Get() {
						log.Printf("Overall discovery process failed. Error: %v, Timeout: %v", err, timedOut)
						finalError = err            // Store error for reporting after wait()
						if err == nil && timedOut { // Synthesize error for timeout case
							finalError = fmt.Errorf("discovery timed out waiting for IP")
						}
						processComplete.Set(true) // Mark as complete to stop wait()
					}
				}
				return func() {}
			}, []firm.Reactive{discoveryError, uartTimeout, stateUpdateDone}) // React to any final state

			log.Println("Finished setting up firm-go effects chain.")

			// Wait explicitly for the processComplete signal *within* the root scope
			// The wait() function returned by firm.Root might return early as no ops are tracked.
			log.Println("Waiting for processComplete signal...")
			for !processComplete.Get() {
				time.Sleep(100 * time.Millisecond) // Avoid busy-waiting
			}
			log.Println("processComplete signal received within root.")

			return func() { log.Println("Running main firm-go cleanup.") }
		})

		fmt.Println("Waiting for Alpine provisioning and discovery process to complete...")
		// Call the original wait() from firm.Root, although it might be non-blocking now.
		// The real wait happens in the loop above.
		wait()

		cleanup() // Run firm-go cleanup *after* the process is complete

		// --- Final Status Check ---
		log.Println("Process finished. Checking final state...")
		// Check finalError first, as state might not have been written on early failure
		if finalError != nil {
			fmt.Fprintf(os.Stderr, "Discovery process for node %d failed: %v\n", discoverNodeID, finalError)
			// Differentiate timeout
			if strings.Contains(finalError.Error(), "timed out") {
				os.Exit(1) // Specific exit code maybe?
			}
			os.Exit(1)
		}

		// If no error was captured, check the state file
		loadedState, err := state.LoadState()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading final state file: %v\n", err)
			os.Exit(1)
		}

		finalState, ok := loadedState.Nodes[discoverNodeID]
		if !ok {
			fmt.Fprintf(os.Stderr, "Error: Node %d not found in final state file, but no error was reported.\n", discoverNodeID)
			os.Exit(1)
		}

		// Check state file status
		if finalState.Status == "provisioned" && finalState.IPAddress != "" {
			fmt.Printf("Successfully provisioned Alpine and discovered IP %s for node %d.\n", finalState.IPAddress, discoverNodeID)
		} else {
			// This case implies success signal was set, but state file doesn't reflect it - unlikely
			fmt.Fprintf(os.Stderr, "Discovery process for node %d finished, but final state is unexpected: Status=%s, IP=%s\n", discoverNodeID, finalState.Status, finalState.IPAddress)
			os.Exit(1)
		}
		log.Println("Provisioning/Discovery command finished.")
	},
}

func init() {
	rootCmd.AddCommand(discoverCmd)

	discoverCmd.Flags().IntVarP(&discoverNodeID, "node", "n", 0, "Target node number (1-4) to provision Alpine on and discover (required)")

	if err := discoverCmd.MarkFlagRequired("node"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'node' as required: %v\n", err)
		os.Exit(1)
	}
}
