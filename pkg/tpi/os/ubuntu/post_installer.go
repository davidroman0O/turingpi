package ubuntu

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/davidroman0O/turingpi/pkg/tpi"
)

// PostInstallConfig holds configuration for post-installation setup.
type PostInstallConfig struct {
	BaseConfig
	RuntimeConfig
	NetworkingConfig

	// SSH configuration
	SSHPort    int
	SSHKeyPath string

	// System configuration
	Username     string
	Password     string
	Timezone     string
	LocaleConfig string

	// Additional packages to install
	Packages []string

	// Custom scripts to run
	PostInstallScripts []string

	// Custom runtime customization function
	RuntimeCustomizationFunc func(localRuntime tpi.Runtime, remoteRuntime tpi.Runtime) error

	// Force flag to continue with standard operations even if custom function succeeds
	Force bool
}

// UbuntuPostInstallerBuilder defines the configuration for Phase 3: Post-Installation Setup for Ubuntu.
type UbuntuPostInstallerBuilder struct {
	nodeID tpi.NodeID
	config *PostInstallConfig
}

// NewPostInstaller creates a new builder for post-installation setup of Ubuntu.
func NewPostInstaller(nodeID tpi.NodeID) *UbuntuPostInstallerBuilder {
	return &UbuntuPostInstallerBuilder{
		nodeID: nodeID,
	}
}

// Configure accepts an OS-specific post-installation configuration.
func (b *UbuntuPostInstallerBuilder) Configure(config interface{}) error {
	postConfig, ok := config.(*PostInstallConfig)
	if !ok {
		return fmt.Errorf("expected *PostInstallConfig, got %T", config)
	}

	// Set defaults if not specified
	if postConfig.SSHPort == 0 {
		postConfig.SSHPort = 22
	}
	if postConfig.Username == "" {
		postConfig.Username = "ubuntu"
	}
	if postConfig.Password == "" {
		return fmt.Errorf("password must be specified")
	}

	b.config = postConfig
	return nil
}

// Run executes the post-installation setup phase.
func (b *UbuntuPostInstallerBuilder) Run(ctx tpi.Context, cluster tpi.Cluster) error {
	log.Printf("--- Starting Phase 3: Post-Installation for Node %d (Ubuntu) ---", b.nodeID)

	// --- Validate Builder Config ---
	if b.config == nil {
		return fmt.Errorf("phase 3 validation failed: Configure must be called before Run")
	}

	nodeConfig := cluster.GetNodeConfig(b.nodeID)
	if nodeConfig == nil {
		return fmt.Errorf("internal error: node config not found for Node %d", b.nodeID)
	}

	// Get the node's IP information for SSH connection
	ipAddress := nodeConfig.IP
	if idx := strings.Index(ipAddress, "/"); idx != -1 {
		ipAddress = ipAddress[:idx] // Remove CIDR notation if present
	}

	// Create local runtime for local operations
	localRuntime := newLocalRuntime(cluster.GetCacheDir())

	// Create remote runtime for node operations
	remoteRuntime := newUbuntuRuntime(ipAddress, b.config.Username, b.config.Password)

	// First check if there's a custom runtime function, which takes precedence
	if b.config.RuntimeCustomizationFunc != nil {
		log.Printf("Executing custom runtime operations...")
		if err := b.config.RuntimeCustomizationFunc(localRuntime, remoteRuntime); err != nil {
			return fmt.Errorf("custom runtime operations failed: %w", err)
		}
		log.Printf("Custom runtime operations completed successfully")

		// If a custom function was provided and succeeded, we're done
		if !b.config.Force {
			log.Printf("--- Finished Phase 3: Post-Installation for Node %d (custom) ---", b.nodeID)
			return nil
		}

		// If Force is true, continue with standard operations as well
		log.Printf("Force flag set, continuing with standard post-installation operations")
	}

	// Configure locale if specified
	if b.config.LocaleConfig != "" {
		log.Printf("Configuring locale to %s...", b.config.LocaleConfig)
		cmds := []string{
			fmt.Sprintf("locale-gen %s", b.config.LocaleConfig),
			fmt.Sprintf("update-locale LANG=%s", b.config.LocaleConfig),
		}
		for _, cmd := range cmds {
			if _, _, err := remoteRuntime.RunCommand(cmd, 30*time.Second); err != nil {
				return fmt.Errorf("locale configuration failed: %w", err)
			}
		}
	}

	// Configure timezone if specified
	if b.config.Timezone != "" {
		log.Printf("Setting timezone to %s...", b.config.Timezone)
		cmd := fmt.Sprintf("timedatectl set-timezone %s", b.config.Timezone)
		if _, _, err := remoteRuntime.RunCommand(cmd, 30*time.Second); err != nil {
			return fmt.Errorf("timezone configuration failed: %w", err)
		}
	}

	// Configure SSH if key path specified
	if b.config.SSHKeyPath != "" {
		log.Printf("Configuring SSH...")
		sshDir := fmt.Sprintf("/home/%s/.ssh", b.config.Username)
		if _, _, err := remoteRuntime.RunCommand(fmt.Sprintf("mkdir -p %s", sshDir), 10*time.Second); err != nil {
			return fmt.Errorf("failed to create SSH directory: %w", err)
		}

		// Copy SSH key
		authorizedKeysPath := fmt.Sprintf("%s/authorized_keys", sshDir)
		if err := remoteRuntime.CopyFile(b.config.SSHKeyPath, authorizedKeysPath, true); err != nil {
			return fmt.Errorf("failed to copy SSH key: %w", err)
		}

		// Set proper permissions
		cmds := []string{
			fmt.Sprintf("chown -R %s:%s %s", b.config.Username, b.config.Username, sshDir),
			fmt.Sprintf("chmod 700 %s", sshDir),
			fmt.Sprintf("chmod 600 %s", authorizedKeysPath),
		}
		for _, cmd := range cmds {
			if _, _, err := remoteRuntime.RunCommand(cmd, 10*time.Second); err != nil {
				return fmt.Errorf("failed to set SSH permissions: %w", err)
			}
		}

		// Configure SSH port if non-standard
		if b.config.SSHPort != 22 {
			sshConfig := fmt.Sprintf("Port %d\nPermitRootLogin no\nPasswordAuthentication no", b.config.SSHPort)
			if _, _, err := remoteRuntime.RunCommand(fmt.Sprintf("echo '%s' > /etc/ssh/sshd_config", sshConfig), 10*time.Second); err != nil {
				return fmt.Errorf("failed to configure SSH port: %w", err)
			}

			// Restart SSH service
			if _, _, err := remoteRuntime.RunCommand("systemctl restart sshd", 30*time.Second); err != nil {
				return fmt.Errorf("failed to restart SSH service: %w", err)
			}
		}
	}

	// Install packages if specified
	if len(b.config.Packages) > 0 {
		log.Printf("Installing packages: %v", b.config.Packages)
		cmd := fmt.Sprintf("apt-get update && DEBIAN_FRONTEND=noninteractive apt-get install -y %s",
			strings.Join(b.config.Packages, " "))
		if _, _, err := remoteRuntime.RunCommand(cmd, 5*time.Minute); err != nil {
			return fmt.Errorf("package installation failed: %w", err)
		}
	}

	// Execute post-install scripts
	for i, script := range b.config.PostInstallScripts {
		log.Printf("Running post-install script %d/%d...", i+1, len(b.config.PostInstallScripts))
		if _, _, err := remoteRuntime.RunCommand(script, 5*time.Minute); err != nil {
			return fmt.Errorf("post-install script %d failed: %w", i+1, err)
		}
	}

	log.Printf("--- Finished Phase 3: Post-Installation for Node %d ---", b.nodeID)
	return nil
}

// Ensure UbuntuPostInstallerBuilder implements tpi.PostInstaller interface
var _ tpi.PostInstaller = (*UbuntuPostInstallerBuilder)(nil)
