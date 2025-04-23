package bmc

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHConfig holds the configuration for SSH connections
type SSHConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	RemoteDir string `json:"remote_dir"`
}

// SSHExecutor implements CommandExecutor by executing commands over SSH on a remote Turing Pi cluster
type SSHExecutor struct {
	config SSHConfig
}

// NewSSHExecutorFromConfig creates a new SSHExecutor from a config file
func NewSSHExecutorFromConfig(configPath string) (*SSHExecutor, error) {
	// Read and parse the SSH config file
	configFile, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open SSH config file: %w", err)
	}
	defer configFile.Close()

	configData, err := io.ReadAll(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read SSH config file: %w", err)
	}

	var config SSHConfig
	if err := json.Unmarshal(configData, &config); err != nil {
		return nil, fmt.Errorf("failed to parse SSH config: %w", err)
	}

	return &SSHExecutor{
		config: config,
	}, nil
}

// NewSSHExecutor creates a new SSHExecutor from direct connection parameters
func NewSSHExecutor(host string, port int, user, password string) *SSHExecutor {
	return &SSHExecutor{
		config: SSHConfig{
			Host:     host,
			Port:     port,
			User:     user,
			Password: password,
		},
	}
}

// ExecuteCommand implements CommandExecutor interface by running commands over SSH
func (s *SSHExecutor) ExecuteCommand(command string) (stdout string, stderr string, err error) {
	// Build the SSH command
	// Example: ssh user@host -p port "command"
	sshCmd := fmt.Sprintf("ssh %s@%s -p %d",
		s.config.User,
		s.config.Host,
		s.config.Port)

	// If password auth is used, we would need to use sshpass or similar tools
	// This is a simplified version - in production, consider using SSH keys or a Go SSH library
	if s.config.Password != "" {
		sshCmd = fmt.Sprintf("sshpass -p '%s' %s", s.config.Password, sshCmd)
	}

	// Add the actual command to execute remotely
	fullCmd := fmt.Sprintf("%s \"%s\"", sshCmd, command)

	// Execute the SSH command
	cmd := exec.Command("sh", "-c", fullCmd)
	stdoutBytes, err := cmd.Output()
	stdout = string(stdoutBytes)

	// If there was an error, try to extract stderr from it
	var stderrBytes []byte
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderrBytes = exitErr.Stderr
		}
	}
	stderr = string(stderrBytes)

	// Trim trailing newlines for consistent behavior
	stdout = strings.TrimSuffix(stdout, "\n")
	stderr = strings.TrimSuffix(stderr, "\n")

	return stdout, stderr, err
}

// getSSHClientConfig creates an SSH client config from SSHConfig
func (s *SSHExecutor) getSSHClientConfig() *ssh.ClientConfig {
	return &ssh.ClientConfig{
		User: s.config.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(s.config.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}
}

// UploadFile implements FileUploader interface to upload files via SFTP
func (s *SSHExecutor) UploadFile(localPath, remotePath string) error {
	// Create SSH connection configuration
	sshConfig := s.getSSHClientConfig()

	// Connect to remote server
	addr := fmt.Sprintf("%s:22", s.config.Host)
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
