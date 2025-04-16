package node

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// InteractionStep defines a step in an interactive SSH session.
// It expects a certain prompt and sends a response.
type InteractionStep struct {
	Expect string // String to wait for in the output
	Send   string // String to send (newline typically added automatically)
	LogMsg string // Message to log before sending
}

// ExpectAndSend performs a sequence of expect/send interactions over an SSH session.
// It handles setting up the PTY and shell.
// It returns the full captured stdout/stderr output and an error if any step failed.
func ExpectAndSend(ip, user, password string, steps []InteractionStep, interactionTimeout time.Duration) (string, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         10 * time.Second, // Connection timeout
	}

	addr := fmt.Sprintf("%s:22", ip)
	fmt.Printf("[NODE SSH] Connecting to %s as %s...\n", addr, user)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return "", fmt.Errorf("failed to dial %s: %w", addr, err)
	}
	defer client.Close()
	fmt.Println("[NODE SSH] Connected.")

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}
	defer session.Close()

	// Set up pseudo-terminal
	modes := ssh.TerminalModes{
		ssh.ECHO:          0, // Disable echoing, common for password prompts
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

	fmt.Println("[NODE SSH] Requesting PTY...")
	if err := session.RequestPty("xterm", 80, 40, modes); err != nil {
		return "", fmt.Errorf("request for pseudo terminal failed: %w", err)
	}

	stdin, err := session.StdinPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	fmt.Println("[NODE SSH] Starting shell...")
	if err := session.Shell(); err != nil {
		return "", fmt.Errorf("failed to start shell: %w", err)
	}

	outputBuffer := bytes.Buffer{}
	// Capture stdout and potentially stderr
	multiWriter := io.MultiWriter(&outputBuffer)
	stdoutReader := bufio.NewReader(stdout)

	// Perform interaction steps
	for i, step := range steps {
		fmt.Printf("[NODE SSH] Step %d: Expecting '%s'\n", i+1, step.Expect)
		readBytes, err := readUntil(stdoutReader, multiWriter, step.Expect, interactionTimeout)
		if err != nil {
			log.Printf("[NODE SSH] Output before error/timeout:\n%s", outputBuffer.String())
			return outputBuffer.String(), fmt.Errorf("error waiting for prompt '%s': %w", step.Expect, err)
		}
		log.Printf("[NODE SSH] Read %d bytes until prompt.\nOutput snippet:\n%s", readBytes, GetLastLines(outputBuffer.String(), 5))

		// Ensure the Send string includes a newline if needed for shell commands
		sendData := step.Send
		if !strings.HasSuffix(sendData, "\n") {
			sendData += "\n"
		}

		fmt.Printf("[NODE SSH] Step %d: %s\n", i+1, step.LogMsg)
		if _, err := stdin.Write([]byte(sendData)); err != nil {
			return outputBuffer.String(), fmt.Errorf("failed to write '%s' to stdin: %w", step.Send, err)
		}
	}

	// Close stdin to signal the shell we are done sending input
	// This might cause the remote shell/process to exit if it was waiting for input.
	stdin.Close()

	// Read any remaining output until the session closes
	fmt.Println("[NODE SSH] Interactions complete. Reading final output...")
	// Use a tee reader to simultaneously write to buffer and log/process if needed
	remainingBytes, readErr := io.ReadAll(stdoutReader)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		log.Printf("[NODE SSH] Error reading final output: %v", readErr)
		// Don't necessarily fail here, just log it, but return the buffer content so far.
	}
	// Ensure remainingBytes are written to the buffer
	outputBuffer.Write(remainingBytes)
	finalFullOutput := outputBuffer.String()
	log.Printf("[NODE SSH] Final output snippet:\n%s", GetLastLines(finalFullOutput, 10))

	// Wait for the session to finish
	waitErr := session.Wait()
	if waitErr != nil {
		if exitErr, ok := waitErr.(*ssh.ExitError); ok {
			// Process exited, potentially expected depending on the interaction
			log.Printf("[NODE SSH] Session closed (exit status %d).", exitErr.ExitStatus())
		} else {
			// Other Wait error
			log.Printf("[NODE SSH] Warning: session.Wait() returned an unexpected error: %v", waitErr)
		}
		// Return existing error only if it's not an expected exit (like after successful password change)
		// This logic might need refinement based on specific interactions
		if _, ok := waitErr.(*ssh.ExitError); !ok {
			return finalFullOutput, waitErr // Return unexpected wait errors
		}
	}

	fmt.Println("[NODE SSH] Interaction sequence finished.")
	// Return the full output and nil error if the sequence completed without fatal errors
	return finalFullOutput, nil
}

// --- Helper functions ---

// readUntil reads from the reader, writing to the writer, until the target string is found in the writer's content or timeout.
func readUntil(reader *bufio.Reader, writer io.Writer, target string, timeout time.Duration) (int, error) {
	totalRead := 0
	startTime := time.Now()
	deadline := startTime.Add(timeout)

	// Use a buffer internally to check for the target string efficiently
	internalBuffer := bytes.Buffer{}
	teeWriter := io.MultiWriter(writer, &internalBuffer) // Write to original writer AND internal buffer

	foundChan := make(chan bool, 1)
	errorChan := make(chan error, 1)
	readBytesChan := make(chan int, 10)

	go func() {
		for {
			// Check if target already exists in internal buffer before reading more
			if strings.Contains(internalBuffer.String(), target) {
				foundChan <- true
				return
			}

			// Check for timeout within the goroutine
			if time.Now().After(deadline) {
				errorChan <- errors.New("timeout reached")
				return
			}

			b := make([]byte, 1) // Read one byte at a time
			n, err := reader.Read(b)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errorChan <- err // Report read errors
				} else {
					// EOF reached before target found
					errorChan <- fmt.Errorf("EOF reached before finding target: %s", target)
				}
				return
			}

			if n > 0 {
				// Write the byte to the tee writer (goes to original writer and internal buffer)
				_, writeErr := teeWriter.Write(b[:n])
				if writeErr != nil {
					errorChan <- fmt.Errorf("failed to write byte to buffer/writer: %w", writeErr)
					return
				}
				readBytesChan <- n
			}

			// Check if the target is now present in the internal buffer
			if strings.Contains(internalBuffer.String(), target) {
				foundChan <- true
				return
			}
		}
	}()

	for {
		select {
		case <-foundChan:
			close(readBytesChan)
			for n := range readBytesChan {
				totalRead += n
			}
			return totalRead, nil
		case numRead := <-readBytesChan:
			totalRead += numRead
		case err := <-errorChan:
			close(readBytesChan)
			for n := range readBytesChan {
				totalRead += n
			}
			return totalRead, err
		case <-time.After(time.Until(deadline)):
			close(readBytesChan)
			for n := range readBytesChan {
				totalRead += n
			}
			return totalRead, errors.New("timeout waiting for target string")
		}
	}
}

// GetLastLines returns the last N lines of a string.
// Renamed to be exported.
func GetLastLines(s string, n int) string {
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
