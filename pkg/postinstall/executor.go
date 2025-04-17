package postinstall

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"os/exec"
	"text/template"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/node"
	"gopkg.in/yaml.v3"
)

// Execute runs the post-installation plan defined in the config file.
func Execute(configFile string, execCtx Context) error {
	log.Printf("Loading post-install config: %s", configFile)
	yamlFile, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read config file %s: %w", configFile, err)
	}

	var config Config
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return fmt.Errorf("failed to parse config file %s: %w", configFile, err)
	}

	log.Printf("Executing post-install plan: %s (OS: %s, Board: %s)", config.Description, config.OS, config.Board)

	// --- Step Execution Loop ---
	for i, step := range config.Steps {
		stepIndex := i + 1
		log.Printf("--- Starting Step %d: %s (%s @ %s) ---", stepIndex, step.Name, step.Type, step.Location)

		// Parse timeout if specified
		if step.Timeout != "" {
			step.timeoutDur, err = time.ParseDuration(step.Timeout)
			if err != nil {
				log.Printf("Error parsing timeout duration '%s' for step %d: %v. Using default.", step.Timeout, stepIndex, err)
				step.timeoutDur = 1 * time.Minute // Default timeout? Or make configurable
			}
		} else {
			step.timeoutDur = 5 * time.Minute // Default overall step timeout
		}

		// --- Execute based on type ---
		var stepErr error
		switch step.Type {
		case TypeCommand:
			stepErr = executeCommandStep(step, execCtx)
		case TypeExpect:
			stepErr = executeExpectStep(step, execCtx)
		case TypeCopy:
			stepErr = executeCopyStep(step, execCtx)
		case TypeScript:
			// TODO: Implement script execution (local/remote)
			stepErr = fmt.Errorf("step type '%s' not yet implemented", step.Type)
		case TypeWait:
			// TODO: Implement wait logic
			stepErr = fmt.Errorf("step type '%s' not yet implemented", step.Type)
		default:
			stepErr = fmt.Errorf("unknown step type '%s'", step.Type)
		}

		// --- Handle Step Result ---
		if stepErr != nil {
			log.Printf("--- Error in Step %d: %s ---", stepIndex, step.Name)
			log.Printf("Error details: %v", stepErr)
			if !step.IgnoreError {
				return fmt.Errorf("step %d ('%s') failed: %w", stepIndex, step.Name, stepErr)
			}
			log.Printf("Ignoring error as IgnoreError is true.")
		} else {
			log.Printf("--- Completed Step %d: %s --- ", stepIndex, step.Name)
		}
		fmt.Println() // Add newline for readability between steps
	} // End of steps loop

	log.Println("Post-install plan finished.")
	return nil
}

// executeCommandStep handles steps of type TypeCommand
func executeCommandStep(step Step, execCtx Context) error {
	// Apply templating to the command string
	tmpl, err := template.New("cmd").Parse(step.Command)
	if err != nil {
		return fmt.Errorf("failed to parse command template: %w", err)
	}
	var cmdBuf bytes.Buffer
	if err := tmpl.Execute(&cmdBuf, execCtx); err != nil {
		return fmt.Errorf("failed to execute command template: %w", err)
	}
	executableCommand := cmdBuf.String()

	switch step.Location {
	case LocationLocal:
		log.Printf("Running local command: %s", executableCommand)
		// Use os/exec to run locally
		// TODO: Consider using context with timeout for os/exec
		cmd := exec.Command("bash", "-c", executableCommand) // Wrap in bash -c for pipelines etc.
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err = cmd.Run()
		log.Printf("Local Command Stdout: %s", stdout.String())
		log.Printf("Local Command Stderr: %s", stderr.String())
		if err != nil {
			return fmt.Errorf("local command failed: %w", err)
		}
		log.Println("Local command finished successfully.")
	case LocationRemote:
		log.Printf("Running remote command on %s: %s", execCtx.NodeIP, executableCommand)
		if execCtx.NodeIP == "" || execCtx.User == "" || execCtx.InitialPassword == "" {
			return fmt.Errorf("missing node IP, user, or password in context for remote command")
		}

		// Create node adapter
		adapter := node.NewNodeAdapter(node.SSHConfig{
			Host:     execCtx.NodeIP,
			User:     execCtx.User,
			Password: execCtx.InitialPassword,
			Timeout:  step.timeoutDur,
		})

		stdout, stderr, err := adapter.ExecuteCommand(executableCommand)
		// Log output regardless of error
		log.Printf("Remote Command Stdout: %s", stdout)
		log.Printf("Remote Command Stderr: %s", stderr)
		if err != nil {
			return fmt.Errorf("remote command execution failed: %w", err)
		}
		log.Println("Remote command finished successfully.")
	default:
		return fmt.Errorf("unknown location '%s' for command step", step.Location)
	}
	return nil
}

// executeExpectStep handles steps of type TypeExpect
func executeExpectStep(step Step, execCtx Context) error {
	if step.Location != LocationRemote {
		return fmt.Errorf("expect steps must run on location 'remote'")
	}

	// Apply templating to ExpectScript Send fields
	processedSteps := make([]node.InteractionStep, len(step.ExpectScript))
	for i, interaction := range step.ExpectScript {
		processedSteps[i] = interaction // Copy base info

		tmpl, err := template.New("send").Parse(interaction.Send)
		if err != nil {
			return fmt.Errorf("failed to parse send template for expect step %d: %w", i+1, err)
		}
		var sendBuf bytes.Buffer
		if err := tmpl.Execute(&sendBuf, execCtx); err != nil {
			return fmt.Errorf("failed to execute send template for expect step %d: %w", i+1, err)
		}
		processedSteps[i].Send = sendBuf.String()
	}

	// Extract necessary context for node adapter
	user := execCtx.User
	password := execCtx.InitialPassword // Assuming this is the password for this expect sequence
	if password == "" {
		// Maybe try NewPassword if Initial is empty? Or require explicit password in step?
		return fmt.Errorf("password required in context for remote expect step")
	}

	// Create node adapter
	adapter := node.NewNodeAdapter(node.SSHConfig{
		Host:     execCtx.NodeIP,
		User:     user,
		Password: password,
		Timeout:  step.timeoutDur,
	})

	log.Printf("Executing remote expect script on %s", execCtx.NodeIP)
	finalOutput, err := adapter.ExpectAndSend(processedSteps, step.timeoutDur)

	if err != nil {
		// Log output even on error
		log.Printf("Expect script failed. Final output:\n%s", finalOutput)
		return fmt.Errorf("expect script execution failed: %w", err)
	}

	log.Printf("Expect script finished successfully. Final output:\n%s", finalOutput)

	// TODO: Add verification logic based on finalOutput?
	// E.g., a step could have an optional `verify_contains` field.
	// if step.VerifyContains != "" && !strings.Contains(finalOutput, step.VerifyContains) {
	//    return fmt.Errorf("verification failed: expected output to contain '%s'", step.VerifyContains)
	// }

	return nil
}

// executeCopyStep handles steps of type TypeCopy
func executeCopyStep(step Step, execCtx Context) error {
	// Check for unsupported recursive copy
	if step.CopyRecursive {
		return fmt.Errorf("recursive directory copy is not yet implemented")
	}

	// Apply templating to source and destination paths
	tmpl, err := template.New("paths").Parse(step.CopySource)
	if err != nil {
		return fmt.Errorf("failed to parse source path template: %w", err)
	}
	var sourceBuf bytes.Buffer
	if err := tmpl.Execute(&sourceBuf, execCtx); err != nil {
		return fmt.Errorf("failed to execute source path template: %w", err)
	}
	sourcePath := sourceBuf.String()

	tmpl, err = template.New("paths").Parse(step.CopyDest)
	if err != nil {
		return fmt.Errorf("failed to parse destination path template: %w", err)
	}
	var destBuf bytes.Buffer
	if err := tmpl.Execute(&destBuf, execCtx); err != nil {
		return fmt.Errorf("failed to execute destination path template: %w", err)
	}
	destPath := destBuf.String()

	// Create node adapter
	adapter := node.NewNodeAdapter(node.SSHConfig{
		Host:     execCtx.NodeIP,
		User:     execCtx.User,
		Password: execCtx.InitialPassword,
		Timeout:  step.timeoutDur,
	})

	log.Printf("Copying file: %s -> %s (from_remote: %t)", sourcePath, destPath, step.CopyFromRemote)
	err = adapter.CopyFile(sourcePath, destPath, !step.CopyFromRemote)
	if err != nil {
		return fmt.Errorf("file copy failed: %w", err)
	}

	log.Printf("File copy completed successfully.")
	return nil
}
