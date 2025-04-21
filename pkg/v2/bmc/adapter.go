package bmc

import (
	"context"
	"time"
)

// PowerState represents the power state of a node
type PowerState string

const (
	PowerStateOn      PowerState = "On"
	PowerStateOff     PowerState = "Off"
	PowerStateUnknown PowerState = "Unknown"
)

// PowerStatus represents the power status of a node
type PowerStatus struct {
	NodeID int
	State  PowerState
}

// BMCInfo represents the BMC information
type BMCInfo struct {
	APIVersion   string
	BuildVersion string
	Buildroot    string
	BuildTime    string
	IPAddress    string
	MACAddress   string
	Version      string
}

// InteractionStep represents a single step in an expect-and-send interaction sequence
type InteractionStep struct {
	// Expect is the string to wait for before sending the next command
	Expect string
	// Send is the string to send after Expect is found
	Send string
	// LogMsg is a message to log when this step is performed
	LogMsg string
}

// BMC defines the interface for interacting with the Board Management Controller
type BMC interface {
	// GetPowerStatus retrieves the power status of a specific node
	GetPowerStatus(ctx context.Context, nodeID int) (*PowerStatus, error)

	// PowerOn turns on a specific node
	PowerOn(ctx context.Context, nodeID int) error

	// PowerOff turns off a specific node
	PowerOff(ctx context.Context, nodeID int) error

	// Reset performs a hard reset on a specific node
	Reset(ctx context.Context, nodeID int) error

	// GetInfo retrieves information about the BMC
	GetInfo(ctx context.Context) (*BMCInfo, error)

	// Reboot reboots the BMC chip
	Reboot(ctx context.Context) error

	// UpdateFirmware updates the BMC firmware
	UpdateFirmware(ctx context.Context, firmwarePath string) error

	// ExecuteCommand executes a BMC-specific command
	ExecuteCommand(ctx context.Context, command string) (stdout string, stderr string, err error)

	// ExpectAndSend performs an interactive session with a node via UART
	// nodeID is the node to interact with (1-4)
	// steps is the sequence of expect-and-send steps to perform
	// timeout is the maximum time to wait for each expected string
	ExpectAndSend(ctx context.Context, nodeID int, steps []InteractionStep, timeout time.Duration) (string, error)
}
