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

// NodeMode represents the mode of operation for a node
type NodeMode string

const (
	// Normal mode is the default operating mode
	NodeModeNormal NodeMode = "normal"
	// Mass Storage Device mode
	NodeModeMSD NodeMode = "msd"
)

// USBConfig represents the USB configuration
type USBConfig struct {
	// NodeID is the node that the USB bus is connected to
	// If 0, USB is not routed to any node
	NodeID int
	// Host indicates if the node is in host mode
	Host bool
}

// BMC defines the interface for interacting with the Board Management Controller
type BMC interface {
	// Power Operations

	// GetPowerStatus retrieves the power status of a specific node
	GetPowerStatus(ctx context.Context, nodeID int) (*PowerStatus, error)

	// PowerOn turns on a specific node
	PowerOn(ctx context.Context, nodeID int) error

	// PowerOn turns on all nodes
	PowerOnAll(ctx context.Context) error

	// PowerOff turns off a specific node
	PowerOff(ctx context.Context, nodeID int) error

	// PowerOff turns off all nodes
	PowerOffAll(ctx context.Context) error

	// Reset performs a hard reset on a specific node
	Reset(ctx context.Context, nodeID int) error

	// Reset performs a hard reset on all nodes
	ResetAll(ctx context.Context) error

	// BMC Operations

	// GetInfo retrieves information about the BMC
	GetInfo(ctx context.Context) (*BMCInfo, error)

	// Reboot reboots the BMC chip
	Reboot(ctx context.Context) error

	// UpdateFirmware updates the BMC firmware
	UpdateFirmware(ctx context.Context, firmwarePath string) error

	// USB Operations

	// GetUSBConfig retrieves the current USB routing configuration
	GetUSBConfig(ctx context.Context) (*USBConfig, error)

	// SetUSBConfig sets the USB routing to a specific node
	// nodeID: the node to route USB to (1-4), or 0 to disconnect all
	// host: true for host mode, false for device mode
	SetUSBConfig(ctx context.Context, nodeID int, host bool) error

	// Ethernet Operations

	// ResetEthSwitch resets the on-board Ethernet switch
	ResetEthSwitch(ctx context.Context) error

	// Node Mode Operations

	// SetNodeMode sets a node to a specific operating mode
	// nodeID: the node to set (1-4)
	// mode: the mode to set (normal, msd)
	SetNodeMode(ctx context.Context, nodeID int, mode NodeMode) error

	// Flash Operations

	// FlashNode flashes a node with an image
	// nodeID: the node to flash (1-4)
	// imagePath: path to the image file on the BMC filesystem
	FlashNode(ctx context.Context, nodeID int, imagePath string) error

	// UART Operations

	// GetUARTOutput retrieves the UART output from a specific node
	GetUARTOutput(ctx context.Context, nodeID int) (string, error)

	// SendUARTInput sends input to a specific node via UART
	SendUARTInput(ctx context.Context, nodeID int, input string) error

	// ExpectAndSend performs an interactive session with a node via UART
	// nodeID is the node to interact with (1-4)
	// steps is the sequence of expect-and-send steps to perform
	// timeout is the maximum time to wait for each expected string
	ExpectAndSend(ctx context.Context, nodeID int, steps []InteractionStep, timeout time.Duration) (string, error)

	// File Operations

	// UploadFile uploads a file from the local filesystem to the BMC
	// localPath is the path to the local file
	// remotePath is the destination path on the BMC
	UploadFile(ctx context.Context, localPath, remotePath string) error

	// Generic Command Execution

	// ExecuteCommand executes a BMC-specific command
	ExecuteCommand(ctx context.Context, command string) (stdout string, stderr string, err error)
}
