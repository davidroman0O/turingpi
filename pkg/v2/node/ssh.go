package node

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

func (n *nodeImpl) ExpectAndSend(ctx context.Context, steps []InteractionStep, timeout time.Duration) (string, error) {
	var output string
	err := n.withRetry(ctx, "expect and send interaction", func() error {
		client, err := n.getSSHClient()
		if err != nil {
			return fmt.Errorf("failed to get SSH client: %w", err)
		}

		session, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("failed to create session: %w", err)
		}

		n.mu.Lock()
		n.sessions[session] = true
		n.mu.Unlock()

		defer func() {
			session.Close()
			n.mu.Lock()
			delete(n.sessions, session)
			n.mu.Unlock()
		}()

		// Set up pseudo-terminal
		modes := ssh.TerminalModes{
			ssh.ECHO:          0,
			ssh.TTY_OP_ISPEED: 14400,
			ssh.TTY_OP_OSPEED: 14400,
		}

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

		if err := session.Shell(); err != nil {
			return fmt.Errorf("failed to start shell: %w", err)
		}

		outputBuffer := strings.Builder{}
		stdoutReader := bufio.NewReader(stdout)

		// Perform interaction steps
		for i, step := range steps {
			log.Printf("[NODE %d] Step %d: Expecting '%s'\n", n.id, i+1, step.Expect)
			if err := waitForPrompt(ctx, stdoutReader, &outputBuffer, step.Expect, timeout); err != nil {
				log.Printf("[NODE %d] Output before error/timeout:\n%s", n.id, outputBuffer.String())
				return fmt.Errorf("error waiting for prompt '%s': %w", step.Expect, err)
			}

			sendData := step.Send
			if !strings.HasSuffix(sendData, "\n") {
				sendData += "\n"
			}

			log.Printf("[NODE %d] Step %d: %s\n", n.id, i+1, step.LogMsg)
			if _, err := stdin.Write([]byte(sendData)); err != nil {
				return fmt.Errorf("failed to write '%s' to stdin: %w", step.Send, err)
			}
		}

		stdin.Close()

		// Read any remaining output
		remainingBytes, err := io.ReadAll(stdoutReader)
		if err != nil && !errors.Is(err, io.EOF) {
			log.Printf("[NODE %d] Error reading final output: %v", n.id, err)
		}
		outputBuffer.Write(remainingBytes)
		output = outputBuffer.String()

		return nil
	})

	return output, err
}

func (n *nodeImpl) CopyFile(ctx context.Context, localPath, remotePath string, toNode bool) error {
	return n.withRetry(ctx, fmt.Sprintf("copy file %s to %s", localPath, remotePath), func() error {
		client, err := n.getSSHClient()
		if err != nil {
			return fmt.Errorf("failed to get SSH client: %w", err)
		}

		sftpClient, err := sftp.NewClient(client)
		if err != nil {
			return fmt.Errorf("failed to create SFTP client: %w", err)
		}
		defer sftpClient.Close()

		if toNode {
			// Local to Remote
			log.Printf("[NODE %d] Uploading %s to %s...", n.id, localPath, remotePath)
			remoteDir := filepath.Dir(remotePath)
			// Ensure remote directory exists
			if err := sftpClient.MkdirAll(remoteDir); err != nil {
				if _, statErr := sftpClient.Stat(remoteDir); os.IsNotExist(statErr) {
					return fmt.Errorf("failed to create remote directory %s: %w", remoteDir, err)
				}
				log.Printf("[NODE %d] Remote directory %s likely already exists.", n.id, remoteDir)
			}

			srcFile, err := os.Open(localPath)
			if err != nil {
				return fmt.Errorf("failed to open local file %s: %w", localPath, err)
			}
			defer srcFile.Close()

			dstFile, err := sftpClient.Create(remotePath)
			if err != nil {
				return fmt.Errorf("failed to create remote file %s: %w", remotePath, err)
			}
			defer dstFile.Close()

			if _, err := io.Copy(dstFile, srcFile); err != nil {
				_ = sftpClient.Remove(remotePath) // Attempt cleanup
				return fmt.Errorf("failed to copy content to remote: %w", err)
			}
		} else {
			// Remote to Local
			log.Printf("[NODE %d] Downloading %s to %s...", n.id, remotePath, localPath)
			localDir := filepath.Dir(localPath)
			// Ensure local directory exists
			if err := os.MkdirAll(localDir, 0755); err != nil {
				return fmt.Errorf("failed to create local directory %s: %w", localDir, err)
			}

			srcFile, err := sftpClient.Open(remotePath)
			if err != nil {
				return fmt.Errorf("failed to open remote file %s: %w", remotePath, err)
			}
			defer srcFile.Close()

			dstFile, err := os.Create(localPath)
			if err != nil {
				return fmt.Errorf("failed to create local file %s: %w", localPath, err)
			}
			defer dstFile.Close()

			if _, err := io.Copy(dstFile, srcFile); err != nil {
				_ = os.Remove(localPath) // Attempt cleanup
				return fmt.Errorf("failed to copy content to local: %w", err)
			}
		}

		return nil
	})
}

// waitForPrompt waits for a specific string in the output with timeout
func waitForPrompt(ctx context.Context, reader *bufio.Reader, writer *strings.Builder, target string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for prompt: %s", target)
			}

			b := make([]byte, 1)
			n, err := reader.Read(b)
			if err != nil {
				if err == io.EOF {
					return fmt.Errorf("EOF reached before finding target: %s", target)
				}
				return err
			}

			if n > 0 {
				writer.Write(b[:n])
				if strings.Contains(writer.String(), target) {
					return nil
				}
			}
		}
	}
}
