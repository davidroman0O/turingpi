package node

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

// SSHConfig holds the configuration for SSH connections to nodes
type SSHConfig struct {
	Host     string
	User     string
	Password string
	Timeout  time.Duration
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
	ExecuteCommand(command string) (stdout string, stderr string, err error)

	// ExpectAndSend performs a sequence of expect/send interactions over an SSH session
	ExpectAndSend(steps []InteractionStep, interactionTimeout time.Duration) (string, error)

	// CopyFile transfers a file between the local machine and the remote node using SFTP
	CopyFile(localPath, remotePath string, toRemote bool) error

	// Close closes any open connections and frees resources
	Close() error
}

// NewNodeAdapter creates a new instance of NodeAdapter
func NewNodeAdapter(config SSHConfig) NodeAdapter {
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
		return fmt.Errorf("adapter already closed")
	}

	// Close all SFTP clients
	for client := range a.sftp {
		if err := client.Close(); err != nil {
			// Log error but continue closing others
			log.Printf("Error closing SFTP client: %v", err)
		}
		delete(a.sftp, client)
	}

	// Close all SSH sessions
	for session := range a.sessions {
		session.Close() // SSH session Close() doesn't return error
		delete(a.sessions, session)
	}

	// Close all SSH clients
	for client := range a.clients {
		if err := client.Close(); err != nil {
			// Log error but continue closing others
			log.Printf("Error closing SSH client: %v", err)
		}
		delete(a.clients, client)
	}

	a.closed = true
	return nil
}
