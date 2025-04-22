package ubuntu

import (
	"fmt"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/turingpi/pkg/v2/workflows/actions/common"
	ubuntuStages "github.com/davidroman0O/turingpi/pkg/v2/workflows/stages/ubuntu"
)

// NetworkConfig holds the network configuration for a node
type NetworkConfig struct {
	Hostname   string   // Hostname for the node
	IPCIDR     string   // IP address with CIDR suffix
	Gateway    string   // Gateway IP address
	DNSServers []string // List of DNS server IP addresses
}

type UbuntuRK1DeploymentOptions struct {
	SourceImagePath string // Base Ubuntu image path
	NetworkConfig   *NetworkConfig
	NewPassword     string // cannot be `ubuntu` or less than 6 characters
}

// CreateUbuntuRK1Deployment creates a workflow for deploying Ubuntu to a RK1 node
func CreateUbuntuRK1Deployment(nodeID int, options UbuntuRK1DeploymentOptions) *gostage.Workflow {
	workflow := gostage.NewWorkflow(
		fmt.Sprintf("rk1-ubuntu-node-%d-deployment", nodeID),
		fmt.Sprintf("RK1 Ubuntu Deployment for Node %d", nodeID),
		fmt.Sprintf("Complete workflow for deploying Ubuntu to RK1 Node %d", nodeID),
	)

	// Initialize workflow store with options
	workflow.Store.Put("SourceImagePath", options.SourceImagePath)

	// Store password for post-installation
	if options.NewPassword != "" {
		// Validate password requirements
		if options.NewPassword == "ubuntu" {
			// Default Ubuntu password, not secure
			workflow.Store.Put("NewPassword", "turingpi123!") // Default secure password
		} else if len(options.NewPassword) < 6 {
			// Too short, use default
			workflow.Store.Put("NewPassword", "turingpi123!") // Default secure password
		} else {
			workflow.Store.Put("NewPassword", options.NewPassword)
		}
	} else {
		// No password specified, use default
		workflow.Store.Put("NewPassword", "turingpi123!") // Default secure password
	}

	if options.NetworkConfig != nil {
		workflow.Store.Put("Hostname", options.NetworkConfig.Hostname)
		workflow.Store.Put("IPCIDR", options.NetworkConfig.IPCIDR)
		workflow.Store.Put("Gateway", options.NetworkConfig.Gateway)
		workflow.Store.Put("DNSServers", fmt.Sprintf("%v", options.NetworkConfig.DNSServers))
	}

	// Add initialization stage to set up node ID
	initStage := gostage.NewStage(
		"init",
		"Initialization",
		"Set up workflow parameters",
	)
	initStage.AddAction(common.NewSetCurrentNodeAction(nodeID))
	workflow.AddStage(initStage)

	// Add node reset stage
	// workflow.AddStage(node.CreateResetStage())

	// Add Ubuntu image preparation stage
	workflow.AddStage(ubuntuStages.CreateImagePreparationStage())

	// // Add Ubuntu image deployment stage
	// workflow.AddStage(ubuntuStages.CreateImageDeploymentStage())

	// // Add Ubuntu post-installation stage for password configuration
	// workflow.AddStage(ubuntuStages.CreatePostInstallationStage())

	return workflow
}
