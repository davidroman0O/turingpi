// Package operations provides a unified interface for operations that can run
// either directly on the host (Linux) or inside a container (non-Linux systems)
package operations

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/davidroman0O/turingpi/pkg/v2/container"
)

// CommandExecutor defines an interface for executing commands
// that works both on native Linux systems and inside containers
type CommandExecutor interface {
	// Execute runs a command and returns its output
	Execute(ctx context.Context, name string, args ...string) ([]byte, error)

	// ExecuteWithInput runs a command with input and returns its output
	ExecuteWithInput(ctx context.Context, input string, name string, args ...string) ([]byte, error)

	// ExecuteInPath runs a command in a specific directory and returns its output
	ExecuteInPath(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

// ExecuteCommand is a helper that executes a command and returns a formatted error if it fails
func ExecuteCommand(executor CommandExecutor, ctx context.Context, name string, args ...string) ([]byte, error) {
	output, err := executor.Execute(ctx, name, args...)
	if err != nil {
		// Create a detailed error with command information
		return output, NewCommandError(name, args, string(output), err)
	}
	return output, nil
}

// ExecuteCommandWithInput is a helper that executes a command with input and returns a formatted error if it fails
func ExecuteCommandWithInput(executor CommandExecutor, ctx context.Context, input string, name string, args ...string) ([]byte, error) {
	output, err := executor.ExecuteWithInput(ctx, input, name, args...)
	if err != nil {
		// Create a detailed error with command information
		return output, NewCommandError(name, args, string(output), err)
	}
	return output, nil
}

// ExecuteCommandInPath is a helper that executes a command in a specific directory and returns a formatted error if it fails
func ExecuteCommandInPath(executor CommandExecutor, ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	output, err := executor.ExecuteInPath(ctx, dir, name, args...)
	if err != nil {
		// Create a detailed error with command information
		cmdErr := NewCommandError(name, args, string(output), err)
		return output, fmt.Errorf("in directory %s: %w", dir, cmdErr)
	}
	return output, nil
}

// NativeExecutor implements CommandExecutor by directly executing commands on the host OS
type NativeExecutor struct{}

// Execute implements CommandExecutor.Execute for native OS execution
func (e *NativeExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.CombinedOutput()
}

// ExecuteWithInput implements CommandExecutor.ExecuteWithInput for native OS execution
func (e *NativeExecutor) ExecuteWithInput(ctx context.Context, input string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = strings.NewReader(input)
	return cmd.CombinedOutput()
}

// ExecuteInPath implements CommandExecutor.ExecuteInPath for native OS execution
func (e *NativeExecutor) ExecuteInPath(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// ContainerExecutor implements CommandExecutor by executing commands inside a container
type ContainerExecutor struct {
	container container.Container
}

// NewContainerExecutor creates a new ContainerExecutor
func NewContainerExecutor(container container.Container) *ContainerExecutor {
	return &ContainerExecutor{
		container: container,
	}
}

// Execute implements CommandExecutor.Execute for container execution
func (e *ContainerExecutor) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := append([]string{name}, args...)
	output, err := e.container.Exec(ctx, cmd)
	return []byte(output), err
}

// ExecuteWithInput implements CommandExecutor.ExecuteWithInput for container execution
func (e *ContainerExecutor) ExecuteWithInput(ctx context.Context, input string, name string, args ...string) ([]byte, error) {
	// For container execution with input, we need to create a temporary file with the input
	// and pipe it to the command inside the container
	inputFile := "/tmp/cmd_input"

	// Write input to a file in the container
	if err := e.container.ExecDetached(ctx, []string{"sh", "-c", fmt.Sprintf("cat > %s", inputFile)}); err != nil {
		return nil, fmt.Errorf("failed to create input file in container: %w", err)
	}

	// Execute command with input from the file
	cmd := append([]string{"sh", "-c", fmt.Sprintf("cat %s | %s %s",
		inputFile,
		name,
		strings.Join(args, " "))})

	output, err := e.container.Exec(ctx, cmd)

	// Clean up temporary file
	_ = e.container.ExecDetached(ctx, []string{"rm", "-f", inputFile})

	return []byte(output), err
}

// ExecuteInPath implements CommandExecutor.ExecuteInPath for container execution
func (e *ContainerExecutor) ExecuteInPath(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	// First ensure the directory exists in the container
	if err := e.container.ExecDetached(ctx, []string{"mkdir", "-p", dir}); err != nil {
		return nil, fmt.Errorf("failed to create directory in container: %w", err)
	}

	// Execute the command in the specified directory
	cmd := append([]string{"sh", "-c", fmt.Sprintf("cd %s && %s %s",
		dir,
		name,
		strings.Join(args, " "))})

	output, err := e.container.Exec(ctx, cmd)
	return []byte(output), err
}

// NewExecutor creates a CommandExecutor based on the current runtime environment
func NewExecutor(containerClient container.Container) CommandExecutor {
	// If we're on Linux, use the native executor
	if runtime.GOOS == "linux" {
		return &NativeExecutor{}
	}

	// If we're on a different OS, use the container executor
	return NewContainerExecutor(containerClient)
}
