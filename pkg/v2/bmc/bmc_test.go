package bmc

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// mockExecutor implements CommandExecutor for testing
type mockExecutor struct {
	responses map[string]struct {
		stdout string
		stderr string
		err    error
	}
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		responses: make(map[string]struct {
			stdout string
			stderr string
			err    error
		}),
	}
}

func (m *mockExecutor) ExecuteCommand(command string) (string, string, error) {
	// Try exact match first
	if response, ok := m.responses[command]; ok {
		return response.stdout, response.stderr, response.err
	}

	// Try prefix match for commands with variable parts
	for cmd, response := range m.responses {
		if strings.HasPrefix(command, cmd) {
			return response.stdout, response.stderr, response.err
		}
	}

	return "", "", fmt.Errorf("no mock response for command: %s", command)
}

func (m *mockExecutor) addResponse(command string, stdout, stderr string, err error) {
	m.responses[command] = struct {
		stdout string
		stderr string
		err    error
	}{stdout, stderr, err}
}

func TestBMC_GetPowerStatus(t *testing.T) {
	tests := []struct {
		name     string
		nodeID   int
		stdout   string
		stderr   string
		err      error
		want     *PowerStatus
		wantErr  bool
		errMatch string
	}{
		{
			name:   "node is on",
			nodeID: 1,
			stdout: "node1: On\nnode2: Off\n",
			want:   &PowerStatus{NodeID: 1, State: PowerStateOn},
		},
		{
			name:   "node is off",
			nodeID: 2,
			stdout: "node1: On\nnode2: Off\n",
			want:   &PowerStatus{NodeID: 2, State: PowerStateOff},
		},
		{
			name:     "command error",
			nodeID:   1,
			err:      fmt.Errorf("command failed"),
			stderr:   "error output",
			wantErr:  true,
			errMatch: "failed to get power status",
		},
		{
			name:     "invalid output format",
			nodeID:   1,
			stdout:   "invalid format",
			wantErr:  true,
			errMatch: "power status not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := newMockExecutor()
			executor.addResponse("tpi power status", tt.stdout, tt.stderr, tt.err)

			bmc := New(executor)
			got, err := bmc.GetPowerStatus(context.Background(), tt.nodeID)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMatch)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got.NodeID != tt.want.NodeID {
				t.Errorf("NodeID = %v, want %v", got.NodeID, tt.want.NodeID)
			}
			if got.State != tt.want.State {
				t.Errorf("State = %v, want %v", got.State, tt.want.State)
			}
		})
	}
}

func TestBMC_GetInfo(t *testing.T) {
	tests := []struct {
		name     string
		stdout   string
		stderr   string
		err      error
		want     *BMCInfo
		wantErr  bool
		errMatch string
	}{
		{
			name: "successful info retrieval",
			stdout: `api: 1.0
build_version: v1.2.3
buildroot: 2023.02
buildtime: 2023-01-01
ip: 192.168.1.100
mac: 00:11:22:33:44:55
version: 2.0.0`,
			want: &BMCInfo{
				APIVersion:   "1.0",
				BuildVersion: "v1.2.3",
				Buildroot:    "2023.02",
				BuildTime:    "2023-01-01",
				IPAddress:    "192.168.1.100",
				MACAddress:   "00:11:22:33:44:55",
				Version:      "2.0.0",
			},
		},
		{
			name:     "command error",
			err:      fmt.Errorf("command failed"),
			stderr:   "error output",
			wantErr:  true,
			errMatch: "failed to get BMC info",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := newMockExecutor()
			executor.addResponse("tpi info", tt.stdout, tt.stderr, tt.err)

			bmc := New(executor)
			got, err := bmc.GetInfo(context.Background())

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMatch)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got.APIVersion != tt.want.APIVersion {
				t.Errorf("APIVersion = %v, want %v", got.APIVersion, tt.want.APIVersion)
			}
			if got.BuildVersion != tt.want.BuildVersion {
				t.Errorf("BuildVersion = %v, want %v", got.BuildVersion, tt.want.BuildVersion)
			}
			if got.Version != tt.want.Version {
				t.Errorf("Version = %v, want %v", got.Version, tt.want.Version)
			}
		})
	}
}

func TestBMC_PowerOperations(t *testing.T) {
	tests := []struct {
		name     string
		op       string
		nodeID   int
		err      error
		stderr   string
		wantErr  bool
		errMatch string
	}{
		{
			name:   "power on success",
			op:     "on",
			nodeID: 1,
		},
		{
			name:     "power on error",
			op:       "on",
			nodeID:   1,
			err:      fmt.Errorf("command failed"),
			stderr:   "error output",
			wantErr:  true,
			errMatch: "failed to power on",
		},
		{
			name:   "power off success",
			op:     "off",
			nodeID: 1,
		},
		{
			name:     "power off error",
			op:       "off",
			nodeID:   1,
			err:      fmt.Errorf("command failed"),
			stderr:   "error output",
			wantErr:  true,
			errMatch: "failed to power off",
		},
		{
			name:   "reset success",
			op:     "reset",
			nodeID: 1,
		},
		{
			name:     "reset error",
			op:       "reset",
			nodeID:   1,
			err:      fmt.Errorf("command failed"),
			stderr:   "error output",
			wantErr:  true,
			errMatch: "failed to reset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := newMockExecutor()
			command := fmt.Sprintf("tpi power %s --node %d", tt.op, tt.nodeID)
			executor.addResponse(command, "", tt.stderr, tt.err)

			bmc := New(executor)
			var err error

			switch tt.op {
			case "on":
				err = bmc.PowerOn(context.Background(), tt.nodeID)
			case "off":
				err = bmc.PowerOff(context.Background(), tt.nodeID)
			case "reset":
				err = bmc.Reset(context.Background(), tt.nodeID)
			}

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.errMatch)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
