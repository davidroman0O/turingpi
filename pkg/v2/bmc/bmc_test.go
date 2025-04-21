package bmc

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
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

	// Special handling for progressive UART get commands in tests
	if strings.HasPrefix(command, "tpi uart --node ") && strings.Contains(command, " get") {
		for cmd, response := range m.responses {
			if strings.HasPrefix(cmd, "tpi uart --node ") &&
				strings.Contains(cmd, " get-") &&
				strings.HasPrefix(command, cmd[:strings.Index(cmd, "-")]) {
				return response.stdout, response.stderr, response.err
			}
		}
	}

	// Try prefix match for commands with variable parts
	for cmd, response := range m.responses {
		if strings.HasPrefix(command, cmd) {
			return response.stdout, response.stderr, response.err
		}
	}

	return "", fmt.Errorf("no mock response for command: %s", command).Error(), fmt.Errorf("no mock response for command: %s", command)
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

// Create a specialized sequence mock executor
type sequenceMockExecutor struct {
	*mockExecutor
	sequence []struct {
		command string
		stdout  string
		stderr  string
		err     error
	}
	index int
}

func newSequenceMockExecutor() *sequenceMockExecutor {
	return &sequenceMockExecutor{
		mockExecutor: newMockExecutor(),
		sequence: []struct {
			command string
			stdout  string
			stderr  string
			err     error
		}{
			// Initial state
			{
				command: "tpi uart --node 2 get",
				stdout:  "Welcome to Node 2\nlogin: ",
				stderr:  "",
				err:     nil,
			},
			// After sending "root"
			{
				command: "tpi uart --node 2 get",
				stdout:  "Welcome to Node 2\nlogin: root\nPassword: ",
				stderr:  "",
				err:     nil,
			},
			// After sending "password123"
			{
				command: "tpi uart --node 2 get",
				stdout:  "Welcome to Node 2\nlogin: root\nPassword: password123\n# ",
				stderr:  "",
				err:     nil,
			},
			// After sending "echo 'test done'"
			{
				command: "tpi uart --node 2 get",
				stdout:  "Welcome to Node 2\nlogin: root\nPassword: password123\n# echo 'test done'\ntest done\n# ",
				stderr:  "",
				err:     nil,
			},
		},
		index: 0,
	}
}

func (s *sequenceMockExecutor) ExecuteCommand(command string) (string, string, error) {
	// For UART get commands, use the sequence
	if command == "tpi uart --node 2 get" && s.index < len(s.sequence) {
		item := s.sequence[s.index]
		s.index++
		return item.stdout, item.stderr, item.err
	}

	// For other commands, use the default implementation
	return s.mockExecutor.ExecuteCommand(command)
}

func TestBMC_ExpectAndSend(t *testing.T) {
	ctx := context.Background()

	// Create a sequence mock for successful tests
	sequenceMock := newSequenceMockExecutor()
	// Handle all UART set commands generically
	sequenceMock.mockExecutor.addResponse("tpi uart --node 2 set", "", "", nil)

	tests := []struct {
		name     string
		nodeID   int
		steps    []InteractionStep
		executor CommandExecutor
		wantErr  bool
		errMatch string
	}{
		{
			name:   "successful interaction",
			nodeID: 2,
			steps: []InteractionStep{
				{
					Expect: "login:",
					Send:   "root",
					LogMsg: "Sending username",
				},
				{
					Expect: "Password:",
					Send:   "password123",
					LogMsg: "Sending password",
				},
				{
					Expect: "#",
					Send:   "echo 'test done'",
					LogMsg: "Running echo command",
				},
			},
			executor: sequenceMock,
		},
		{
			name:     "invalid node ID",
			nodeID:   5,
			steps:    []InteractionStep{},
			executor: newMockExecutor(),
			wantErr:  true,
			errMatch: "invalid node ID",
		},
		{
			name:   "UART get error",
			nodeID: 2,
			steps: []InteractionStep{
				{
					Expect: "login:",
					Send:   "root",
					LogMsg: "Sending username",
				},
			},
			executor: func() CommandExecutor {
				mock := newMockExecutor()
				mock.addResponse("tpi uart --node 2 get", "", "UART error", fmt.Errorf("failed to get UART data"))
				return mock
			}(),
			wantErr:  true,
			errMatch: "failed to get UART output",
		},
		{
			name:   "UART set error",
			nodeID: 2,
			steps: []InteractionStep{
				{
					Expect: "login:",
					Send:   "root",
					LogMsg: "Sending username",
				},
			},
			executor: func() CommandExecutor {
				mock := newMockExecutor()
				mock.addResponse("tpi uart --node 2 get", "Welcome to Node 2\nlogin: ", "", nil)
				mock.addResponse("tpi uart --node 2 set", "", "UART error", fmt.Errorf("failed to set UART data"))
				return mock
			}(),
			wantErr:  true,
			errMatch: "failed to send UART data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bmc := New(tt.executor)

			// Use a longer timeout for more reliable tests
			timeout := 50 * time.Millisecond

			output, err := bmc.ExpectAndSend(ctx, tt.nodeID, tt.steps, timeout)

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

			// For successful test, verify expected output parts
			if tt.name == "successful interaction" {
				expectedParts := []string{
					"Welcome to Node 2",
					"login: root",
					"Password: password123",
					"echo 'test done'",
					"test done",
				}

				for _, part := range expectedParts {
					if !strings.Contains(output, part) {
						t.Errorf("output missing expected part %q\nGot: %q", part, output)
					}
				}
			}
		})
	}
}
