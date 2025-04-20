package bmc

import (
	"context"
	"fmt"
	"strings"
)

// bmcImpl implements the BMC interface
type bmcImpl struct {
	executor CommandExecutor
}

// CommandExecutor defines the interface for executing commands
type CommandExecutor interface {
	ExecuteCommand(command string) (stdout string, stderr string, err error)
}

// New creates a new BMC instance
func New(executor CommandExecutor) BMC {
	return &bmcImpl{
		executor: executor,
	}
}

// GetPowerStatus implements BMC interface
func (b *bmcImpl) GetPowerStatus(ctx context.Context, nodeID int) (*PowerStatus, error) {
	stdout, stderr, err := b.executor.ExecuteCommand("tpi power status")
	if err != nil {
		return nil, fmt.Errorf("failed to get power status: %w (stderr: %s)", err, stderr)
	}

	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, fmt.Sprintf("node%d:", nodeID)) {
			parts := strings.Split(line, ":")
			if len(parts) != 2 {
				return nil, fmt.Errorf("unexpected power status format: %s", line)
			}
			state := strings.TrimSpace(parts[1])
			// Normalize state
			switch strings.ToLower(state) {
			case "on":
				state = string(PowerStateOn)
			case "off":
				state = string(PowerStateOff)
			default:
				state = string(PowerStateUnknown)
			}
			return &PowerStatus{
				NodeID: nodeID,
				State:  PowerState(state),
			}, nil
		}
	}

	return nil, fmt.Errorf("power status not found for node %d", nodeID)
}

// PowerOn implements BMC interface
func (b *bmcImpl) PowerOn(ctx context.Context, nodeID int) error {
	_, stderr, err := b.executor.ExecuteCommand(fmt.Sprintf("tpi power on --node %d", nodeID))
	if err != nil {
		return fmt.Errorf("failed to power on node %d: %w (stderr: %s)", nodeID, err, stderr)
	}
	return nil
}

// PowerOff implements BMC interface
func (b *bmcImpl) PowerOff(ctx context.Context, nodeID int) error {
	_, stderr, err := b.executor.ExecuteCommand(fmt.Sprintf("tpi power off --node %d", nodeID))
	if err != nil {
		return fmt.Errorf("failed to power off node %d: %w (stderr: %s)", nodeID, err, stderr)
	}
	return nil
}

// Reset implements BMC interface
func (b *bmcImpl) Reset(ctx context.Context, nodeID int) error {
	_, stderr, err := b.executor.ExecuteCommand(fmt.Sprintf("tpi power reset --node %d", nodeID))
	if err != nil {
		return fmt.Errorf("failed to reset node %d: %w (stderr: %s)", nodeID, err, stderr)
	}
	return nil
}

// GetInfo implements BMC interface
func (b *bmcImpl) GetInfo(ctx context.Context) (*BMCInfo, error) {
	stdout, stderr, err := b.executor.ExecuteCommand("tpi info")
	if err != nil {
		return nil, fmt.Errorf("failed to get BMC info: %w (stderr: %s)", err, stderr)
	}

	info := &BMCInfo{}
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "|") || line == "" {
			continue
		}
		parts := strings.Split(line, ":")
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		value = strings.Trim(value, "\"") // Remove quotes if present

		switch key {
		case "api":
			info.APIVersion = value
		case "build_version":
			info.BuildVersion = value
		case "buildroot":
			info.Buildroot = value
		case "buildtime":
			info.BuildTime = value
		case "ip":
			info.IPAddress = value
		case "mac":
			info.MACAddress = value
		case "version":
			info.Version = value
		}
	}

	return info, nil
}

// Reboot implements BMC interface
func (b *bmcImpl) Reboot(ctx context.Context) error {
	_, stderr, err := b.executor.ExecuteCommand("tpi reboot")
	if err != nil {
		return fmt.Errorf("failed to reboot BMC: %w (stderr: %s)", err, stderr)
	}
	return nil
}

// UpdateFirmware implements BMC interface
func (b *bmcImpl) UpdateFirmware(ctx context.Context, firmwarePath string) error {
	_, stderr, err := b.executor.ExecuteCommand(fmt.Sprintf("tpi firmware upgrade %s", firmwarePath))
	if err != nil {
		return fmt.Errorf("failed to update BMC firmware: %w (stderr: %s)", err, stderr)
	}
	return nil
}

// ExecuteCommand implements BMC interface
func (b *bmcImpl) ExecuteCommand(ctx context.Context, command string) (string, string, error) {
	return b.executor.ExecuteCommand(command)
}
