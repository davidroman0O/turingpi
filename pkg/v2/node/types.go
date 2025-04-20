package node

import "time"

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

// NodeInfo represents detailed information about a node
type NodeInfo struct {
	ID       int
	Name     string
	Status   string
	Hardware HardwareInfo
	Network  NetworkInfo
}

// HardwareInfo represents hardware-specific information
type HardwareInfo struct {
	Model     string
	CPU       string
	Memory    string
	Storage   string
	BoardType string
}

// NetworkInfo represents network configuration information
type NetworkInfo struct {
	IPAddress  string
	MACAddress string
	Gateway    string
	DNS        []string
	Hostname   string
}
