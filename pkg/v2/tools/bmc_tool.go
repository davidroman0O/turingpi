package tools

import (
	"context"

	"github.com/davidroman0O/turingpi/pkg/v2/bmc"
)

// BMCToolImpl is the implementation of the BMCTool interface
type BMCToolImpl struct {
	client bmc.BMC
}

// NewBMCTool creates a new BMCTool
func NewBMCTool(executor bmc.CommandExecutor) BMCTool {
	client := bmc.New(executor)
	return &BMCToolImpl{client: client}
}

// GetPowerStatus retrieves the power status of a specific node
func (t *BMCToolImpl) GetPowerStatus(ctx context.Context, nodeID int) (*bmc.PowerStatus, error) {
	return t.client.GetPowerStatus(ctx, nodeID)
}

// PowerOn turns on a specific node
func (t *BMCToolImpl) PowerOn(ctx context.Context, nodeID int) error {
	return t.client.PowerOn(ctx, nodeID)
}

// PowerOff turns off a specific node
func (t *BMCToolImpl) PowerOff(ctx context.Context, nodeID int) error {
	return t.client.PowerOff(ctx, nodeID)
}

// Reset performs a hard reset on a specific node
func (t *BMCToolImpl) Reset(ctx context.Context, nodeID int) error {
	return t.client.Reset(ctx, nodeID)
}

// GetInfo retrieves information about the BMC
func (t *BMCToolImpl) GetInfo(ctx context.Context) (*bmc.BMCInfo, error) {
	return t.client.GetInfo(ctx)
}

// Reboot reboots the BMC chip
func (t *BMCToolImpl) Reboot(ctx context.Context) error {
	return t.client.Reboot(ctx)
}

// UpdateFirmware updates the BMC firmware
func (t *BMCToolImpl) UpdateFirmware(ctx context.Context, firmwarePath string) error {
	return t.client.UpdateFirmware(ctx, firmwarePath)
}

// ExecuteCommand executes a BMC-specific command
func (t *BMCToolImpl) ExecuteCommand(ctx context.Context, command string) (stdout string, stderr string, err error) {
	return t.client.ExecuteCommand(ctx, command)
}
