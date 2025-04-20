package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi"
	"github.com/davidroman0O/turingpi/pkg/tpi/os/ubuntu"
	"github.com/davidroman0O/turingpi/pkg/tpi/platform"
	"github.com/davidroman0O/turingpi/pkg/tpi/state"
)

func main() {
	log.Println("--- Starting TPI Playground - Direct Toolkit Approach ---")

	// Get absolute path for our working directory
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get working directory: %v", err)
	}

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
	cfg := tpi.TPIConfig{
		BMCIP:        "192.168.1.90",                                  // Real Turing Pi BMC IP
		BMCUser:      "root",                                          // Default BMC username
		BMCPassword:  "turing",                                        // Default BMC password
		CacheDir:     filepath.Join(workDir, ".tpi_cache_playground"), // Use a local cache for the example
		PrepImageDir: filepath.Join(workDir, "tmp-image-prep"),        // Temporary directory for image processing
		Node1: &tpi.NodeConfig{
			IP:    "192.168.1.101/24", // Target static IP for the node (including CIDR)
			Board: state.RK1,          // Node 1 is RK1
			Network: &tpi.Network{
				Gateway:    "192.168.1.1",
				DNSServers: []string{"1.1.1.1", "8.8.8.8"},
			},
		},
	}

	// --- Initialize System Components ---
	executor, err := tpi.NewTuringPi(cfg)
	if err != nil {
		log.Fatalf("Error initializing TPI Executor: %v", err)
		os.Exit(1)
	}
	log.Println("TPI Executor initialized.")

	// Create a context with timeout for the entire operation
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Convert to a TPI context
	tpiCtx := tpi.NewContext(ctx)

	// Set up node details for node 1
	nodeID := tpi.Node1
	node := &tpi.Node{
		ID:     nodeID,
		Config: executor.GetNodeConfig(nodeID),
	}

	// Define hostname for this node
	hostname := fmt.Sprintf("turingpi-node%d", node.ID)

	// Base image path (example)
	sourceImageDir := "/Users/davidroman/Documents/iso/turingpi"
	sourceImageName := "ubuntu-22.04.3-preinstalled-server-arm64-turing-rk1_v1.33.img.xz"
	baseImagePath := filepath.Join(sourceImageDir, sourceImageName)

	// Get the Ubuntu provider
	ubuntuProvider := &ubuntu.Provider{}

	// --- Phase 1: Image Building ---
	log.Printf("Starting Phase 1: Image Customization for Node %d", node.ID)

	// Create an image builder
	imageBuilder := ubuntuProvider.NewImageBuilder(node.ID).(*ubuntu.ImageBuilder)

	// Create image build configuration
	buildConfig := &ubuntu.ImageBuildConfig{
		BaseConfig: ubuntu.BaseConfig{
			Key:     fmt.Sprintf("turingpi-node%d-ubuntu2204-%d", node.ID, time.Now().Unix()),
			Version: ubuntu.V2204LTS,
			Tags:    map[string]string{"purpose": "playground"},
			Force:   true,
		},
		NetworkingConfig: ubuntu.NetworkingConfig{
			StaticIP:   strings.Split(node.Config.IP, "/")[0],
			Gateway:    node.Config.Network.Gateway,
			DNSServers: node.Config.Network.DNSServers,
			Hostname:   hostname,
		},
		Board:           node.Config.Board,
		BaseImageXZPath: baseImagePath,

		// Custom image modifications
		ImageCustomizationFunc: func(image tpi.ImageModifier) error {
			// Example 1: Customize a configuration file
			helloContent := []byte(fmt.Sprintf("Hello from Turing Pi node %d (%s)!\n", node.ID, hostname))
			image.WriteFile("/etc/motd", helloContent, 0644)

			// Example 2: Create a directory and write a script
			image.MkdirAll("/opt/turingpi", 0755)
			setupScript := []byte("#!/bin/bash\n\necho 'Setup complete!'\n")
			image.WriteFile("/opt/turingpi/setup.sh", setupScript, 0755)

			return nil
		},
	}

	// Configure the image builder
	if err := imageBuilder.Configure(buildConfig); err != nil {
		log.Fatalf("Failed to configure image builder: %v", err)
	}

	// Run the image build process
	imageResult, err := imageBuilder.Run(tpiCtx, executor)
	if err != nil {
		log.Fatalf("Image build process failed: %v", err)
	}
	log.Printf("Image built successfully. Image at: %s", imageResult.ImagePath)

	// --- Phase 2: OS Installation ---
	log.Printf("Starting Phase 2: OS Installation for Node %d", node.ID)

	// Create an OS installer
	osInstaller := ubuntuProvider.NewOSInstaller(node.ID).(*ubuntu.UbuntuOSInstallerBuilder)

	// Configure the OS installer
	installConfig := &ubuntu.InstallConfig{
		BaseConfig: ubuntu.BaseConfig{
			Version: ubuntu.V2204LTS,
			Tags:    map[string]string{"purpose": "playground"},
		},
		NetworkingConfig: ubuntu.NetworkingConfig{
			Hostname: hostname,
		},
		TargetDevice: "/dev/sda",
		Username:     "ubuntu",
		Password:     "TuringPi123!",
	}

	// Configure and run the OS installation
	if err := osInstaller.Configure(installConfig); err != nil {
		log.Fatalf("Failed to configure OS installer: %v", err)
	}

	if err := osInstaller.UsingImage(imageResult).Run(tpiCtx, executor); err != nil {
		log.Fatalf("OS installation failed: %v", err)
	}
	log.Printf("OS installation completed successfully for node %d", node.ID)

	// --- Phase 3: Post-Installation Configuration ---
	log.Printf("Starting Phase 3: Post-Installation Configuration for Node %d", node.ID)

	// Create a post-installer
	postInstaller := ubuntuProvider.NewPostInstaller(node.ID).(*ubuntu.UbuntuPostInstallerBuilder)

	// Configure post-installation
	postConfig := &ubuntu.PostInstallConfig{
		BaseConfig: ubuntu.BaseConfig{
			Version: ubuntu.V2204LTS,
			Tags:    map[string]string{"purpose": "playground"},
		},
		NetworkingConfig: ubuntu.NetworkingConfig{
			Hostname: hostname,
		},
		Username:     "ubuntu",
		Password:     "TuringPi123!",
		LocaleConfig: "en_US.UTF-8",
		Timezone:     "UTC",
		Packages:     []string{"vim", "curl", "htop", "docker.io", "docker-compose"},

		// Runtime customization
		RuntimeConfig: ubuntu.RuntimeConfig{
			RuntimeCustomizationFunc: func(local ubuntu.LocalRuntime, remote ubuntu.UbuntuRuntime) error {
				log.Println("Performing custom runtime operations...")

				// Example: Create a welcome file
				welcomeMsg := fmt.Sprintf("Welcome to Node %d!\n", node.ID)
				if err := remote.CopyFile("welcome.txt", "/home/ubuntu/welcome.txt", true); err != nil {
					// Handle errors gracefully - create the file directly if copy fails
					cmd := fmt.Sprintf("echo '%s' > /home/ubuntu/welcome.txt", welcomeMsg)
					if _, _, err := remote.RunCommand(cmd, 10*time.Second); err != nil {
						log.Printf("Warning: failed to create welcome file: %v", err)
						// Continue execution, don't fail the entire process
					}
				}

				// Check if Docker is already installed
				_, _, err := remote.RunCommand("which docker", 10*time.Second)
				if err != nil {
					log.Println("Docker not found, attempting installation...")
					// Try to install Docker if not already installed
					_, _, err := remote.RunCommand("apt-get update && apt-get install -y docker.io", 5*time.Minute)
					if err != nil {
						log.Printf("Warning: Docker installation failed: %v", err)
						log.Println("Continuing with post-installation despite Docker installation failure")
						// Continue execution, don't fail the entire process
					} else {
						log.Println("Docker successfully installed")
					}
				} else {
					log.Println("Docker is already installed")
				}

				// Configure Docker and system services - continue even if some commands fail
				cmds := []string{
					"systemctl enable docker || true",
					"systemctl start docker || true",
					"usermod -aG docker ubuntu || true",
				}
				for _, cmd := range cmds {
					stdout, _, err := remote.RunCommand(cmd, 30*time.Second)
					if err != nil {
						log.Printf("Warning: command '%s' failed: %v", cmd, err)
						log.Printf("Command output: %s", stdout)
						// Continue with next command, don't fail the entire process
					}
				}

				return nil
			},
		},
	}

	// Run post-installation
	if err := postInstaller.Configure(postConfig); err != nil {
		log.Fatalf("Failed to configure post-installer: %v", err)
	}

	if err := postInstaller.Run(tpiCtx, executor); err != nil {
		log.Fatalf("Post-installation configuration failed: %v", err)
	}

	log.Printf("Post-installation configuration completed successfully for node %d", node.ID)
	log.Println("All operations completed successfully!")
}
