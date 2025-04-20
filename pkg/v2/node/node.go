package node

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/davidroman0O/turingpi/pkg/v2/bmc"
	"golang.org/x/crypto/ssh"
)

// Node defines the interface for interacting with a Turing Pi node
type Node interface {
	// ExecuteCommand runs a non-interactive command on the target node via SSH
	ExecuteCommand(ctx context.Context, command string) (stdout string, stderr string, err error)

	// ExpectAndSend performs a sequence of expect/send interactions over an SSH session
	ExpectAndSend(ctx context.Context, steps []InteractionStep, timeout time.Duration) (string, error)

	// CopyFile copies a file to or from the node
	CopyFile(ctx context.Context, localPath, remotePath string, toNode bool) error

	// GetInfo retrieves detailed information about the node
	GetInfo(ctx context.Context) (*NodeInfo, error)

	// GetPowerStatus retrieves the power status of the node
	GetPowerStatus(ctx context.Context) (*bmc.PowerStatus, error)

	// PowerOn turns on the node
	PowerOn(ctx context.Context) error

	// PowerOff turns off the node
	PowerOff(ctx context.Context) error

	// Reset performs a hard reset on the node
	Reset(ctx context.Context) error

	// Close releases any resources used by the node
	Close() error
}

// nodeImpl implements the Node interface
type nodeImpl struct {
	id       int
	config   *SSHConfig
	bmc      bmc.BMC
	mu       sync.Mutex
	client   *ssh.Client
	sessions map[*ssh.Session]bool
	closed   bool
}

// NewNode creates a new Node instance
func NewNode(id int, config *SSHConfig, bmcClient bmc.BMC) Node {
	return &nodeImpl{
		id:       id,
		config:   config,
		bmc:      bmcClient,
		sessions: make(map[*ssh.Session]bool),
	}
}

// withRetry executes the given operation with retry logic
func (n *nodeImpl) withRetry(ctx context.Context, operation string, fn func() error) error {
	var lastErr error
	currentDelay := n.config.RetryDelay

	for attempt := 0; attempt <= n.config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if attempt > 0 {
			log.Printf("[NODE %d] Retry attempt %d/%d for operation '%s' after %v delay",
				n.id, attempt, n.config.MaxRetries, operation, currentDelay)
			time.Sleep(currentDelay)
			currentDelay += n.config.RetryIncrement
		}

		err := fn()
		if err == nil {
			if attempt > 0 {
				log.Printf("[NODE %d] Operation '%s' succeeded after %d retries", n.id, operation, attempt)
			}
			return nil
		}

		lastErr = err
		log.Printf("[NODE %d] Operation '%s' failed (attempt %d/%d): %v",
			n.id, operation, attempt+1, n.config.MaxRetries+1, err)
	}

	return fmt.Errorf("operation '%s' failed after %d retries: %w",
		operation, n.config.MaxRetries, lastErr)
}

// getSSHClient establishes an SSH client connection
func (n *nodeImpl) getSSHClient() (*ssh.Client, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return nil, fmt.Errorf("node is closed")
	}

	if n.client != nil {
		return n.client, nil
	}

	config := &ssh.ClientConfig{
		User: n.config.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(n.config.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         n.config.Timeout,
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", n.config.Host), config)
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}

	n.client = client
	return client, nil
}

func (n *nodeImpl) ExecuteCommand(ctx context.Context, command string) (string, string, error) {
	var stdout, stderr string
	err := n.withRetry(ctx, fmt.Sprintf("execute command: %s", command), func() error {
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

		var stdoutBuf, stderrBuf strings.Builder
		session.Stdout = &stdoutBuf
		session.Stderr = &stderrBuf

		if err := session.Run(command); err != nil {
			return fmt.Errorf("command failed: %w", err)
		}

		stdout = stdoutBuf.String()
		stderr = stderrBuf.String()
		return nil
	})

	return stdout, stderr, err
}

func (n *nodeImpl) GetInfo(ctx context.Context) (*NodeInfo, error) {
	// Get hardware info
	stdout, stderr, err := n.ExecuteCommand(ctx, "uname -a && lscpu && free -h && df -h")
	if err != nil {
		return nil, fmt.Errorf("failed to get system info: %w (stderr: %s)", err, stderr)
	}

	// Get network info
	netOut, netErr, err := n.ExecuteCommand(ctx, "hostname && ip addr && ip route")
	if err != nil {
		return nil, fmt.Errorf("failed to get network info: %w (stderr: %s)", err, netErr)
	}

	// Parse the output and create NodeInfo
	// This is a simplified version - in practice you'd want more robust parsing
	info := &NodeInfo{
		ID:     n.id,
		Status: "online", // We got this far, so the node is online
		Hardware: HardwareInfo{
			Model: parseUname(stdout),
			CPU:   parseCPU(stdout),
		},
		Network: NetworkInfo{
			Hostname:  parseHostname(netOut),
			IPAddress: parseIPAddress(netOut),
		},
	}

	return info, nil
}

func (n *nodeImpl) GetPowerStatus(ctx context.Context) (*bmc.PowerStatus, error) {
	return n.bmc.GetPowerStatus(ctx, n.id)
}

func (n *nodeImpl) PowerOn(ctx context.Context) error {
	return n.bmc.PowerOn(ctx, n.id)
}

func (n *nodeImpl) PowerOff(ctx context.Context) error {
	return n.bmc.PowerOff(ctx, n.id)
}

func (n *nodeImpl) Reset(ctx context.Context) error {
	return n.bmc.Reset(ctx, n.id)
}

func (n *nodeImpl) Close() error {
	n.mu.Lock()
	defer n.mu.Unlock()

	if n.closed {
		return nil
	}

	// Close all sessions
	for session := range n.sessions {
		session.Close()
		delete(n.sessions, session)
	}

	// Close client
	if n.client != nil {
		if err := n.client.Close(); err != nil {
			return fmt.Errorf("failed to close SSH client: %w", err)
		}
		n.client = nil
	}

	n.closed = true
	return nil
}

// Helper functions for parsing command output
func parseUname(output string) string {
	// Simplified - would need more robust parsing in practice
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		return lines[0]
	}
	return ""
}

func parseCPU(output string) string {
	// Simplified - would need more robust parsing in practice
	if idx := strings.Index(output, "model name"); idx >= 0 {
		line := output[idx:]
		if nl := strings.Index(line, "\n"); nl > 0 {
			return strings.TrimSpace(line[10:nl])
		}
	}
	return ""
}

func parseHostname(output string) string {
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return ""
}

func parseIPAddress(output string) string {
	// Simplified - would need more robust parsing in practice
	if idx := strings.Index(output, "inet "); idx >= 0 {
		line := output[idx+5:]
		if sp := strings.Index(line, " "); sp > 0 {
			return line[:sp]
		}
	}
	return ""
}
