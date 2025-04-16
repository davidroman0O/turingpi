package cmd

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
)

var (
	postInstallNodeIP      string
	postInstallInitialUser string
	postInstallInitialPass string
	postInstallNewPass     string
)

// postInstallUbuntuCmd represents the post-install-ubuntu command
var postInstallUbuntuCmd = &cobra.Command{
	Use:   "post-install-ubuntu",
	Short: "Perform initial setup (password change) for Ubuntu nodes",
	Long: `Connects directly to a freshly installed Ubuntu node via SSH 
and automates the mandatory initial password change process.

Requires the node to be booted and reachable via the provided IP address.
Uses the default initial credentials (ubuntu/ubuntu) unless overridden.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting Ubuntu post-installation steps...")

		// --- Input Validation ---
		if postInstallNodeIP == "" {
			fmt.Fprintln(os.Stderr, "Error: --node-ip is required.")
			os.Exit(1)
		}
		if postInstallNewPass == "" {
			fmt.Fprintln(os.Stderr, "Error: --new-password is required.")
			os.Exit(1)
		}

		err := runUbuntuPasswordChange()
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nPost-installation failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Println("\nPost-installation completed successfully!")
	},
}

func runUbuntuPasswordChange() error {
	config := &ssh.ClientConfig{
		User: postInstallInitialUser,
		Auth: []ssh.AuthMethod{
			ssh.Password(postInstallInitialPass),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second, // Connection timeout
	}

	addr := fmt.Sprintf("%s:22", postInstallNodeIP)
	fmt.Printf("Connecting to %s as %s...\n", addr, postInstallInitialUser)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return fmt.Errorf("failed to dial %s: %w", addr, err)
	}
	defer client.Close()
	fmt.Println("Connected.")

	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Set up pseudo-terminal
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,     // Disable echoing (passwords)
		ssh.TTY_OP_ISPEED: 14400, // Typical speeds
		ssh.TTY_OP_OSPEED: 14400,
	}

	fmt.Println("Requesting PTY...")
	if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
		return fmt.Errorf("request for pseudo terminal failed: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	// Stderr can be useful too, though prompts are usually on stdout
	// stderr, _ := session.StderrPipe()

	fmt.Println("Starting shell...")
	if err := session.Shell(); err != nil {
		return fmt.Errorf("failed to start shell: %w", err)
	}

	interactionTimeout := 30 * time.Second // Timeout for each expect/send step

	// --- Interaction Sequence ---
	prompts := []struct {
		expect string
		send   string
		logMsg string
	}{
		{"Current password:", postInstallInitialPass + "\n", "Sending initial password..."},
		{"New password:", postInstallNewPass + "\n", "Sending new password..."},
		{"Retype new password:", postInstallNewPass + "\n", "Retyping new password..."},
	}

	outputBuffer := bytes.Buffer{}
	stdoutReader := bufio.NewReader(stdout)

	for _, p := range prompts {
		fmt.Printf("Expecting '%s'\n", p.expect)
		readBytes, err := readUntil(stdoutReader, &outputBuffer, p.expect, interactionTimeout)
		if err != nil {
			log.Printf("Output before error/timeout:\n%s", outputBuffer.String())
			return fmt.Errorf("error waiting for prompt '%s': %w", p.expect, err)
		}
		log.Printf("Read %d bytes until prompt.\nOutput snippet:\n%s", readBytes, getLastLines(outputBuffer.String(), 5))

		fmt.Println(p.logMsg)
		if _, err := stdin.Write([]byte(p.send)); err != nil {
			return fmt.Errorf("failed to write to stdin: %w", err)
		}
	}

	// --- Final Check ---
	fmt.Println("Checking final result...")
	// Read remaining output until connection closes or timeout
	finalOutputChan := make(chan string)
	finalErrorChan := make(chan error)
	go func() {
		remainingBytes, err := io.ReadAll(stdoutReader) // Read until EOF (connection close)
		if err != nil && !errors.Is(err, io.EOF) {
			finalErrorChan <- err
			return
		}
		outputBuffer.Write(remainingBytes) // Append final output
		finalOutputChan <- outputBuffer.String()
	}()

	var finalFullOutput string
	select {
	case finalFullOutput = <-finalOutputChan:
		log.Println("Read final output successfully.")
	case err := <-finalErrorChan:
		log.Printf("Error reading final output: %v\nOutput so far:\n%s", err, outputBuffer.String())
		return fmt.Errorf("error reading final output: %w", err)
	case <-time.After(interactionTimeout): // Timeout for final check
		log.Printf("Timeout reading final output.\nOutput so far:\n%s", outputBuffer.String())
		return errors.New("timeout waiting for final confirmation")
	}

	log.Printf("Final output check:\n%s", getLastLines(finalFullOutput, 10))

	// Check for success message or known errors
	if strings.Contains(finalFullOutput, "passwd: password updated successfully") {
		fmt.Println("Password change confirmed.")
	} else if strings.Contains(finalFullOutput, "You must choose a longer password") {
		return errors.New("password rejected by node: too short")
	} else if strings.Contains(finalFullOutput, "Sorry, passwords do not match") {
		return errors.New("password rejected by node: passwords do not match")
	} else {
		// Add more known error checks if needed
		return errors.New("password change confirmation not found in output")
	}

	// We expect the session to close after successful password change
	err = session.Wait()
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			// Process exited, this is expected after password change closes connection
			log.Printf("Session closed as expected (exit status %d).", exitErr.ExitStatus())
		} else {
			// Other Wait error, might be unexpected
			log.Printf("Warning: session.Wait() returned an unexpected error: %v", err)
		}
	}

	return nil // Success
}

// readUntil reads from the reader into the buffer until the target string is found or timeout.
func readUntil(reader *bufio.Reader, buffer *bytes.Buffer, target string, timeout time.Duration) (int, error) {
	totalRead := 0
	startTime := time.Now()
	deadline := startTime.Add(timeout)

	// Channel to signal when the target is found
	foundChan := make(chan bool, 1)
	// Channel to signal errors during reading
	errorChan := make(chan error, 1)
	// Channel to pass bytes read
	readBytesChan := make(chan int, 10)

	go func() {
		for {
			// Check if target already exists in buffer before reading more
			if strings.Contains(buffer.String(), target) {
				foundChan <- true
				return
			}

			// Check for timeout within the goroutine
			if time.Now().After(deadline) {
				// Signal timeout via error channel, but don't close it, let select handle it
				errorChan <- errors.New("timeout reached")
				return
			}

			b, err := reader.ReadByte()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errorChan <- err // Report read errors
				} else {
					// EOF reached before target found
					errorChan <- fmt.Errorf("EOF reached before finding target: %s", target)
				}
				return
			}

			buffer.WriteByte(b) // Append to buffer
			readBytesChan <- 1
			// Optional: Log the character read for debugging
			// log.Printf("Read char: %c", b)

			// Check if the target is now present in the buffer
			if strings.Contains(buffer.String(), target) {
				foundChan <- true
				return
			}
		}
	}()

	for {
		select {
		case <-foundChan:
			// Target found
			// Consume remaining bytes from readBytesChan to get total
			close(readBytesChan)
			for n := range readBytesChan {
				totalRead += n
			}
			return totalRead, nil
		case numRead := <-readBytesChan:
			totalRead += numRead
			// Continue loop, waiting for foundChan or errorChan
		case err := <-errorChan:
			// Error occurred during read or timeout in goroutine
			// Consume remaining bytes from readBytesChan
			close(readBytesChan)
			for n := range readBytesChan {
				totalRead += n
			}
			return totalRead, err
		case <-time.After(time.Until(deadline)):
			// Overall timeout check in select
			// Consume remaining bytes from readBytesChan
			close(readBytesChan)
			for n := range readBytesChan {
				totalRead += n
			}
			return totalRead, errors.New("timeout waiting for target string")
		}
	}
	// This part should not be reachable due to the infinite loop and select
	return totalRead, errors.New("unexpected exit from read loop")
}

// getLastLines returns the last N lines of a string
func getLastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	start := len(lines) - n
	if start < 0 {
		start = 0
	}
	// Handle potential trailing newline creating an empty last element
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) > n {
		return strings.Join(lines[len(lines)-n:], "\n")
	}
	return s
}

func init() {
	rootCmd.AddCommand(postInstallUbuntuCmd)

	postInstallUbuntuCmd.Flags().StringVar(&postInstallNodeIP, "node-ip", "", "IP address of the target node (required)")
	postInstallUbuntuCmd.Flags().StringVar(&postInstallInitialUser, "initial-user", "ubuntu", "Initial username")
	postInstallUbuntuCmd.Flags().StringVar(&postInstallInitialPass, "initial-password", "ubuntu", "Initial password")
	postInstallUbuntuCmd.Flags().StringVar(&postInstallNewPass, "new-password", "", "New password to set (required)")

	if err := postInstallUbuntuCmd.MarkFlagRequired("node-ip"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'node-ip' as required: %v\n", err)
		os.Exit(1)
	}
	if err := postInstallUbuntuCmd.MarkFlagRequired("new-password"); err != nil {
		fmt.Fprintf(os.Stderr, "Error marking flag 'new-password' as required: %v\n", err)
		os.Exit(1)
	}
}
