package node

import (
	"context"
	"fmt"
	"strings"
)

// NodePowerState represents the power state of a node
type NodePowerState string

const (
	PowerStateOn      NodePowerState = "On"
	PowerStateOff     NodePowerState = "Off"
	PowerStateUnknown NodePowerState = "Unknown"
)

// PowerStatus represents the power status of a node
type PowerStatus struct {
	NodeID int
	State  NodePowerState
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

// GetPowerStatus retrieves the power status of a specific node
func (a *nodeAdapter) GetPowerStatus(ctx context.Context, nodeID int) (*PowerStatus, error) {
	stdout, stderr, err := a.ExecuteBMCCommand("tpi power status")
	if err != nil {
		return nil, fmt.Errorf("failed to get power status: %w (stderr: %s)", err, stderr)
	}

	// Parse the output which is in format "nodeX: State"
	lines := strings.Split(stdout, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, fmt.Sprintf("node%d:", nodeID)) {
			parts := strings.Split(line, ":")
			if len(parts) != 2 {
				return nil, fmt.Errorf("unexpected power status format: %s", line)
			}
			state := strings.TrimSpace(parts[1])
			// Normalize state to proper case
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
				State:  NodePowerState(state),
			}, nil
		}
	}

	return nil, fmt.Errorf("power status not found for node %d", nodeID)
}

// PowerOn turns on a specific node
func (a *nodeAdapter) PowerOn(ctx context.Context, nodeID int) error {
	_, stderr, err := a.ExecuteBMCCommand(fmt.Sprintf("tpi power on --node %d", nodeID))
	if err != nil {
		return fmt.Errorf("failed to power on node %d: %w (stderr: %s)", nodeID, err, stderr)
	}
	return nil
}

// PowerOff turns off a specific node
func (a *nodeAdapter) PowerOff(ctx context.Context, nodeID int) error {
	_, stderr, err := a.ExecuteBMCCommand(fmt.Sprintf("tpi power off --node %d", nodeID))
	if err != nil {
		return fmt.Errorf("failed to power off node %d: %w (stderr: %s)", nodeID, err, stderr)
	}
	return nil
}

// Reset performs a hard reset on a specific node
func (a *nodeAdapter) Reset(ctx context.Context, nodeID int) error {
	_, stderr, err := a.ExecuteBMCCommand(fmt.Sprintf("tpi power reset --node %d", nodeID))
	if err != nil {
		return fmt.Errorf("failed to reset node %d: %w (stderr: %s)", nodeID, err, stderr)
	}
	return nil
}

// GetBMCInfo retrieves information about the BMC
func (a *nodeAdapter) GetBMCInfo(ctx context.Context) (*BMCInfo, error) {
	stdout, stderr, err := a.ExecuteBMCCommand("tpi info")
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

// RebootBMC reboots the BMC chip
func (a *nodeAdapter) RebootBMC(ctx context.Context) error {
	_, stderr, err := a.ExecuteBMCCommand("tpi reboot")
	if err != nil {
		return fmt.Errorf("failed to reboot BMC: %w (stderr: %s)", err, stderr)
	}
	return nil
}

// UpdateBMCFirmware updates the BMC firmware
func (a *nodeAdapter) UpdateBMCFirmware(ctx context.Context, firmwarePath string) error {
	_, stderr, err := a.ExecuteBMCCommand(fmt.Sprintf("tpi firmware upgrade %s", firmwarePath))
	if err != nil {
		return fmt.Errorf("failed to update BMC firmware: %w (stderr: %s)", err, stderr)
	}
	return nil
}

// GetNodeInfo retrieves detailed information about a specific node
func (a *nodeAdapter) GetNodeInfo(ctx context.Context, nodeID int) (map[string]string, error) {
	_, stderr, err := a.ExecuteBMCCommand(fmt.Sprintf("tpi node info --node %d", nodeID))
	if err != nil {
		return nil, fmt.Errorf("failed to get node info: %w (stderr: %s)", err, stderr)
	}

	info := make(map[string]string)
	// TODO: Parse node info output once we have an example of the actual output format
	return info, nil
}

// UpdateNodeFirmware updates the firmware of a specific node
func (a *nodeAdapter) UpdateNodeFirmware(ctx context.Context, nodeID int, firmwarePath string) error {
	_, stderr, err := a.ExecuteBMCCommand(fmt.Sprintf("tpi node update --node %d --firmware %s", nodeID, firmwarePath))
	if err != nil {
		return fmt.Errorf("failed to update node firmware: %w (stderr: %s)", err, stderr)
	}
	return nil
}
