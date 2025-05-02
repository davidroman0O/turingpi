package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/davidroman0O/turingpi/cueworkflow"
	"github.com/spf13/cobra"
)

func main() {
	// Create the root command
	rootCmd := &cobra.Command{
		Use:   "tpi",
		Short: "Turing Pi Workflow Engine CLI",
		Long:  "A command-line interface for the Turing Pi Workflow Engine",
	}

	// Create the run command
	configFlag := ""
	runCmd := &cobra.Command{
		Use:   "run [file.cue] [workflow] [param1=value1] [param2=value2] ...",
		Short: "Run a workflow from a CUE file",
		Args:  cobra.MinimumNArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			// Extract arguments
			cueFile := args[0]
			workflowName := args[1]
			params := make(map[string]interface{})

			// Parse any parameter arguments (format: key=value)
			for i := 2; i < len(args); i++ {
				parts := strings.SplitN(args[i], "=", 2)
				if len(parts) != 2 {
					fmt.Printf("Error: Invalid parameter format: %s (expected name=value)\n", args[i])
					os.Exit(1)
				}

				name := parts[0]
				valueStr := parts[1]

				// Try to parse as different types
				if intVal, err := strconv.Atoi(valueStr); err == nil {
					// It's an integer
					params[name] = intVal
				} else if floatVal, err := strconv.ParseFloat(valueStr, 64); err == nil {
					// It's a float
					params[name] = floatVal
				} else if boolVal, err := strconv.ParseBool(valueStr); err == nil {
					// It's a boolean
					params[name] = boolVal
				} else {
					// Default to string
					params[name] = valueStr
				}
			}

			ctx := context.Background()

			// Load cluster configuration
			if configFlag == "" {
				configFlag = "testdata/examples/configs/config.cue"
			}
			log.Printf("Loading cluster configuration from %s", configFlag)
			config, err := cueworkflow.LoadConfig(ctx, configFlag)
			if err != nil {
				log.Fatalf("Error loading configuration: %v", err)
			}

			// Load workflow
			log.Printf("Loading workflow %s from %s", workflowName, cueFile)
			workflow, err := cueworkflow.LoadWorkflow(ctx, cueFile, workflowName, params)
			if err != nil {
				log.Fatalf("Error loading workflow: %v", err)
			}

			// Execute the workflow
			log.Println("Executing workflow")
			if err := cueworkflow.ExecuteWorkflow(ctx, workflow, config); err != nil {
				log.Fatalf("Error executing workflow: %v", err)
			}

			log.Println("Workflow execution completed successfully")
		},
	}

	// Add flags to the run command
	runCmd.Flags().StringVarP(&configFlag, "config", "c", "", "Path to cluster configuration file (default: testdata/examples/configs/config.cue)")

	// Create the init command
	initCmd := &cobra.Command{
		Use:   "init [type] [output]",
		Short: "Create a template file",
		Long:  "Create a workflow or cluster configuration template file",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			templateType := args[0]
			outputFile := args[1]

			// Validate template type
			if templateType != "workflow" && templateType != "cluster" {
				log.Fatalf("Error: Invalid template type: %s (supported: workflow, cluster)", templateType)
			}

			// Ensure the directory exists
			outputDir := filepath.Dir(outputFile)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				log.Fatalf("Error creating output directory: %v", err)
			}

			// Save the template
			ctx := context.Background()
			if err := cueworkflow.SaveCUETemplate(ctx, templateType, outputFile); err != nil {
				log.Fatalf("Error saving template: %v", err)
			}

			log.Printf("Template created successfully: %s", outputFile)
		},
	}

	// Add subcommands to the root command
	rootCmd.AddCommand(runCmd, initCmd)

	// Execute the command
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
