package bmc

import (
	"bytes"
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

// SSHConfig holds parameters for connecting to the BMC
// We might pass this around instead of relying on global vars in the future.
type SSHConfig struct {
	Host     string
	User     string
	Password string
	Timeout  time.Duration
}

func getSSHClientConfig(cfg SSHConfig) (*ssh.ClientConfig, error) {
	if cfg.User == "" {
		return nil, fmt.Errorf("SSH user cannot be empty")
	}
	// Password can be empty if using other auth methods in the future

	auth := []ssh.AuthMethod{}
	if cfg.Password != "" {
		auth = append(auth, ssh.Password(cfg.Password))
	} else {
		// Handle other auth methods if needed (e.g., key files)
		return nil, fmt.Errorf("SSH password is required (key auth not implemented)")
	}

	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 20 * time.Second // Default timeout
	}

	return &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Make this configurable?
		Timeout:         timeout,
	}, nil
}

// ExecuteCommand executes a command on the BMC via SSH.
// It requires providing explicit connection details.
func ExecuteCommand(cfg SSHConfig, command string) (stdout string, stderr string, err error) {
	sshConfig, err := getSSHClientConfig(cfg)
	if err != nil {
		return "", "", fmt.Errorf("invalid SSH config: %w", err)
	}

	addr := fmt.Sprintf("%s:22", cfg.Host)
	log.Printf("[BMC SSH EXEC] Connecting to %s...", addr)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return "", "", fmt.Errorf("ssh dial to %s failed: %w", addr, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("ssh session creation failed: %w", err)
	}
	defer session.Close()

	var stdoutBuf, stderrBuf bytes.Buffer
	session.Stdout = &stdoutBuf
	session.Stderr = &stderrBuf

	log.Printf("[BMC SSH EXEC] Running: %s", command)
	err = session.Run(command)

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	if stdoutStr != "" {
		log.Printf("[BMC SSH STDOUT]:\n%s", stdoutStr)
	}
	if stderrStr != "" {
		log.Printf("[BMC SSH STDERR]:\n%s", stderrStr)
	}

	if err != nil {
		// Combine error and stderr for better context
		return stdoutStr, stderrStr, fmt.Errorf("command '%s' failed: %w. Stderr: %s", command, err, stderrStr)
	}

	log.Printf("[BMC SSH EXEC] Command '%s' completed successfully.", command)
	return stdoutStr, stderrStr, nil
}

// CheckRemoteFileExists checks if a file exists on the remote host using ls.
// It requires providing explicit connection details.
func CheckRemoteFileExists(cfg SSHConfig, remotePath string) (bool, error) {
	// Use 'ls' and check exit code / stderr. ls exits 0 if file exists,
	// non-zero (often 1 or 2) if not found.
	// We redirect stderr to stdout to capture potential "No such file" messages in one place.
	cmdStr := fmt.Sprintf("ls %s 2>&1", remotePath)
	stdout, _, err := ExecuteCommand(cfg, cmdStr)

	if err == nil {
		// Command succeeded (exit code 0), file exists
		log.Printf("[BMC SSH LS] File %s exists.", remotePath)
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
				log.Printf("[BMC SSH LS] File %s does not exist (ls exit code %d).", remotePath, exitErr.ExitStatus())
				return false, nil // File not found is not an execution error
			}
		}
		// If we couldn't confirm exit code, rely on stdout message check
		if strings.Contains(stdout, "No such file or directory") {
			log.Printf("[BMC SSH LS] File %s does not exist (stderr message).", remotePath)
			return false, nil
		}
	}

	// If the error wasn't clearly a "not found" error, report it as a failure
	log.Printf("[BMC SSH LS] Error checking file %s: %v. Output: %s", remotePath, err, stdout)
	return false, fmt.Errorf("failed to check remote file %s: %w. Output: %s", remotePath, err, stdout)
}

// UploadFile uploads a local file to a remote path using SFTP.
// It requires providing explicit connection details.
func UploadFile(cfg SSHConfig, localPath, remotePath string) error {
	sshConfig, err := getSSHClientConfig(cfg)
	if err != nil {
		return fmt.Errorf("invalid SSH config: %w", err)
	}

	addr := fmt.Sprintf("%s:22", cfg.Host)
	log.Printf("[BMC SCP UPLOAD] Connecting to %s...", addr)
	conn, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("ssh dial for sftp to %s failed: %w", addr, err)
	}
	defer conn.Close()

	log.Println("[BMC SCP UPLOAD] Creating SFTP client...")
	client, err := sftp.NewClient(conn)
	if err != nil {
		return fmt.Errorf("sftp client creation failed: %w", err)
	}
	defer client.Close()

	remoteDir := filepath.Dir(remotePath)
	log.Printf("[BMC SCP UPLOAD] Ensuring remote directory exists: %s", remoteDir)
	// MkdirAll creates parent directories as needed.
	if err := client.MkdirAll(remoteDir); err != nil {
		// Ignore error if directory already exists, handle others
		// Stat returns an error if path doesn't exist
		if _, statErr := client.Stat(remoteDir); os.IsNotExist(statErr) {
			return fmt.Errorf("failed to create remote directory %s: %w", remoteDir, err)
		}
		// If Stat succeeded, directory exists, ignore MkdirAll error
		log.Printf("[BMC SCP UPLOAD] Remote directory %s likely already exists.", remoteDir)
	} else {
		log.Printf("[BMC SCP UPLOAD] Created remote directory %s.", remoteDir)
	}

	log.Printf("[BMC SCP UPLOAD] Opening local file: %s", localPath)
	srcFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file %s: %w", localPath, err)
	}
	defer srcFile.Close()

	log.Printf("[BMC SCP UPLOAD] Creating remote file: %s", remotePath)
	dstFile, err := client.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file %s: %w", remotePath, err)
	}
	defer dstFile.Close()

	log.Printf("[BMC SCP UPLOAD] Copying data...")
	bytesCopied, err := io.Copy(dstFile, srcFile)
	if err != nil {
		// Attempt to remove partially uploaded file on error
		_ = client.Remove(remotePath)
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	log.Printf("[BMC SCP UPLOAD] Successfully copied %d bytes to %s", bytesCopied, remotePath)
	return nil
}
