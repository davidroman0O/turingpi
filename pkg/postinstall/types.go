package postinstall

import (
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi/node"
)

// StepType defines the type of post-install action.
type StepType string

const (
	TypeCommand StepType = "command" // Run a shell command
	TypeScript  StepType = "script"  // Run a shell script
	TypeExpect  StepType = "expect"  // Interactive expect/send sequence
	TypeCopy    StepType = "copy"    // Copy files (local->remote or remote->local?)
	TypeWait    StepType = "wait"    // Wait for a condition (e.g., port open, command success)
)

// StepLocation defines where the step should be executed.
type StepLocation string

const (
	LocationLocal  StepLocation = "local"  // Run on the machine executing the CLI
	LocationRemote StepLocation = "remote" // Run via SSH on the target node
)

// Step defines a single post-installation step.
type Step struct {
	Name        string        `yaml:"name"`                   // Descriptive name for the step
	Type        StepType      `yaml:"type"`                   // Type of action
	Location    StepLocation  `yaml:"location,omitempty"`     // Where to run (defaults might apply based on type)
	IgnoreError bool          `yaml:"ignore_error,omitempty"` // Continue even if this step fails
	Timeout     string        `yaml:"timeout,omitempty"`      // Optional timeout duration string (e.g., "60s", "5m")
	timeoutDur  time.Duration // Parsed timeout duration

	// --- Type-specific parameters ---

	// For TypeCommand, TypeScript, TypeWait (command field)
	Command string `yaml:"command,omitempty"`

	// For TypeScript (alternative to Command if script content is large)
	Script string `yaml:"script,omitempty"`

	// For TypeExpect - Use the definition from pkg/node
	ExpectScript []node.InteractionStep `yaml:"expect_script,omitempty"`

	// For TypeCopy
	CopySource     string `yaml:"copy_source,omitempty"`
	CopyDest       string `yaml:"copy_dest,omitempty"`
	CopyRecursive  bool   `yaml:"copy_recursive,omitempty"`
	CopyFromRemote bool   `yaml:"copy_from_remote,omitempty"` // Flag to indicate remote->local copy

	// For TypeWait (in addition to Command which checks condition)
	WaitInterval    string        `yaml:"wait_interval,omitempty"` // How often to check (e.g., "5s")
	waitIntervalDur time.Duration // Parsed interval

	// TODO: Add fields for user/permissions if needed for remote execution?
}

// Config defines the overall post-installation plan.
type Config struct {
	OS          string `yaml:"os,omitempty"`    // Target OS identifier (e.g., "ubuntu", "debian")
	Board       string `yaml:"board,omitempty"` // Target board identifier (e.g., "rk1", "cm4")
	Description string `yaml:"description,omitempty"`
	Steps       []Step `yaml:"steps"`
}

// Context holds dynamic data needed during execution, passed to templates.
type Context struct {
	NodeIP          string
	User            string // User for SSH/remote commands
	InitialPassword string // Optional: Initial password if needed
	NewPassword     string // Optional: New password if provided
	// Add other dynamic parameters as needed
	Params map[string]string // Generic parameters map
}
