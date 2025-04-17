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

// getSSHClient establishes an SSH client connection
func (a *nodeAdapter) getSSHClient() (*ssh.Client, error) {
	if err := a.checkClosed(); err != nil {
		return nil, err
	}

	config := &ssh.ClientConfig{
		User: a.config.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(a.config.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         a.config.Timeout,
	}

	addr := fmt.Sprintf("%s:22", a.config.Host)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial %s: %w", addr, err)
	}

	// Track the client
	a.mu.Lock()
	a.clients[client] = true
	a.mu.Unlock()

	return client, nil
}

// ExecuteCommand implements NodeAdapter
func (a *nodeAdapter) ExecuteCommand(command string) (stdout string, stderr string, err error) {
	if err := a.checkClosed(); err != nil {
		return "", "", err
	}

	client, err := a.getSSHClient()
	if err != nil {
		return "", "", fmt.Errorf("failed to establish SSH connection: %w", err)
	}
	// Don't defer client.Close() anymore as it's managed by the adapter

	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("failed to create session: %w", err)
	}

	// Track the session
	a.mu.Lock()
	a.sessions[session] = true
	a.mu.Unlock()

	defer func() {
		session.Close()
		a.mu.Lock()
		delete(a.sessions, session)
		a.mu.Unlock()
	}()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	log.Printf("[NODE SSH EXEC] Running: %s", command)
	err = session.Run(command)

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if stdoutStr != "" {
		log.Printf("[NODE SSH STDOUT]:\n%s", stdoutStr)
	}
	if stderrStr != "" {
		log.Printf("[NODE SSH STDERR]:\n%s", stderrStr)
	}

	if err != nil {
		return stdoutStr, stderrStr, fmt.Errorf("command '%s' failed: %w. Stderr: %s", command, err, stderrStr)
	}

	log.Printf("[NODE SSH EXEC] Command '%s' completed successfully.", command)
	return stdoutStr, stderrStr, nil
}

// ExpectAndSend implements NodeAdapter
func (a *nodeAdapter) ExpectAndSend(steps []InteractionStep, interactionTimeout time.Duration) (string, error) {
	if err := a.checkClosed(); err != nil {
		return "", err
	}

	client, err := a.getSSHClient()
	if err != nil {
		return "", fmt.Errorf("failed to establish SSH connection: %w", err)
	}
	// Don't defer client.Close() anymore as it's managed by the adapter

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	// Track the session
	a.mu.Lock()
	a.sessions[session] = true
	a.mu.Unlock()

	defer func() {
		session.Close()
		a.mu.Lock()
		delete(a.sessions, session)
		a.mu.Unlock()
	}()

	// Set up pseudo-terminal
	modes := ssh.TerminalModes{
		ssh.ECHO:          0,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}

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

	if err := session.Shell(); err != nil {
		return "", fmt.Errorf("failed to start shell: %w", err)
	}

	outputBuffer := bytes.Buffer{}
	multiWriter := io.MultiWriter(&outputBuffer)
	stdoutReader := bufio.NewReader(stdout)

	// Perform interaction steps
	for i, step := range steps {
		log.Printf("[NODE SSH] Step %d: Expecting '%s'\n", i+1, step.Expect)
		readBytes, err := readUntil(stdoutReader, multiWriter, step.Expect, interactionTimeout)
		if err != nil {
			log.Printf("[NODE SSH] Output before error/timeout:\n%s", outputBuffer.String())
			return outputBuffer.String(), fmt.Errorf("error waiting for prompt '%s': %w", step.Expect, err)
		}
		log.Printf("[NODE SSH] Read %d bytes until prompt.\nOutput snippet:\n%s", readBytes, getLastLines(outputBuffer.String(), 5))

		sendData := step.Send
		if !strings.HasSuffix(sendData, "\n") {
			sendData += "\n"
		}

		log.Printf("[NODE SSH] Step %d: %s\n", i+1, step.LogMsg)
		if _, err := stdin.Write([]byte(sendData)); err != nil {
			return outputBuffer.String(), fmt.Errorf("failed to write '%s' to stdin: %w", step.Send, err)
		}
	}

	stdin.Close()

	remainingBytes, readErr := io.ReadAll(stdoutReader)
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		log.Printf("[NODE SSH] Error reading final output: %v", readErr)
	}
	outputBuffer.Write(remainingBytes)
	finalOutput := outputBuffer.String()
	log.Printf("[NODE SSH] Final output snippet:\n%s", getLastLines(finalOutput, 10))

	waitErr := session.Wait()
	if waitErr != nil {
		if exitErr, ok := waitErr.(*ssh.ExitError); ok {
			log.Printf("[NODE SSH] Session closed (exit status %d).", exitErr.ExitStatus())
		} else {
			log.Printf("[NODE SSH] Warning: session.Wait() returned an unexpected error: %v", waitErr)
		}
		if _, ok := waitErr.(*ssh.ExitError); !ok {
			return finalOutput, waitErr
		}
	}

	log.Println("[NODE SSH] Interaction sequence finished.")
	return finalOutput, nil
}

// readUntil reads from the reader, writing to the writer, until the target string is found or timeout
func readUntil(reader *bufio.Reader, writer io.Writer, target string, timeout time.Duration) (int, error) {
	totalRead := 0
	startTime := time.Now()
	deadline := startTime.Add(timeout)

	internalBuffer := bytes.Buffer{}
	teeWriter := io.MultiWriter(writer, &internalBuffer)

	foundChan := make(chan bool, 1)
	errorChan := make(chan error, 1)
	readBytesChan := make(chan int, 10)

	go func() {
		for {
			if strings.Contains(internalBuffer.String(), target) {
				foundChan <- true
				return
			}

			if time.Now().After(deadline) {
				errorChan <- errors.New("timeout reached")
				return
			}

			b := make([]byte, 1)
			n, err := reader.Read(b)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					errorChan <- err
				} else {
					errorChan <- fmt.Errorf("EOF reached before finding target: %s", target)
				}
				return
			}

			if n > 0 {
				_, writeErr := teeWriter.Write(b[:n])
				if writeErr != nil {
					errorChan <- fmt.Errorf("failed to write byte to buffer/writer: %w", writeErr)
					return
				}
				readBytesChan <- n
			}

			if strings.Contains(internalBuffer.String(), target) {
				foundChan <- true
				return
			}
		}
	}()

	for {
		select {
		case <-foundChan:
			return totalRead, nil
		case err := <-errorChan:
			return totalRead, err
		case n := <-readBytesChan:
			totalRead += n
		}
	}
}

// getLastLines returns the last n lines of a string
func getLastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
