package node

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/cache"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHConfig holds the configuration for SSH connections to nodes
type SSHConfig struct {
	Host     string
	User     string
	Password string
	Timeout  time.Duration
	// Retry configuration
	MaxRetries     int           // Maximum number of retry attempts
	RetryDelay     time.Duration // Delay between retries
	RetryIncrement time.Duration // How much to increase delay after each retry
}

// InteractionStep defines a step in an interactive SSH session.
// It expects a certain prompt and sends a response.
type InteractionStep struct {
	Expect string // String to wait for in the output
	Send   string // String to send (newline typically added automatically)
	LogMsg string // Message to log before sending
}

// NodeAdapter defines the interface for interacting with Turing Pi nodes
type NodeAdapter interface {
	// ExecuteCommand runs a non-interactive command on the target node via SSH
	// Now includes retry logic for network issues
	ExecuteCommand(command string) (stdout string, stderr string, err error)

	// ExpectAndSend performs a sequence of expect/send interactions over an SSH session
	// Now includes retry logic for network issues
	ExpectAndSend(steps []InteractionStep, interactionTimeout time.Duration) (string, error)

	// FileOperations returns the Cache interface for file operations
	// This replaces the old CopyFile method
	FileOperations() cache.Cache

	// ExecuteBMCCommand executes a BMC-specific command
	// Examples: power on/off, reset, get status
	ExecuteBMCCommand(command string) (stdout string, stderr string, err error)

	// GetPowerStatus retrieves the power status of a specific node
	GetPowerStatus(ctx context.Context, nodeID int) (*PowerStatus, error)

	// PowerOn turns on a specific node
	PowerOn(ctx context.Context, nodeID int) error

	// PowerOff turns off a specific node
	PowerOff(ctx context.Context, nodeID int) error

	// Reset performs a hard reset on a specific node
	Reset(ctx context.Context, nodeID int) error

	// GetBMCInfo retrieves information about the BMC
	GetBMCInfo(ctx context.Context) (*BMCInfo, error)

	// RebootBMC reboots the BMC chip
	RebootBMC(ctx context.Context) error

	// UpdateBMCFirmware updates the BMC firmware
	UpdateBMCFirmware(ctx context.Context, firmwarePath string) error

	// Close closes any open connections and frees resources
	Close() error
}

// NewNodeAdapter creates a new instance of NodeAdapter
func NewNodeAdapter(config SSHConfig) NodeAdapter {
	// Set default retry values if not provided
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 1 * time.Second
	}
	if config.RetryIncrement == 0 {
		config.RetryIncrement = 2 * time.Second
	}

	return &nodeAdapter{
		config:   config,
		mu:       &sync.Mutex{},
		clients:  make(map[*ssh.Client]bool),
		sessions: make(map[*ssh.Session]bool),
		sftp:     make(map[*sftp.Client]bool),
	}
}

type nodeAdapter struct {
	config   SSHConfig
	mu       *sync.Mutex
	clients  map[*ssh.Client]bool  // Track SSH clients
	sessions map[*ssh.Session]bool // Track SSH sessions
	sftp     map[*sftp.Client]bool // Track SFTP clients
	closed   bool                  // Track if adapter is closed
	cache    cache.Cache           // Lazy-initialized cache instance
}

// withRetry executes the given operation with retry logic
func (a *nodeAdapter) withRetry(operation string, fn func() error) error {
	var lastErr error
	currentDelay := a.config.RetryDelay

	for attempt := 0; attempt <= a.config.MaxRetries; attempt++ {
		if attempt > 0 {
			log.Printf("[NODE] Retry attempt %d/%d for operation '%s' after %v delay",
				attempt, a.config.MaxRetries, operation, currentDelay)
			time.Sleep(currentDelay)
			currentDelay += a.config.RetryIncrement
		}

		err := fn()
		if err == nil {
			if attempt > 0 {
				log.Printf("[NODE] Operation '%s' succeeded after %d retries", operation, attempt)
			}
			return nil
		}

		lastErr = err
		log.Printf("[NODE] Operation '%s' failed (attempt %d/%d): %v",
			operation, attempt+1, a.config.MaxRetries+1, err)
	}

	return fmt.Errorf("operation '%s' failed after %d retries: %w",
		operation, a.config.MaxRetries, lastErr)
}

// FileOperations implements NodeAdapter
func (a *nodeAdapter) FileOperations() cache.Cache {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cache == nil {
		// Create SSH cache with the same configuration
		sshConfig := cache.SSHConfig{
			Host:     a.config.Host,
			Port:     22, // Default SSH port
			User:     a.config.User,
			Password: a.config.Password,
		}
		var err error
		a.cache, err = cache.NewSSHCache(sshConfig, "/tmp/node_cache")
		if err != nil {
			log.Printf("[NODE] Warning: Failed to initialize SSH cache: %v", err)
			return nil
		}
	}

	return a.cache
}

// ExecuteBMCCommand implements NodeAdapter
func (a *nodeAdapter) ExecuteBMCCommand(command string) (stdout string, stderr string, err error) {
	// BMC commands are just special SSH commands, so we can reuse ExecuteCommand
	// We might want to add validation or transformation of commands here
	return a.ExecuteCommand(command)
}

// checkClosed returns an error if the adapter is closed
func (a *nodeAdapter) checkClosed() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return fmt.Errorf("adapter is closed")
	}
	return nil
}

// Close implements NodeAdapter
func (a *nodeAdapter) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.closed {
		return nil
	}

	// Close cache if it exists
	if a.cache != nil {
		if err := a.cache.Close(); err != nil {
			log.Printf("[NODE] Warning: Error closing cache: %v", err)
		}
	}

	// Close all SFTP clients
	for client := range a.sftp {
		client.Close()
		delete(a.sftp, client)
	}

	// Close all sessions
	for session := range a.sessions {
		session.Close()
		delete(a.sessions, session)
	}

	// Close all clients
	for client := range a.clients {
		client.Close()
		delete(a.clients, client)
	}

	a.closed = true
	return nil
}
