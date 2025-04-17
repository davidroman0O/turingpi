// Package bmc provides functionality for interacting with the Turing Pi Board Management Controller (BMC).
package bmc

import (
	"time"
)

// SSHConfig holds parameters for connecting to the BMC.
type SSHConfig struct {
	Host     string        // BMC IP address or hostname
	User     string        // SSH username
	Password string        // SSH password
	Timeout  time.Duration // Connection timeout
}

// BMCAdapter defines the interface for interacting with the BMC.
type BMCAdapter interface {
	// ExecuteCommand runs a command on the BMC via SSH.
	ExecuteCommand(command string) (stdout string, stderr string, err error)

	// CheckFileExists verifies if a file exists on the BMC.
	CheckFileExists(remotePath string) (bool, error)

	// UploadFile transfers a local file to the BMC.
	UploadFile(localPath, remotePath string) error
}

// NewBMCAdapter creates a new BMC adapter instance.
func NewBMCAdapter(config SSHConfig) BMCAdapter {
	return &bmcAdapter{
		config: config,
	}
}

// bmcAdapter implements the BMCAdapter interface.
type bmcAdapter struct {
	config SSHConfig
}
