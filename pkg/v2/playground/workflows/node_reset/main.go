package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/turingpi/pkg/v2/actions"
	"github.com/davidroman0O/turingpi/pkg/v2/bmc"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
	"github.com/davidroman0O/turingpi/pkg/v2/workflows"
)

// SSHConfig holds SSH connection details
type SSHConfig struct {
	Host      string `json:"host"`
	Port      int    `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	RemoteDir string `json:"remote_dir"`
}

// VerboseLogger implements gostage.Logger with detailed logging output
type VerboseLogger struct{}

func (l *VerboseLogger) Info(format string, args ...interface{}) {
	fmt.Printf("[INFO] "+format+"\n", args...)
}

func (l *VerboseLogger) Debug(format string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+format+"\n", args...)
}

func (l *VerboseLogger) Error(format string, args ...interface{}) {
	fmt.Printf("[ERROR] "+format+"\n", args...)
}

func (l *VerboseLogger) Warn(format string, args ...interface{}) {
	fmt.Printf("[WARN] "+format+"\n", args...)
}

func main() {
	// Parse command line arguments
	nodeID := flag.Int("node", 1, "Node ID to reset (1-4)")
	hardReset := flag.Bool("hard", false, "Perform a hard reset (default: soft reset)")
	configPath := flag.String("config", "", "Path to SSH config (default: use built-in)")
	cacheDir := flag.String("cache", filepath.Join(os.TempDir(), "turingpi-cache"), "Path for local cache")
	verbose := flag.Bool("verbose", false, "Enable verbose logging")
	flag.Parse()

	// Validate node ID
	if *nodeID < 1 || *nodeID > 4 {
		log.Fatalf("Invalid node ID: %d (must be 1-4)", *nodeID)
	}

	// Set up logging
	var logger gostage.Logger
	if *verbose {
		fmt.Println("Verbose logging enabled")
		logger = &VerboseLogger{}
	} else {
		logger = gostage.NewDefaultLogger()
	}

	// Load SSH config
	var sshConfig SSHConfig
	if *configPath != "" {
		// Use specified config file
		data, err := os.ReadFile(*configPath)
		if err != nil {
			log.Fatalf("Failed to read SSH config: %v", err)
		}
		if err := json.Unmarshal(data, &sshConfig); err != nil {
			log.Fatalf("Failed to parse SSH config: %v", err)
		}
	} else {
		// Use built-in default values
		sshConfig = SSHConfig{
			Host:      "192.168.1.90",
			Port:      22,
			User:      "root",
			Password:  "turing",
			RemoteDir: "/tmp/turingpi-cache",
		}
	}

	// Print configuration
	fmt.Printf("Node Reset Playground\n")
	fmt.Printf("--------------------\n")
	fmt.Printf("Target Node: %d\n", *nodeID)
	fmt.Printf("Reset Type: %s\n", map[bool]string{false: "Soft", true: "Hard"}[*hardReset])
	fmt.Printf("SSH Host: %s:%d\n", sshConfig.Host, sshConfig.Port)
	fmt.Printf("Cache Directory: %s\n", *cacheDir)
	fmt.Printf("Remote Cache: %s\n", sshConfig.RemoteDir)
	fmt.Println()

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle OS signals for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nReceived shutdown signal, canceling operations...")
		cancel()
	}()

	// Initialize BMC executor
	fmt.Println("Setting up BMC connection...")
	bmcExecutor := bmc.NewCommandExecutor("tpi")

	// Initialize tool provider
	toolConfig := &tools.TuringPiToolConfig{
		BMCExecutor: bmcExecutor,
		CacheDir:    *cacheDir,
		SSH: &tools.SSHConfig{
			Host:       sshConfig.Host,
			User:       sshConfig.User,
			Password:   sshConfig.Password,
			RemotePath: sshConfig.RemoteDir,
		},
	}

	// Let the tool provider automatically detect platform capabilities
	toolProvider, err := tools.NewTuringPiToolProvider(toolConfig)
	if err != nil {
		log.Fatalf("Failed to initialize tools: %v", err)
	}

	// Create the node reset workflow
	fmt.Printf("Creating %s reset workflow for node %d...\n",
		map[bool]string{false: "soft", true: "hard"}[*hardReset], *nodeID)
	workflow := workflows.CreateNodeResetWorkflow(*nodeID, *hardReset)

	// Store the tool provider in workflow store and set up middleware to propagate it to action contexts
	fmt.Println("Initializing workflow...")
	if err := workflow.Store.Put("$tools", toolProvider); err != nil {
		log.Fatalf("Failed to store tools in workflow store: %v", err)
	}

	// Set up middleware to ensure tools are passed to each action context
	workflow.AddMiddleware(func(ctx *gostage.ActionContext, next gostage.ActionExecutor) error {
		// Ensure tools are available in the action context
		if err := actions.StoreToolsInContext(ctx, toolProvider); err != nil {
			return fmt.Errorf("middleware failed to inject tools: %w", err)
		}
		return next(ctx)
	})

	// Execute the workflow
	fmt.Printf("Executing workflow: %s\n", workflow.Name)
	fmt.Println("--------------------")
	startTime := time.Now()

	err = workflow.Execute(ctx, logger)

	duration := time.Since(startTime)
	fmt.Println("--------------------")

	if err != nil {
		fmt.Printf("Workflow failed after %v: %v\n", duration, err)
		os.Exit(1)
	}

	fmt.Printf("Workflow completed successfully in %v\n", duration)
}
