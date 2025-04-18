package node

import (
	"context"
	"testing"
	"time"
)

func TestNodeAdapter_GetPowerStatus(t *testing.T) {
	adapter := NewNodeAdapter(getTestConfig())
	defer adapter.Close()

	ctx := context.Background()
	nodeID := 1
	status, err := adapter.GetPowerStatus(ctx, nodeID)
	if err != nil {
		t.Fatalf("Failed to get power status: %v", err)
	}

	// Verify state is either On or Off
	if status.State != PowerStateOn && status.State != PowerStateOff {
		t.Errorf("Expected power state to be On or Off, got %s", status.State)
	}
}

func TestNodeAdapter_GetBMCInfo(t *testing.T) {
	adapter := NewNodeAdapter(getTestConfig())
	defer adapter.Close()

	ctx := context.Background()
	info, err := adapter.GetBMCInfo(ctx)
	if err != nil {
		t.Fatalf("Failed to get BMC info: %v", err)
	}

	// Verify we get non-empty values
	if info.APIVersion == "" {
		t.Error("Expected non-empty API version")
	}
	if info.Version == "" {
		t.Error("Expected non-empty version")
	}
	if info.IPAddress == "" {
		t.Error("Expected non-empty IP address")
	}
}

func TestNodeAdapter_PowerOperations(t *testing.T) {
	adapter := NewNodeAdapter(getTestConfig())
	defer adapter.Close()

	ctx := context.Background()
	nodeID := 1

	// Get initial power state
	initialStatus, err := adapter.GetPowerStatus(ctx, nodeID)
	if err != nil {
		t.Fatalf("Failed to get initial power status: %v", err)
	}

	// Test power off
	if err := adapter.PowerOff(ctx, nodeID); err != nil {
		t.Fatalf("Failed to power off node: %v", err)
	}

	// Wait for power state to change
	time.Sleep(2 * time.Second)

	// Verify power is off
	status, err := adapter.GetPowerStatus(ctx, nodeID)
	if err != nil {
		t.Fatalf("Failed to get power status after power off: %v", err)
	}
	if status.State != PowerStateOff {
		t.Errorf("Expected power state Off, got %s", status.State)
	}

	// Test power on
	if err := adapter.PowerOn(ctx, nodeID); err != nil {
		t.Fatalf("Failed to power on node: %v", err)
	}

	// Wait for power state to change
	time.Sleep(2 * time.Second)

	// Verify power is on
	status, err = adapter.GetPowerStatus(ctx, nodeID)
	if err != nil {
		t.Fatalf("Failed to get power status after power on: %v", err)
	}
	if status.State != PowerStateOn {
		t.Errorf("Expected power state On, got %s", status.State)
	}

	// Test reset
	if err := adapter.Reset(ctx, nodeID); err != nil {
		t.Fatalf("Failed to reset node: %v", err)
	}

	// Wait for reset to complete
	time.Sleep(5 * time.Second)

	// Verify power is on after reset
	status, err = adapter.GetPowerStatus(ctx, nodeID)
	if err != nil {
		t.Fatalf("Failed to get power status after reset: %v", err)
	}
	if status.State != PowerStateOn {
		t.Errorf("Expected power state On after reset, got %s", status.State)
	}

	// If initial state was off, restore it
	if initialStatus.State == PowerStateOff {
		if err := adapter.PowerOff(ctx, nodeID); err != nil {
			t.Fatalf("Failed to restore initial power state: %v", err)
		}
	}
}

func TestNodeAdapter_RebootBMC(t *testing.T) {
	adapter := NewNodeAdapter(getTestConfig())
	defer adapter.Close()

	ctx := context.Background()
	err := adapter.RebootBMC(ctx)
	if err != nil {
		t.Fatalf("Failed to reboot BMC: %v", err)
	}

	// After reboot, we should wait a bit before trying to get BMC info
	// to ensure BMC is back online
	time.Sleep(10 * time.Second)

	// Verify BMC is responsive after reboot by getting info
	info, err := adapter.GetBMCInfo(ctx)
	if err != nil {
		t.Fatalf("BMC not responsive after reboot: %v", err)
	}
	if info.Version == "" {
		t.Error("Expected non-empty version after reboot")
	}
}
