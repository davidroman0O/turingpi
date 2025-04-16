package cmd

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi"
	"github.com/davidroman0O/turingpi/pkg/tpi/ubuntu"
	"github.com/spf13/cobra"
)

// Variables to store flag values for workflow command
var (
	workflowNodeID       int
	workflowBaseImage    string
	workflowNodeIP       string
	workflowNodeCIDR     string
	workflowNodeHostname string
	workflowNodeGateway  string
	workflowNodeDNS      string
)

// workflowCmd represents the workflow command
var workflowCmd = &cobra.Command{
	Use:   "workflow",
	Short: "Run a complete node setup workflow (all phases)",
	Long: `Executes all three phases of the node setup workflow:
1. Image customization - prepares an image with network settings
2. OS installation - flashes the customized image to the node
3. Post-installation - performs configuration after first boot

This is a powerful end-to-end command that automates the entire 
node setup process. Make sure to provide all required parameters.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Starting complete workflow execution...")

		// --- Input validation ---
		if workflowNodeID < 1 || workflowNodeID > 4 {
			fmt.Fprintf(os.Stderr, "Error: Invalid node ID %d. Please specify a node between 1 and 4.\n", workflowNodeID)
			os.Exit(1)
		}

		if workflowBaseImage == "" {
			fmt.Fprintln(os.Stderr, "Error: Base image path is required (--base-image).")
			os.Exit(1)
		}

		if workflowNodeIP == "" {
			fmt.Fprintln(os.Stderr, "Error: Node IP is required (--node-ip).")
			os.Exit(1)
		}

		if workflowNodeGateway == "" {
			fmt.Fprintln(os.Stderr, "Error: Gateway IP is required (--gateway).")
			os.Exit(1)
		}

		if workflowNodeDNS == "" {
			// Use Cloudflare and Google DNS as defaults
			workflowNodeDNS = "1.1.1.1,8.8.8.8"
			fmt.Println("No DNS servers specified, using defaults:", workflowNodeDNS)
		}

		// Default hostname if not provided
		if workflowNodeHostname == "" {
			workflowNodeHostname = fmt.Sprintf("tpi-node%d", workflowNodeID)
			fmt.Println("No hostname specified, using default:", workflowNodeHostname)
		}

		// Default CIDR if not provided
		cidrSuffix := "/24"
		if workflowNodeCIDR != "" {
			cidrSuffix = fmt.Sprintf("/%s", workflowNodeCIDR)
		}
		ipCIDR := fmt.Sprintf("%s%s", workflowNodeIP, cidrSuffix)

		// --- Configuration ---
		log.Println("Creating Turing Pi configuration...")
		var nodeID tpi.NodeID
		switch workflowNodeID {
		case 1:
			nodeID = tpi.Node1
		case 2:
			nodeID = tpi.Node2
		case 3:
			nodeID = tpi.Node3
		case 4:
			nodeID = tpi.Node4
		}

		// Convert comma-separated DNS to slice
		var dnsServers []string
		for _, dns := range strings.Split(workflowNodeDNS, ",") {
			dns = strings.TrimSpace(dns)
			if dns != "" {
				dnsServers = append(dnsServers, dns)
			}
		}

		// Prepare node config
		nodeConfig := &tpi.NodeConfig{
			IP:    ipCIDR,
			Board: tpi.RK1, // Assuming RK1 for now, could make this configurable
		}

		// Prepare TPI config based on command line flags
		cfg := tpi.TPIConfig{
			IP:          bmcHost,     // Use global flag from root.go
			BMCUser:     bmcUser,     // Use global flag from root.go
			BMCPassword: bmcPassword, // Use global flag from root.go
		}

		// Set the appropriate node config in the TPIConfig
		switch nodeID {
		case tpi.Node1:
			cfg.Node1 = nodeConfig
		case tpi.Node2:
			cfg.Node2 = nodeConfig
		case tpi.Node3:
			cfg.Node3 = nodeConfig
		case tpi.Node4:
			cfg.Node4 = nodeConfig
		}

		// --- Initialize Executor ---
		log.Println("Initializing Turing Pi executor...")
		executor, err := tpi.NewTuringPi(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error initializing TPI executor: %v\n", err)
			os.Exit(1)
		}

		// --- Define Network Config ---
		networkConfig := tpi.NetworkConfig{
			IPCIDR:     ipCIDR,
			Hostname:   workflowNodeHostname,
			Gateway:    workflowNodeGateway,
			DNSServers: dnsServers,
		}

		// --- Define Workflow Function ---
		log.Println("Defining workflow function...")
		workflowFunc := func(ctx tpi.Context, node tpi.Node) error {
			log.Printf("Starting workflow execution for Node %d (%s)", node.ID, node.Config.Board)

			// --- Phase 1: Image Customization ---
			log.Println("Executing Phase 1: Image Customization...")
			imageBuilder := ubuntu.NewImageBuilder(node.ID)

			// Configure the image builder
			imageBuilder = imageBuilder.WithBaseImage(workflowBaseImage)
			imageBuilder = imageBuilder.WithNetworkConfig(networkConfig)

			// Add pre-install customizations
			imageBuilder = imageBuilder.WithPreInstall(func(image tpi.ImageModifier) error {
				// Add a custom motd
				motdContent := []byte(fmt.Sprintf("Welcome to %s!\nThis node was automatically configured by Turing Pi CLI.\n",
					workflowNodeHostname))
				image.WriteFile("/etc/motd", motdContent, 0644)
				return nil
			})

			// Run Phase 1
			imageResult, err := imageBuilder.Run(ctx, executor)
			if err != nil {
				return fmt.Errorf("phase 1 (Image Customization) failed: %w", err)
			}
			log.Printf("Phase 1 completed successfully. Image at: %s", imageResult.ImagePath)

			// --- Phase 2: OS Installation ---
			log.Println("Executing Phase 2: OS Installation...")
			osInstaller := ubuntu.NewOSInstaller(node.ID, tpi.UbuntuInstallConfig{
				// Default Ubuntu password (will be changed in post-install)
				InitialUserPassword: "ubuntu",
			})

			// Run Phase 2
			err = osInstaller.UsingImage(imageResult).Run(ctx, executor)
			if err != nil {
				return fmt.Errorf("phase 2 (OS Installation) failed: %w", err)
			}
			log.Printf("Phase 2 completed successfully. OS installed on node %d", node.ID)

			// --- Phase 3: Post Installation ---
			log.Println("Executing Phase 3: Post-Installation Configuration...")
			postInstaller := ubuntu.NewPostInstaller(node.ID)

			// Define post-installation actions
			postInstaller = postInstaller.RunActions(func(local tpi.LocalRuntime, remote tpi.UbuntuRuntime) error {
				// Wait a bit for the node to fully boot
				time.Sleep(30 * time.Second)

				// Check if the node is reachable
				stdout, _, err := remote.RunCommand("hostname", 10*time.Second)
				if err != nil {
					return fmt.Errorf("failed to connect to node: %w", err)
				}
				log.Printf("Node is reachable. Hostname: %s", stdout)

				// Run system update
				_, _, err = remote.RunCommand("sudo apt update && sudo apt upgrade -y", 5*time.Minute)
				if err != nil {
					log.Printf("Warning: System update failed: %v", err)
					// Continue despite error
				} else {
					log.Println("System updated successfully")
				}

				// Install some useful tools
				_, _, err = remote.RunCommand("sudo apt install -y htop iotop fail2ban", 2*time.Minute)
				if err != nil {
					log.Printf("Warning: Tools installation failed: %v", err)
					// Continue despite error
				} else {
					log.Println("Additional tools installed")
				}

				// Create a report file
				reportPath := filepath.Join(executor.GetCacheDir(), "node_report.txt")
				reportContent := []byte(fmt.Sprintf("Turing Pi Node Setup Report\n-------------------------\nNode: %d\nHostname: %s\nIP: %s\nBoard: RK1\nSetup Time: %s\n",
					node.ID, workflowNodeHostname, workflowNodeIP, time.Now().Format(time.RFC3339)))

				if err := local.WriteFile(reportPath, reportContent, 0644); err != nil {
					return fmt.Errorf("failed to create report file: %w", err)
				}

				log.Printf("Node %d setup completed successfully!", node.ID)
				return nil
			})

			// Run Phase 3
			err = postInstaller.Run(ctx, executor)
			if err != nil {
				return fmt.Errorf("phase 3 (Post-Installation) failed: %w", err)
			}

			log.Printf("Workflow for Node %d completed successfully!", node.ID)
			return nil
		}

		// --- Execute Workflow ---
		log.Println("Starting workflow execution...")
		wrappedWorkflowFunc := func(ctx tpi.Context, cluster tpi.Cluster, node tpi.Node) error {
			return workflowFunc(ctx, node)
		}
		executeWorkflow := executor.Run(wrappedWorkflowFunc)
		err = executeWorkflow(context.Background(), nodeID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Workflow execution failed: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("Complete workflow for Node %d finished successfully!\n", workflowNodeID)
	},
}

func init() {
	rootCmd.AddCommand(workflowCmd)

	// Add workflow-specific flags
	workflowCmd.Flags().IntVarP(&workflowNodeID, "node", "n", 0, "Target node number (1-4) (required)")
	workflowCmd.Flags().StringVarP(&workflowBaseImage, "base-image", "b", "", "Path to base OS image (.img.xz) (required)")
	workflowCmd.Flags().StringVar(&workflowNodeIP, "node-ip", "", "Static IP for the node (e.g., 192.168.1.101) (required)")
	workflowCmd.Flags().StringVar(&workflowNodeCIDR, "cidr", "24", "CIDR suffix for the IP (default: 24)")
	workflowCmd.Flags().StringVar(&workflowNodeHostname, "hostname", "", "Hostname for the node (default: tpi-node<N>)")
	workflowCmd.Flags().StringVar(&workflowNodeGateway, "gateway", "", "Gateway IP address (required)")
	workflowCmd.Flags().StringVar(&workflowNodeDNS, "dns", "", "Comma-separated DNS server IPs (default: 1.1.1.1,8.8.8.8)")

	// Mark required flags
	_ = workflowCmd.MarkFlagRequired("node")
	_ = workflowCmd.MarkFlagRequired("base-image")
	_ = workflowCmd.MarkFlagRequired("node-ip")
	_ = workflowCmd.MarkFlagRequired("gateway")
}
