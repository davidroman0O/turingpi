package tftpi

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/pkg/v2/config"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
)

// ShellBMCExecutor implements the bmc.CommandExecutor interface using local shell commands
type ShellBMCExecutor struct {
	bmcIP       string
	bmcUsername string
	bmcPassword string
}

// ExecuteCommand runs a command for BMC operations
func (e *ShellBMCExecutor) ExecuteCommand(cmd string) (string, string, error) {
	// Create the command with the BMC credentials
	shellCmd := fmt.Sprintf("export BMC_IP=%s BMC_USER=%s BMC_PASS=%s && %s",
		e.bmcIP, e.bmcUsername, e.bmcPassword, cmd)

	// Execute the command
	execCmd := exec.Command("sh", "-c", shellCmd)
	output, err := execCmd.CombinedOutput()
	outputStr := string(output)

	// Split output into stdout and stderr (simplistic approach)
	parts := strings.Split(outputStr, "\n")
	stdoutLines := []string{}
	stderrLines := []string{}

	for _, line := range parts {
		if strings.HasPrefix(line, "ERROR:") || strings.HasPrefix(line, "WARN:") {
			stderrLines = append(stderrLines, line)
		} else {
			stdoutLines = append(stdoutLines, line)
		}
	}

	stdout := strings.Join(stdoutLines, "\n")
	stderr := strings.Join(stderrLines, "\n")

	return stdout, stderr, err
}

// TuringPiProvider is the main entry point for the Turing Pi toolkit
type TuringPiProvider struct {
	// Configuration
	config     *config.Config
	configFile *config.ConfigFile

	// Store for configuration values
	configStore *store.KVStore

	// Tool providers for each cluster
	toolProviders map[string]*tools.TuringPiToolProvider

	// Workflow runner
	*gostage.Runner
}

// Option defines configuration options for TuringPiProvider
type Option func(*TuringPiProvider) error

// WithConfigFile loads configuration from a file
func WithConfigFile(path string) Option {
	return func(t *TuringPiProvider) error {
		configFile, err := config.LoadConfigFile(path)
		if err != nil {
			return fmt.Errorf("failed to load config file: %w", err)
		}

		t.configFile = configFile
		return nil
	}
}

// WithClusterConfig sets configuration directly from a cluster config
func WithClusterConfig(cluster *config.ClusterConfig) Option {
	return func(t *TuringPiProvider) error {
		if t.configFile == nil {
			t.configFile = &config.ConfigFile{
				Clusters: []config.ClusterConfig{},
			}
		}

		t.configFile.Clusters = append(t.configFile.Clusters, *cluster)
		return nil
	}
}

// New creates a new TuringPiProvider
func New(opts ...Option) (*TuringPiProvider, error) {
	// Create basic configuration
	cfg, err := config.New()
	if err != nil {
		return nil, err
	}

	// Initialize provider
	provider := &TuringPiProvider{
		config:        cfg,
		configStore:   store.NewKVStore(),
		toolProviders: make(map[string]*tools.TuringPiToolProvider),
		Runner:        gostage.NewRunner(),
	}

	// Apply options
	for _, opt := range opts {
		if err := opt(provider); err != nil {
			return nil, err
		}
	}

	// Map configuration to store
	if provider.configFile != nil {
		if err := config.MapConfigToStore(provider.configFile, provider.configStore); err != nil {
			return nil, fmt.Errorf("failed to map config to store: %w", err)
		}

		// Initialize tool providers for each cluster
		if err := provider.initializeToolProviders(); err != nil {
			return nil, fmt.Errorf("failed to initialize tool providers: %w", err)
		}
	}

	return provider, nil
}

// initializeToolProviders creates a tool provider for each cluster
func (t *TuringPiProvider) initializeToolProviders() error {
	// Determine global cache directory
	globalCacheDir := "/tmp/turingpi/cache" // Default
	if t.configFile.Global.Cache.LocalDir != "" {
		globalCacheDir = t.configFile.Global.Cache.LocalDir
	} else {
		// Try to get from config
		cacheVal, err := config.Get[string](t.config, "cacheDir")
		if err == nil && cacheVal != "" {
			globalCacheDir = cacheVal
		}
	}

	for i, cluster := range t.configFile.Clusters {
		// Create BMC executor for this cluster
		bmcExecutor := &ShellBMCExecutor{
			bmcIP:       cluster.BMC.IP,
			bmcUsername: cluster.BMC.Username,
			bmcPassword: cluster.BMC.Password,
		}

		// Determine cache directory - cluster override or global
		cacheDir := globalCacheDir
		if cluster.Cache != nil && cluster.Cache.LocalDir != "" {
			cacheDir = cluster.Cache.LocalDir
		}

		// Create tool provider config
		toolConfig := &tools.TuringPiToolConfig{
			BMCExecutor: bmcExecutor,
			CacheDir:    cacheDir,
		}

		// Use first node's SSH config for remote cache if available
		var remoteNode *config.ClusterNodeConfig
		if len(cluster.Nodes) > 0 {
			remoteNode = &cluster.Nodes[0]
		}

		// Set up remote cache if we have a node with IP
		if remoteNode != nil && remoteNode.IP != "" {
			var sshUser, sshPassword string

			// Note: SSH port is available in configs but RemoteCacheConfig doesn't have a Port field
			// In a real implementation, the port would be passed to the SSH client configuration

			// Use node-specific SSH config if available
			if remoteNode.SSH != nil {
				sshUser = remoteNode.SSH.User
				sshPassword = remoteNode.SSH.Password
				// Port is available here but we can't use it directly
				// if remoteNode.SSH.Port > 0 {
				//     sshPort = remoteNode.SSH.Port
				// }
			} else if t.configFile.Global.DefaultSSH != nil {
				// Fall back to global SSH config
				sshUser = t.configFile.Global.DefaultSSH.User
				sshPassword = t.configFile.Global.DefaultSSH.Password
				// Port is available here too
				// if t.configFile.Global.DefaultSSH.Port > 0 {
				//     sshPort = t.configFile.Global.DefaultSSH.Port
				// }
			} else {
				// Use reasonable defaults
				sshUser = "root"
			}

			// Determine remote path
			remotePath := "/var/cache/turingpi"
			if cluster.Cache != nil && cluster.Cache.RemoteDir != "" {
				remotePath = cluster.Cache.RemoteDir
			} else if t.configFile.Global.Cache.RemoteDir != "" {
				remotePath = t.configFile.Global.Cache.RemoteDir
			}

			// Configure remote cache
			toolConfig.RemoteCache = &tools.RemoteCacheConfig{
				Host:       remoteNode.IP,
				User:       sshUser,
				Password:   sshPassword,
				RemotePath: remotePath,
			}
		}

		// Create a tool provider for this cluster
		toolProvider, err := tools.NewTuringPiToolProvider(toolConfig)
		if err != nil {
			return fmt.Errorf("failed to create tool provider for cluster %s: %w", cluster.Name, err)
		}

		// Store the tool provider
		t.toolProviders[cluster.Name] = toolProvider

		// Add to store for later access in workflows
		clusterPrefix := fmt.Sprintf("turingpi.cluster.%d", i+1)
		t.configStore.Put(fmt.Sprintf("%s.toolProvider", clusterPrefix), toolProvider)
	}

	return nil
}

// Execute runs a workflow with configuration injected from the specified cluster and node
func (t *TuringPiProvider) Execute(ctx context.Context, workflow *gostage.Workflow, logger gostage.Logger, clusterName string, nodeID int) error {
	// Initialize workflow store if not present
	if workflow.Store == nil {
		workflow.Store = store.NewKVStore()
	}

	// Validate the target cluster exists
	if clusterName == "" {
		return fmt.Errorf("no cluster specified, must provide a valid cluster name")
	}

	provider, exists := t.toolProviders[clusterName]
	if !exists {
		return fmt.Errorf("cluster '%s' not found", clusterName)
	}

	// Validate the node ID
	if nodeID <= 0 {
		return fmt.Errorf("invalid node ID: %d, must be greater than 0", nodeID)
	}

	// Important: Add the tool provider directly to the workflow store first
	// This ensures it's available immediately for the workflow
	workflow.Store.Put("$tools", provider)

	// Set the current node as the target node for all actions
	workflow.Store.Put("turingpi.workflow.current_node", nodeID)

	// Create a store with only the configuration for the target cluster and node
	filteredStore := store.NewKVStore()

	// Find the cluster and its index
	var clusterIndex int
	var targetClusterConfig *config.ClusterConfig

	for i, cluster := range t.configFile.Clusters {
		if cluster.Name == clusterName {
			clusterIndex = i + 1 // 1-based indexing
			targetClusterConfig = &cluster
			break
		}
	}

	if targetClusterConfig == nil {
		return fmt.Errorf("failed to find configuration for cluster '%s'", clusterName)
	}

	// Add the target cluster's configuration
	clusterPrefix := fmt.Sprintf("turingpi.cluster.%d", clusterIndex)

	// Store cluster details
	filteredStore.Put(fmt.Sprintf("%s.name", clusterPrefix), targetClusterConfig.Name)

	// Store BMC details
	bmcPrefix := fmt.Sprintf("%s.bmc", clusterPrefix)
	filteredStore.Put(fmt.Sprintf("%s.ip", bmcPrefix), targetClusterConfig.BMC.IP)
	filteredStore.Put(fmt.Sprintf("%s.user", bmcPrefix), targetClusterConfig.BMC.Username)
	filteredStore.Put(fmt.Sprintf("%s.password", bmcPrefix), targetClusterConfig.BMC.Password)

	// Store the active cluster and tool provider for easy access
	filteredStore.Put("turingpi.targetCluster", clusterName)
	filteredStore.Put("turingpi.clusterIndex", clusterIndex)

	// Store the current node for all actions
	filteredStore.Put("turingpi.workflow.current_node", nodeID)

	// Add specific node information if available in the config
	var foundNode bool
	for _, node := range targetClusterConfig.Nodes {
		if node.ID == nodeID {
			nodePrefix := fmt.Sprintf("turingpi.node.%d", nodeID)

			// Store node details
			filteredStore.Put(fmt.Sprintf("%s.name", nodePrefix), node.Name)
			filteredStore.Put(fmt.Sprintf("%s.ip", nodePrefix), node.IP)
			filteredStore.Put(fmt.Sprintf("%s.board", nodePrefix), string(node.Board))
			filteredStore.Put(fmt.Sprintf("%s.cluster", nodePrefix), targetClusterConfig.Name)

			foundNode = true
			break
		}
	}

	// Add warning if node not found in config but still continue
	if !foundNode {
		logger.Warn("Node %d not found in cluster '%s' configuration, continuing with limited information",
			nodeID, clusterName)
	}

	// We already put this directly in the workflow store for immediate availability
	filteredStore.Put("$tools", provider)

	// Inject the filtered configuration into the workflow store
	copied, overwritten, err := workflow.Store.CopyFromWithOverwrite(filteredStore)
	if err != nil {
		return fmt.Errorf("failed to inject configuration: %w", err)
	}

	logger.Info("Injected %d configuration values for cluster '%s', node %d (%d existing values overwritten)",
		copied, clusterName, nodeID, overwritten)

	// Execute workflow
	return t.Runner.Execute(ctx, workflow, logger)
}

// GetToolProvider returns the tool provider for a specific cluster
func (t *TuringPiProvider) GetToolProvider(clusterName string) *tools.TuringPiToolProvider {
	return t.toolProviders[clusterName]
}

// GetClusterNodes returns the list of node IDs in the targeted cluster
func GetClusterNodes(ctx *gostage.ActionContext) ([]int, error) {
	nodeIDs, err := store.Get[[]int](ctx.Store, "turingpi.clusterNodes")
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster nodes: %w", err)
	}
	return nodeIDs, nil
}
