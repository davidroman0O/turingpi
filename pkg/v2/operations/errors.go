package operations

import (
	"fmt"
	"strings"
)

// CommandError represents an error that occurred while executing a command
type CommandError struct {
	Command string   // The command that was executed
	Args    []string // The arguments passed to the command
	Output  string   // The command output (stdout/stderr)
	Err     error    // The underlying error
}

// Error implements the error interface
func (e *CommandError) Error() string {
	// Format the full command for display
	fullCmd := e.Command
	if len(e.Args) > 0 {
		fullCmd += " " + strings.Join(e.Args, " ")
	}

	if e.Output == "" {
		return fmt.Sprintf("command failed: '%s': %v", fullCmd, e.Err)
	}

	// Provide detailed output for debugging
	return fmt.Sprintf("command failed: '%s': %v\nOutput: %s",
		fullCmd, e.Err, formatCommandOutput(e.Output))
}

// Unwrap returns the underlying error
func (e *CommandError) Unwrap() error {
	return e.Err
}

// NewCommandError creates a new CommandError
func NewCommandError(command string, args []string, output string, err error) *CommandError {
	return &CommandError{
		Command: command,
		Args:    args,
		Output:  output,
		Err:     err,
	}
}

// formatCommandOutput formats command output for better readability in error messages
func formatCommandOutput(output string) string {
	if output == "" {
		return "<no output>"
	}

	// Trim whitespace and limit length if it's too long
	output = strings.TrimSpace(output)
	if len(output) > 1000 {
		output = output[:1000] + "... [output truncated]"
	}

	// Add indentation for multi-line output
	if strings.Contains(output, "\n") {
		lines := strings.Split(output, "\n")
		for i, line := range lines {
			lines[i] = "  | " + line
		}
		return "\n" + strings.Join(lines, "\n")
	}

	return output
}

// OperationError represents an error that occurred during an operation
type OperationError struct {
	Operation string // The operation that failed
	Context   string // Additional context about the operation
	Err       error  // The underlying error
}

// Error implements the error interface
func (e *OperationError) Error() string {
	if e.Context == "" {
		return fmt.Sprintf("%s failed: %v", e.Operation, e.Err)
	}
	return fmt.Sprintf("%s failed for %s: %v", e.Operation, e.Context, e.Err)
}

// Unwrap returns the underlying error
func (e *OperationError) Unwrap() error {
	return e.Err
}

// NewOperationError creates a new OperationError
func NewOperationError(operation string, context string, err error) *OperationError {
	return &OperationError{
		Operation: operation,
		Context:   context,
		Err:       err,
	}
}
