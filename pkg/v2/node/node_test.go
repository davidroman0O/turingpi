package node

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/davidroman0O/turingpi/pkg/v2/bmc"
)

// mockBMC implements bmc.BMC for testing
type mockBMC struct {
	powerStatus map[int]*bmc.PowerStatus
	errors      map[string]error
}

func newMockBMC() *mockBMC {
	return &mockBMC{
		powerStatus: make(map[int]*bmc.PowerStatus),
		errors:      make(map[string]error),
	}
}

func (m *mockBMC) GetPowerStatus(ctx context.Context, nodeID int) (*bmc.PowerStatus, error) {
	if err := m.errors["GetPowerStatus"]; err != nil {
		return nil, err
	}
	if status, ok := m.powerStatus[nodeID]; ok {
		return status, nil
	}
	return &bmc.PowerStatus{NodeID: nodeID, State: bmc.PowerStateUnknown}, nil
}

func (m *mockBMC) PowerOn(ctx context.Context, nodeID int) error {
	if err := m.errors["PowerOn"]; err != nil {
		return err
	}
	m.powerStatus[nodeID] = &bmc.PowerStatus{NodeID: nodeID, State: bmc.PowerStateOn}
	return nil
}

func (m *mockBMC) PowerOff(ctx context.Context, nodeID int) error {
	if err := m.errors["PowerOff"]; err != nil {
		return err
	}
	m.powerStatus[nodeID] = &bmc.PowerStatus{NodeID: nodeID, State: bmc.PowerStateOff}
	return nil
}

func (m *mockBMC) Reset(ctx context.Context, nodeID int) error {
	if err := m.errors["Reset"]; err != nil {
		return err
	}
	m.powerStatus[nodeID] = &bmc.PowerStatus{NodeID: nodeID, State: bmc.PowerStateOn}
	return nil
}

func (m *mockBMC) GetInfo(ctx context.Context) (*bmc.BMCInfo, error) {
	if err := m.errors["GetInfo"]; err != nil {
		return nil, err
	}
	return &bmc.BMCInfo{Version: "test"}, nil
}

func (m *mockBMC) Reboot(ctx context.Context) error {
	return m.errors["Reboot"]
}

func (m *mockBMC) UpdateFirmware(ctx context.Context, path string) error {
	return m.errors["UpdateFirmware"]
}

func (m *mockBMC) ExecuteCommand(ctx context.Context, cmd string) (string, string, error) {
	if err := m.errors["ExecuteCommand"]; err != nil {
		return "", "", err
	}
	return "test output", "", nil
}

func TestNewNode(t *testing.T) {
	config := &SSHConfig{
		Host:           "test.host",
		User:           "test",
		Password:       "test",
		Timeout:        5 * time.Second,
		MaxRetries:     3,
		RetryDelay:     time.Second,
		RetryIncrement: time.Second,
	}
	mockBMC := newMockBMC()
	node := NewNode(1, config, mockBMC)

	if node == nil {
		t.Fatal("Expected non-nil node")
	}

	// Test type assertion
	if _, ok := node.(*nodeImpl); !ok {
		t.Error("Expected node to be *nodeImpl")
	}
}

func TestNodePowerOperations(t *testing.T) {
	config := &SSHConfig{
		Host:           "test.host",
		User:           "test",
		Password:       "test",
		Timeout:        5 * time.Second,
		MaxRetries:     3,
		RetryDelay:     time.Second,
		RetryIncrement: time.Second,
	}
	mockBMC := newMockBMC()
	node := NewNode(1, config, mockBMC)

	ctx := context.Background()

	// Test power on
	if err := node.PowerOn(ctx); err != nil {
		t.Errorf("PowerOn failed: %v", err)
	}
	status, err := node.GetPowerStatus(ctx)
	if err != nil {
		t.Errorf("GetPowerStatus failed: %v", err)
	}
	if status.State != bmc.PowerStateOn {
		t.Errorf("Expected power state On, got %s", status.State)
	}

	// Test power off
	if err := node.PowerOff(ctx); err != nil {
		t.Errorf("PowerOff failed: %v", err)
	}
	status, err = node.GetPowerStatus(ctx)
	if err != nil {
		t.Errorf("GetPowerStatus failed: %v", err)
	}
	if status.State != bmc.PowerStateOff {
		t.Errorf("Expected power state Off, got %s", status.State)
	}

	// Test reset
	if err := node.Reset(ctx); err != nil {
		t.Errorf("Reset failed: %v", err)
	}
	status, err = node.GetPowerStatus(ctx)
	if err != nil {
		t.Errorf("GetPowerStatus failed: %v", err)
	}
	if status.State != bmc.PowerStateOn {
		t.Errorf("Expected power state On after reset, got %s", status.State)
	}
}

func TestNodeGetInfo(t *testing.T) {
	config := &SSHConfig{
		Host:           "test.host",
		User:           "test",
		Password:       "test",
		Timeout:        5 * time.Second,
		MaxRetries:     3,
		RetryDelay:     time.Second,
		RetryIncrement: time.Second,
	}
	mockBMC := newMockBMC()
	node := NewNode(1, config, mockBMC)

	ctx := context.Background()

	_, err := node.GetInfo(ctx)
	if err == nil {
		// We expect an error since we can't actually SSH to the test host
		t.Error("Expected error for GetInfo on non-existent host")
	}

	// The error should mention SSH or connection
	if !strings.Contains(strings.ToLower(err.Error()), "ssh") &&
		!strings.Contains(strings.ToLower(err.Error()), "connection") {
		t.Errorf("Expected SSH/connection error, got: %v", err)
	}
}

func TestNodeClose(t *testing.T) {
	config := &SSHConfig{
		Host:           "test.host",
		User:           "test",
		Password:       "test",
		Timeout:        5 * time.Second,
		MaxRetries:     3,
		RetryDelay:     time.Second,
		RetryIncrement: time.Second,
	}
	mockBMC := newMockBMC()
	node := NewNode(1, config, mockBMC)

	// Close should not error even if we haven't connected
	if err := node.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	// Second close should also not error
	if err := node.Close(); err != nil {
		t.Errorf("Second close failed: %v", err)
	}
}

func TestNodeCopyFile(t *testing.T) {
	config := &SSHConfig{
		Host:           "test.host",
		User:           "test",
		Password:       "test",
		Timeout:        5 * time.Second,
		MaxRetries:     3,
		RetryDelay:     time.Second,
		RetryIncrement: time.Second,
	}
	mockBMC := newMockBMC()
	node := NewNode(1, config, mockBMC)

	ctx := context.Background()

	// Create a test file
	tempDir := t.TempDir()
	localFile := filepath.Join(tempDir, "test.txt")
	if err := os.WriteFile(localFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test copying to node (should fail since we can't actually connect)
	err := node.CopyFile(ctx, localFile, "/remote/test.txt", true)
	if err == nil {
		t.Error("Expected error for CopyFile to non-existent host")
	}

	// The error should mention SSH or connection
	if !strings.Contains(strings.ToLower(err.Error()), "ssh") &&
		!strings.Contains(strings.ToLower(err.Error()), "connection") {
		t.Errorf("Expected SSH/connection error, got: %v", err)
	}
}

func TestNodeExpectAndSend(t *testing.T) {
	config := &SSHConfig{
		Host:           "test.host",
		User:           "test",
		Password:       "test",
		Timeout:        5 * time.Second,
		MaxRetries:     3,
		RetryDelay:     time.Second,
		RetryIncrement: time.Second,
	}
	mockBMC := newMockBMC()
	node := NewNode(1, config, mockBMC)

	ctx := context.Background()

	steps := []InteractionStep{
		{
			Expect: "login:",
			Send:   "test",
			LogMsg: "Sending username",
		},
		{
			Expect: "password:",
			Send:   "test",
			LogMsg: "Sending password",
		},
	}

	// Test expect/send (should fail since we can't actually connect)
	_, err := node.ExpectAndSend(ctx, steps, 5*time.Second)
	if err == nil {
		t.Error("Expected error for ExpectAndSend to non-existent host")
	}

	// The error should mention SSH or connection
	if !strings.Contains(strings.ToLower(err.Error()), "ssh") &&
		!strings.Contains(strings.ToLower(err.Error()), "connection") {
		t.Errorf("Expected SSH/connection error, got: %v", err)
	}
}

func TestNodeRetryLogic(t *testing.T) {
	config := &SSHConfig{
		Host:           "test.host",
		User:           "test",
		Password:       "test",
		Timeout:        1 * time.Second, // Short timeout
		MaxRetries:     2,               // Few retries
		RetryDelay:     100 * time.Millisecond,
		RetryIncrement: 100 * time.Millisecond,
	}
	mockBMC := newMockBMC()
	mockBMC.errors["GetPowerStatus"] = fmt.Errorf("test error")

	node := NewNode(1, config, mockBMC)
	ctx := context.Background()

	start := time.Now()
	_, err := node.GetPowerStatus(ctx)
	duration := time.Since(start)

	if err == nil {
		t.Error("Expected error from GetPowerStatus")
	}

	// Should have tried 3 times (initial + 2 retries)
	expectedMinDuration := 300 * time.Millisecond // Initial + 2 retries with 100ms delay each
	if duration < expectedMinDuration {
		t.Errorf("Retry duration too short. Got %v, expected at least %v", duration, expectedMinDuration)
	}
}
