package operations

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestCommandError(t *testing.T) {
	// Test with basic error and no output
	cmd := "test-command"
	args := []string{"arg1", "arg2"}
	baseErr := errors.New("exit status 1")
	cmdErr := NewCommandError(cmd, args, "", baseErr)

	expectedMsg := "command failed: 'test-command arg1 arg2': exit status 1"
	if cmdErr.Error() != expectedMsg {
		t.Errorf("Expected error message: %s, got: %s", expectedMsg, cmdErr.Error())
	}

	// Test with output
	output := "some command output\nwith multiple lines"
	cmdErrWithOutput := NewCommandError(cmd, args, output, baseErr)

	// Check that the error message contains the expected elements
	if !strings.Contains(cmdErrWithOutput.Error(), "command failed: 'test-command arg1 arg2'") ||
		!strings.Contains(cmdErrWithOutput.Error(), "exit status 1") ||
		!strings.Contains(cmdErrWithOutput.Error(), "some command output") {
		t.Errorf("Expected error to contain command details and output, got: %s", cmdErrWithOutput.Error())
	}

	// Test unwrap functionality
	unwrappedErr := errors.Unwrap(cmdErr)
	if unwrappedErr != baseErr {
		t.Errorf("Unwrap didn't return the original error")
	}
}

func TestOperationError(t *testing.T) {
	// Basic operation error
	operation := "disk cloning"
	context := "my-disk.img"
	baseErr := errors.New("permission denied")
	opErr := NewOperationError(operation, context, baseErr)

	expectedMsg := "disk cloning failed for my-disk.img: permission denied"
	if opErr.Error() != expectedMsg {
		t.Errorf("Expected error message: %s, got: %s", expectedMsg, opErr.Error())
	}

	// Test with empty context
	opErrNoContext := NewOperationError(operation, "", baseErr)
	expectedNoContext := "disk cloning failed: permission denied"
	if opErrNoContext.Error() != expectedNoContext {
		t.Errorf("Expected error message: %s, got: %s", expectedNoContext, opErrNoContext.Error())
	}

	// Test unwrap
	unwrappedErr := errors.Unwrap(opErr)
	if unwrappedErr != baseErr {
		t.Errorf("Unwrap didn't return the original error")
	}

	// Test nesting errors
	cmdErr := NewCommandError("kpartx", []string{"-av", "disk.img"}, "failed output", baseErr)
	nestedOpErr := NewOperationError("partition mapping", "disk.img", cmdErr)

	// The nested error should contain both the operation context and the command details
	nestedErrStr := nestedOpErr.Error()
	if !strings.Contains(nestedErrStr, "partition mapping failed for disk.img") {
		t.Errorf("Nested error doesn't contain operation context: %s", nestedErrStr)
	}
	if !strings.Contains(nestedErrStr, "command failed: 'kpartx -av disk.img'") {
		t.Errorf("Nested error doesn't contain command details: %s", nestedErrStr)
	}
}

func TestFormatCommandOutput(t *testing.T) {
	// Test empty output
	emptyOutput := formatCommandOutput("")
	if emptyOutput != "<no output>" {
		t.Errorf("Expected empty output to be '<no output>', got: %s", emptyOutput)
	}

	// Test single line output
	singleLine := formatCommandOutput("single line output")
	if singleLine != "single line output" {
		t.Errorf("Expected single line to be unchanged, got: %s", singleLine)
	}

	// Test multi-line output
	multiLine := formatCommandOutput("line 1\nline 2\nline 3")
	expectedMultiLine := "\n  | line 1\n  | line 2\n  | line 3"
	if multiLine != expectedMultiLine {
		t.Errorf("Expected formatted multi-line output: %s, got: %s", expectedMultiLine, multiLine)
	}

	// Test truncation of very long output
	longOutput := strings.Repeat("a", 1200)
	truncatedOutput := formatCommandOutput(longOutput)
	if len(truncatedOutput) <= 1000 || !strings.HasSuffix(truncatedOutput, "... [output truncated]") {
		t.Errorf("Long output not truncated correctly: %s", truncatedOutput)
	}
}

func ExampleCommandError() {
	// This shows how to use CommandError in practice
	err := NewCommandError("kpartx", []string{"-av", "disk.img"},
		"add map loop0p1 (253:1): 0 524288 linear 7:1 8192\nadd map loop0p2 (253:2): 0 32768000 linear 7:1 532480",
		fmt.Errorf("exit status 127"))

	fmt.Println(err)
	// Output contains:
	// command failed: 'kpartx -av disk.img': exit status 127
	// Output:
	//   | add map loop0p1 (253:1): 0 524288 linear 7:1 8192
	//   | add map loop0p2 (253:2): 0 32768000 linear 7:1 532480
}

func ExampleOperationError() {
	// This shows how to use OperationError in practice
	baseErr := fmt.Errorf("exit status 127")
	cmdErr := NewCommandError("kpartx", []string{"-av", "disk.img"}, "", baseErr)
	opErr := NewOperationError("partition mapping", "disk.img", cmdErr)

	fmt.Println(opErr)
	// Output:
	// partition mapping failed for disk.img: command failed: 'kpartx -av disk.img': exit status 127
}
