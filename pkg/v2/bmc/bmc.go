package bmc

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
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

// NewWithSSH creates a new BMC instance that connects to a Turing Pi cluster via SSH
// configPath is the path to the SSH configuration JSON file
func NewWithSSH(configPath string) (BMC, error) {
	executor, err := NewSSHExecutorFromConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create SSH executor: %w", err)
	}

	return New(executor), nil
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

// ExpectAndSend implements BMC interface
func (b *bmcImpl) ExpectAndSend(ctx context.Context, nodeID int, steps []InteractionStep, timeout time.Duration) (string, error) {
	if nodeID < 1 || nodeID > 4 {
		return "", fmt.Errorf("invalid node ID: %d (must be 1-4)", nodeID)
	}

	// Create a buffer to capture all output
	var outputBuffer bytes.Buffer

	// Process each step in sequence
	for i, step := range steps {
		select {
		case <-ctx.Done():
			return outputBuffer.String(), ctx.Err()
		default:
			// Continue with the step
		}

		log.Printf("[BMC UART] Step %d: Expecting '%s'", i+1, step.Expect)

		// If there's something to expect, wait for it
		if step.Expect != "" {
			if err := b.waitForUARTOutput(ctx, nodeID, &outputBuffer, step.Expect, timeout); err != nil {
				log.Printf("[BMC UART] Output before error/timeout:\n%s", outputBuffer.String())
				return outputBuffer.String(), fmt.Errorf("error waiting for '%s': %w", step.Expect, err)
			}
		}

		// If there's something to send, send it
		if step.Send != "" {
			log.Printf("[BMC UART] Step %d: %s", i+1, step.LogMsg)

			// Ensure the send data ends with a newline for proper command execution
			sendData := step.Send
			if !strings.HasSuffix(sendData, "\n") {
				sendData += "\n"
			}

			// Send the data via UART
			err := b.sendUARTData(ctx, nodeID, sendData)
			if err != nil {
				return outputBuffer.String(), fmt.Errorf("failed to send '%s': %w", step.Send, err)
			}

			// Add a small delay to ensure the command has time to be processed
			time.Sleep(100 * time.Millisecond)
		}
	}

	// Get any remaining output
	if err := b.captureRemainingOutput(ctx, nodeID, &outputBuffer, 500*time.Millisecond); err != nil {
		log.Printf("[BMC UART] Warning: error capturing final output: %v", err)
	}

	finalOutput := outputBuffer.String()
	log.Printf("[BMC UART] Final output snippet:\n%s", getLastLines(finalOutput, 10))
	log.Println("[BMC UART] Interaction sequence finished.")

	return finalOutput, nil
}

// waitForUARTOutput repeatedly gets UART output and checks for the expected string
func (b *bmcImpl) waitForUARTOutput(ctx context.Context, nodeID int, buffer *bytes.Buffer, expect string, timeout time.Duration) error {
	if expect == "" {
		return nil // Nothing to expect, return immediately
	}

	startTime := time.Now()
	deadline := startTime.Add(timeout)

	// Create a small buffer to compare with
	var searchBuffer bytes.Buffer

	// Keep checking for the expected string until timeout
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue with the check
		}

		// Get the current UART output
		stdout, stderr, err := b.executor.ExecuteCommand(fmt.Sprintf("tpi uart --node %d get", nodeID))
		if err != nil {
			return fmt.Errorf("failed to get UART output: %w (stderr: %s)", err, stderr)
		}

		// Write to both our buffers
		buffer.WriteString(stdout)
		searchBuffer.WriteString(stdout)

		// Check if the expected string is in the output
		if strings.Contains(searchBuffer.String(), expect) {
			return nil // Found the expected string
		}

		// Don't let searchBuffer grow too large - keep last 8KB which should be
		// enough to match multi-line patterns while preventing memory issues
		if searchBuffer.Len() > 8192 {
			data := searchBuffer.Bytes()
			searchBuffer.Reset()
			searchBuffer.Write(data[len(data)-4096:])
		}

		// Sleep a short duration before checking again
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("timeout waiting for '%s'", expect)
}

// sendUARTData sends data to the node via UART
func (b *bmcImpl) sendUARTData(ctx context.Context, nodeID int, data string) error {
	escapedData := strings.ReplaceAll(data, "\"", "\\\"")
	cmd := fmt.Sprintf("tpi uart --node %d set -c \"%s\"", nodeID, escapedData)
	_, stderr, err := b.executor.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to send UART data: %w (stderr: %s)", err, stderr)
	}
	return nil
}

// captureRemainingOutput captures any remaining output from UART
func (b *bmcImpl) captureRemainingOutput(ctx context.Context, nodeID int, buffer *bytes.Buffer, duration time.Duration) error {
	endTime := time.Now().Add(duration)

	// Keep capturing for the specified duration
	for time.Now().Before(endTime) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			// Continue with capturing
		}

		stdout, _, err := b.executor.ExecuteCommand(fmt.Sprintf("tpi uart --node %d get", nodeID))
		if err != nil {
			return err
		}

		if stdout != "" {
			buffer.WriteString(stdout)
		}

		// Sleep a short duration before capturing again
		time.Sleep(100 * time.Millisecond)
	}

	return nil
}

// getLastLines returns the last n lines of a string
func getLastLines(s string, n int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}

// PowerOnAll implements BMC interface
func (b *bmcImpl) PowerOnAll(ctx context.Context) error {
	_, stderr, err := b.executor.ExecuteCommand("tpi power on")
	if err != nil {
		return fmt.Errorf("failed to power on all nodes: %w (stderr: %s)", err, stderr)
	}
	return nil
}

// PowerOffAll implements BMC interface
func (b *bmcImpl) PowerOffAll(ctx context.Context) error {
	_, stderr, err := b.executor.ExecuteCommand("tpi power off")
	if err != nil {
		return fmt.Errorf("failed to power off all nodes: %w (stderr: %s)", err, stderr)
	}
	return nil
}

// ResetAll implements BMC interface
func (b *bmcImpl) ResetAll(ctx context.Context) error {
	_, stderr, err := b.executor.ExecuteCommand("tpi power reset")
	if err != nil {
		return fmt.Errorf("failed to reset all nodes: %w (stderr: %s)", err, stderr)
	}
	return nil
}

// GetUSBConfig implements BMC interface
func (b *bmcImpl) GetUSBConfig(ctx context.Context) (*USBConfig, error) {
	stdout, stderr, err := b.executor.ExecuteCommand("tpi usb get")
	if err != nil {
		return nil, fmt.Errorf("failed to get USB configuration: %w (stderr: %s)", err, stderr)
	}

	// Parse the output to determine the node and host/device mode
	config := &USBConfig{
		NodeID: 0, // Default to no node
		Host:   false,
	}

	// Example output: "USB routed to node 1 in host mode"
	// or "USB routed to node 2 in device mode"
	// or "USB is not routed to any node"
	stdout = strings.TrimSpace(stdout)
	if strings.Contains(stdout, "not routed") {
		return config, nil
	}

	// Extract node ID
	nodeMatch := regexp.MustCompile(`node (\d+)`).FindStringSubmatch(stdout)
	if len(nodeMatch) >= 2 {
		nodeID, err := strconv.Atoi(nodeMatch[1])
		if err == nil && nodeID >= 1 && nodeID <= 4 {
			config.NodeID = nodeID
		}
	}

	// Determine host/device mode
	config.Host = strings.Contains(stdout, "host mode")

	return config, nil
}

// SetUSBConfig implements BMC interface
func (b *bmcImpl) SetUSBConfig(ctx context.Context, nodeID int, host bool) error {
	if nodeID < 0 || nodeID > 4 {
		return fmt.Errorf("invalid node ID: %d (must be 0-4)", nodeID)
	}

	var cmd string
	if nodeID == 0 {
		// Disconnect USB
		cmd = "tpi usb disconnect"
	} else {
		// Connect USB to the specified node in the specified mode
		mode := "device"
		if host {
			mode = "host"
		}
		cmd = fmt.Sprintf("tpi usb --node %d %s", nodeID, mode)
	}

	_, stderr, err := b.executor.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to set USB configuration: %w (stderr: %s)", err, stderr)
	}
	return nil
}

// ResetEthSwitch implements BMC interface
func (b *bmcImpl) ResetEthSwitch(ctx context.Context) error {
	_, stderr, err := b.executor.ExecuteCommand("tpi eth reset")
	if err != nil {
		return fmt.Errorf("failed to reset Ethernet switch: %w (stderr: %s)", err, stderr)
	}
	return nil
}

// SetNodeMode implements BMC interface
func (b *bmcImpl) SetNodeMode(ctx context.Context, nodeID int, mode NodeMode) error {
	if nodeID < 1 || nodeID > 4 {
		return fmt.Errorf("invalid node ID: %d (must be 1-4)", nodeID)
	}

	if mode != NodeModeNormal && mode != NodeModeMSD {
		return fmt.Errorf("invalid node mode: %s (must be normal or msd)", mode)
	}

	cmd := fmt.Sprintf("tpi advanced --node %d %s", nodeID, mode)
	_, stderr, err := b.executor.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to set node %d to mode %s: %w (stderr: %s)", nodeID, mode, err, stderr)
	}
	return nil
}

// FlashNode implements BMC interface
func (b *bmcImpl) FlashNode(ctx context.Context, nodeID int, imagePath string) error {
	if nodeID < 1 || nodeID > 4 {
		return fmt.Errorf("invalid node ID: %d (must be 1-4)", nodeID)
	}

	if imagePath == "" {
		return fmt.Errorf("image path cannot be empty")
	}

	cmd := fmt.Sprintf("tpi flash --node %d %s", nodeID, imagePath)
	_, stderr, err := b.executor.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to flash node %d with image %s: %w (stderr: %s)", nodeID, imagePath, err, stderr)
	}
	return nil
}

// GetUARTOutput implements BMC interface
func (b *bmcImpl) GetUARTOutput(ctx context.Context, nodeID int) (string, error) {
	if nodeID < 1 || nodeID > 4 {
		return "", fmt.Errorf("invalid node ID: %d (must be 1-4)", nodeID)
	}

	stdout, stderr, err := b.executor.ExecuteCommand(fmt.Sprintf("tpi uart --node %d get", nodeID))
	if err != nil {
		return "", fmt.Errorf("failed to get UART output from node %d: %w (stderr: %s)", nodeID, err, stderr)
	}
	return stdout, nil
}

// SendUARTInput implements BMC interface
func (b *bmcImpl) SendUARTInput(ctx context.Context, nodeID int, input string) error {
	if nodeID < 1 || nodeID > 4 {
		return fmt.Errorf("invalid node ID: %d (must be 1-4)", nodeID)
	}

	// Escape quotes in the input
	escapedInput := strings.ReplaceAll(input, "\"", "\\\"")
	cmd := fmt.Sprintf("tpi uart --node %d set --cmd \"%s\"", nodeID, escapedInput)
	_, stderr, err := b.executor.ExecuteCommand(cmd)
	if err != nil {
		return fmt.Errorf("failed to send UART input to node %d: %w (stderr: %s)", nodeID, err, stderr)
	}
	return nil
}
