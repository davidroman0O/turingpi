package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi"
	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
	"github.com/davidroman0O/turingpi/pkg/tpi/state"
	"github.com/davidroman0O/turingpi/pkg/tpi/ubuntu"
)

func main() {
	log.Println("--- Starting TPI Playground - Complete Workflow Example ---")

	// Check if we're running on a non-Linux platform
	if !platform.IsLinux() {
		// Check if Docker is available for image operations
		if !platform.DockerAvailable() {
			log.Println("WARNING: Running on a non-Linux platform and Docker is not available!")
			log.Println("Docker is required for image operations (kpartx, mount) on non-Linux platforms.")
			log.Println("Please install Docker and ensure it is running before continuing.")
			os.Exit(1)
		}
		log.Println("Detected non-Linux platform. Docker will be used for image operations.")
	}

	// --- Configuration ---
	// Using real Turing Pi BMC details!
	cfg := tpi.TPIConfig{
		IP:           "192.168.1.90",                                                    // Real Turing Pi BMC IP
		BMCUser:      "root",                                                            // Default BMC username
		BMCPassword:  "turing",                                                          // Default BMC password
		CacheDir:     "./.tpi_cache_playground",                                         // Use a local cache for the example
		PrepImageDir: "/Users/davidroman/Documents/code/github/turingpi/tmp-image-prep", // Temporary directory for image processing
		Node1: &tpi.NodeConfig{
			IP:    "192.168.1.101/24", // Target static IP for the node (including CIDR)
			Board: state.RK1,          // Node 1 is RK1
		},
		// Add Node2, Node3, Node4 configs if needed
	}

	// --- Initialize Executor ---
	executor, err := tpi.NewTuringPi(cfg)
	if err != nil {
		log.Fatalf("Error initializing TPI Executor: %v", err)
		os.Exit(1)
	}
	log.Println("TPI Executor initialized.")

	// --- Define Workflow ---
	// This is the template function that defines the steps for ONE node.
	workflowTemplate := func(ctx tpi.Context, cluster tpi.Cluster, node tpi.Node) error {
		log.Printf("Running workflow for Node %d (%s)", node.ID, node.Config.Board)

		// Prepare network configuration from node details
		networkConfig := tpi.NetworkConfig{
			IPCIDR:     node.Config.IP,                     // Use the CIDR from the config
			Hostname:   fmt.Sprintf("rk1-node%d", node.ID), // Follow naming convention from docs
			Gateway:    "192.168.1.1",                      // Standard gateway from example docs
			DNSServers: []string{"1.1.1.1", "8.8.8.8"},     // Cloudflare and Google DNS from docs
		}

		// --- Phase 1: Image Build ---
		sourceImageDir := "/Users/davidroman/Documents/iso/turingpi"
		sourceImageName := "ubuntu-22.04.3-preinstalled-server-arm64-turing-rk1_v1.33.img.xz"
		baseImagePath := filepath.Join(sourceImageDir, sourceImageName)

		// Define where final prepared images will be stored
		// This is NOT the temporary processing directory (PrepImageDir)
		// This is where the final customized images are placed
		preparedImageDir := "/Users/davidroman/Documents/code/github/turingpi/prepared-images"

		// Ensure the prepared images directory exists
		err = os.MkdirAll(preparedImageDir, 0755)
		if err != nil {
			log.Printf("Error: Failed to create prepared images directory: %v", err)
			return fmt.Errorf("failed to create prepared images directory: %w", err)
		}

		// Ensure the temporary image processing directory exists
		// This is already done by the UbuntuImageBuilder, but doing it explicitly for clarity
		if tmpDir := cluster.GetPrepImageDir(); tmpDir != "" {
			if err := os.MkdirAll(tmpDir, 0755); err != nil {
				log.Printf("Error: Failed to create temporary image processing directory: %v", err)
				return fmt.Errorf("failed to create temporary image processing directory: %w", err)
			}
			log.Printf("Using temporary image processing directory: %s", tmpDir)
		}

		// Check if the source image exists
		if _, err := os.Stat(baseImagePath); os.IsNotExist(err) {
			log.Printf("Error: Source image '%s' not found. Please check the path.", baseImagePath)
			return fmt.Errorf("source image not found: %s", baseImagePath)
		} else {
			log.Printf("Found source image: %s", baseImagePath)
		}

		// Create an image builder for this specific node
		imageBuilder := ubuntu.NewImageBuilder(node.ID)

		// Specify the source image path
		imageBuilder = imageBuilder.WithBaseImage(baseImagePath)

		// Set the network configuration
		imageBuilder = imageBuilder.WithNetworkConfig(networkConfig)

		// Set the output directory for prepared images
		imageBuilder = imageBuilder.WithOutputDirectory(preparedImageDir)

		// Add pre-install customizations (optional)
		imageBuilder = imageBuilder.WithPreInstall(func(image tpi.ImageModifier) error {
			// Example 1: Customize a configuration file
			helloContent := []byte(fmt.Sprintf("Hello from Turing Pi node %d (%s)!\n", node.ID, networkConfig.Hostname))
			image.WriteFile("/etc/motd", helloContent, 0644)

			// Example 2: Create a directory and write a script
			image.MkdirAll("/opt/turingpi", 0755)
			scriptContent := []byte("#!/bin/bash\necho 'Turing Pi RK1 node is running!'\nuptime\n")
			image.WriteFile("/opt/turingpi/healthcheck.sh", scriptContent, 0755)

			return nil
		})

		// Execute Phase 1 (Image Build)
		log.Println("Executing Phase 1: Image Customization...")
		imageResult, err := imageBuilder.Run(ctx, cluster)
		if err != nil {
			return fmt.Errorf("phase 1 (Image Build) failed: %w", err)
		}
		log.Printf("Phase 1 completed: Prepared image at %s", imageResult.ImagePath)

		// --- Phase 2: OS Installation ---
		log.Println("Executing Phase 2: OS Installation...")

		// Generate a generic config with SSH keys (or hardcode a public key)
		sshPublicKey := "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQCxuZZ1rJJMxuZ0HYW/FZFvd1Y4PT1CUdZmY/1jizwEXxIJ9lpI3laA5hxopV4dUYEQhkj7AcjHLcZCBOKhV0WcqGsJXpqHpiWlk1YEWxwQHPx46gejHi2VL/UBusMw+YMGH/P3p+s8h5LbgFwkIzYxzRbVJNJYv1gOxnQPV7+XnHU5FO+dRN1M4sMt5gGAq0OlU6f1a1+z2zCdHGwXDKVOqWGzGME6v2FVuK32N5c+8XY0kZdXxL8VxmXvFbZIa1wRcYAaohwGhnC4+GrZhJFp9hgzH8nPDLpKAizO9yw7cjZ4KjfRlZanGNQ7GTnQkwGH0D6zLGe0B0L6Q3KAxTmJ turing@example.com"

		genericConfig := tpi.OSInstallConfig{
			SSHKeys: []string{sshPublicKey},
		}

		ubuntuInstaller := ubuntu.NewOSInstaller(node.ID, tpi.UbuntuInstallConfig{
			// Default Ubuntu password
			InitialUserPassword: "ubuntu",
		})

		// Add the generic config with SSH keys
		ubuntuInstaller = ubuntuInstaller.WithGenericConfig(genericConfig)

		// Provide the image from Phase 1 and run installation
		err = ubuntuInstaller.UsingImage(imageResult).Run(ctx, cluster)
		if err != nil {
			return fmt.Errorf("phase 2 (OS Install) failed: %w", err)
		}
		log.Printf("Phase 2 completed: OS installed on node %d", node.ID)

		// --- Phase 3: Post-Install Configuration ---
		log.Println("Executing Phase 3: Post-Installation Configuration...")

		// Note: Password change is already handled by OS installer
		// We'll use the new password set during OS installation
		postInstaller := ubuntu.NewPostInstaller(node.ID)
		postInstaller = postInstaller.WithUser("ubuntu")           // Default Ubuntu username
		postInstaller = postInstaller.WithPassword("TuringPi123!") // Password set during OS installation

		// Define the post-installation actions
		postInstaller = postInstaller.RunActions(func(local tpi.LocalRuntime, remote tpi.UbuntuRuntime) error {
			// Example 1: Run a simple command to check system
			stdout, _, err := remote.RunCommand("uname -a", 5*time.Second)
			if err != nil {
				return fmt.Errorf("failed to get system info: %w", err)
			}
			log.Printf("System info: %s", stdout)

			// Example 2: Create a local file and upload it to the node
			reportPath := filepath.Join(cluster.GetCacheDir(), "install_report.txt")
			reportContent := []byte(fmt.Sprintf("Turing Pi Installation Report\n-------------------------\nNode: %d\nHostname: %s\nIP: %s\nBoard: %s\nInstalled at: %s\n",
				node.ID, networkConfig.Hostname, networkConfig.IPCIDR, node.Config.Board, time.Now().Format(time.RFC3339)))

			if err := local.WriteFile(reportPath, reportContent, 0644); err != nil {
				return fmt.Errorf("failed to create local report file: %w", err)
			}

			// Upload to the ubuntu user's home directory instead of /opt/turingpi
			if err := remote.CopyFile(reportPath, "/home/ubuntu/install_report.txt", true); err != nil {
				return fmt.Errorf("failed to upload report to node: %w", err)
			}

			return nil
		})

		// Execute Phase 3 (Post-Install Configuration)
		err = postInstaller.Run(ctx, cluster)
		if err != nil {
			return fmt.Errorf("phase 3 (Post-Install) failed: %w", err)
		}
		log.Printf("Phase 3 completed: Node %d configured successfully", node.ID)

		log.Printf("Complete workflow finished successfully for Node %d", node.ID)
		return nil
	}

	// --- Get Node Execution Function ---
	// The Run method prepares the execution function based on the template.
	executeNodeWorkflow := executor.Run(workflowTemplate)

	// --- Execute for Node 1 ---
	log.Println("Executing workflow specifically for Node 1...")
	err = executeNodeWorkflow(context.Background(), tpi.Node1)
	if err != nil {
		log.Fatalf("Failed to execute workflow for Node 1: %v", err)
		os.Exit(1)
	}

	log.Println("--- TPI Playground - Complete Workflow Example Finished Successfully ---")
}
