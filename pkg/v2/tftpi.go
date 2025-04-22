package tftpi

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidroman0O/gostage"
	"github.com/davidroman0O/gostage/store"
	"github.com/davidroman0O/turingpi/pkg/v2/bmc"
	"github.com/davidroman0O/turingpi/pkg/v2/config"
	"github.com/davidroman0O/turingpi/pkg/v2/container"
	"github.com/davidroman0O/turingpi/pkg/v2/keys"
	"github.com/davidroman0O/turingpi/pkg/v2/tools"
)

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

	// We will create a container for each workflow if we are running in Docker mode
	// Eitherway it will still create a tmp dir for the workflow
	// If we have a non-linux machine OR forced docker mode, we will create a container which will be used for one workflow
	provider.Runner.Use(func(next gostage.RunnerFunc) gostage.RunnerFunc {
		return func(ctx context.Context, w *gostage.Workflow, logger gostage.Logger) error {
			logger.Info("Starting workflow: %s", w.Name)

			clusterName, err := store.Get[string](w.Store, "turingpi.targetCluster")
			if err != nil {
				return fmt.Errorf("failed to get target cluster: %w", err)
			}

			localName := fmt.Sprintf("tftpi-workflow-%v", w.ID)

			localProvider := provider.toolProviders[clusterName] // TODO: make sure we don't have locks/race

			localCacheDir := localProvider.GetLocalCache().Location()

			tmpDir, err := localProvider.GetTmpCache().CreateTempDir(ctx, localName)
			if err != nil {
				return fmt.Errorf("failed to create temp directory: %w", err)
			}

			absLocalCacheDir, err := filepath.Abs(localCacheDir)
			if err != nil {
				return fmt.Errorf("failed to get absolute path for local cache directory: %w", err)
			}

			absTmpDir, err := filepath.Abs(tmpDir)
			if err != nil {
				return fmt.Errorf("failed to get absolute path for temp directory: %w", err)
			}

			w.Store.Put("workflow.cache.dir", absLocalCacheDir)
			w.Store.Put("workflow.tmp.dir", absTmpDir)

			ctn, err := localProvider.GetContainerTool().CreateContainer(ctx, container.ContainerConfig{
				Image: "alpine:latest", // TODO: make this configurable
				Name:  localName,
				Mounts: map[string]string{
					absLocalCacheDir: "/cache",
					absTmpDir:        "/tmp",
				},
				InitCommands: [][]string{ // TODO: make this configurable
					{"apk", "add", "--no-cache", "xz"},
				},
				Command: []string{"sleep", "infinity"},
			})

			if err != nil {
				return fmt.Errorf("failed to create container: %w", err)
			}

			if ctx != nil && ctn.ID() != "" {
				logger.Info("Container created successfully: %s", ctn.ID())
				w.Store.Put("workflow.container.id", ctn.ID())
			} else {
				return fmt.Errorf("No container created")
			}

			// Add defer to ensure cleanup happens no matter how we exit this function
			defer func() {
				cleanupErr := ctn.Cleanup(ctx)
				if cleanupErr != nil {
					logger.Error("Failed to clean up container: %v", cleanupErr)
				} else {
					logger.Info("Container cleaned up successfully")
				}
			}()

			err = next(ctx, w, logger)

			// Remove tmp dir
			if err := os.RemoveAll(tmpDir); err != nil {
				logger.Error("Failed to remove temp directory: %v", err)
			}

			logger.Info("Completed workflow: %s", w.Name)
			return err
		}
	})

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
		// BMC config is required
		if cluster.BMC.IP == "" {
			return fmt.Errorf("cluster %s has no BMC IP address", cluster.Name)
		}

		if cluster.BMC.Username == "" {
			return fmt.Errorf("cluster %s has no BMC username", cluster.Name)
		}

		if cluster.BMC.Password == "" {
			return fmt.Errorf("cluster %s has no BMC password", cluster.Name)
		}
		// Create BMC executor for this cluster
		bmcExecutor := bmc.NewSSHExecutor(cluster.BMC.IP, 22, cluster.BMC.Username, cluster.BMC.Password)

		// Determine cache directory - cluster override or global
		cacheDir := globalCacheDir
		if cluster.Cache != nil && cluster.Cache.LocalDir != "" {
			cacheDir = cluster.Cache.LocalDir
		}

		// Create tool provider config
		toolConfig := &tools.TuringPiToolConfig{
			BMCExecutor:  bmcExecutor,
			CacheDir:     cacheDir,
			TempCacheDir: cluster.Cache.TempDir,
		}

		// Set up remote cache using BMC connection details
		// BMC and cluster controller are typically the same device
		remotePath := "/var/cache/turingpi" // Default remote path
		if cluster.Cache != nil && cluster.Cache.RemoteDir != "" {
			remotePath = cluster.Cache.RemoteDir
		} else if t.configFile.Global.Cache.RemoteDir != "" {
			remotePath = t.configFile.Global.Cache.RemoteDir
		}

		// Configure remote cache using BMC credentials - simpler and more direct approach
		toolConfig.RemoteCache = &tools.RemoteCacheConfig{
			Host:       cluster.BMC.IP,       // Use BMC IP for SSH
			User:       cluster.BMC.Username, // Use BMC username
			Password:   cluster.BMC.Password, // Use BMC password
			RemotePath: remotePath,
			Port:       22, // Default SSH port
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

	// Add verbose logging about the provider and its BMC tool
	logger.Info("Provider before storing in workflow: %v", provider)
	logger.Info("BMC tool before storing (should not be nil): %v", provider.GetBMCTool())

	// Validate that the provider has a working BMC tool
	if provider.GetBMCTool() == nil {
		logger.Error("BMC tool is nil in the provider - this indicates an initialization problem")

		// Print provider details
		logger.Info("Provider details - BMC tool: %v, Image tool: %v, Container tool: %v, LocalCache: %v, RemoteCache: %v",
			provider.GetBMCTool() != nil,
			provider.GetOperationsTool() != nil,
			provider.GetContainerTool() != nil,
			provider.GetLocalCache() != nil,
			provider.GetRemoteCache() != nil)

		return fmt.Errorf("BMC tool is not initialized in the provider")
	}

	// Validate the node ID
	if nodeID <= 0 {
		return fmt.Errorf("invalid node ID: %d, must be greater than 0", nodeID)
	}

	// Add the tool provider directly to the workflow store
	// This ensures it's available immediately for the workflow
	workflow.Store.Put(keys.ToolsProvider, provider)

	// Verify the provider was correctly stored by retrieving it again
	storedProvider, err := store.Get[*tools.TuringPiToolProvider](workflow.Store, keys.ToolsProvider)
	if err != nil {
		logger.Error("Failed to retrieve stored provider: %v", err)
	} else {
		logger.Info("Provider after storing and retrieving: %v", storedProvider)
		logger.Info("BMC tool after storing (should not be nil): %v", storedProvider.GetBMCTool())

		// Double-check BMC tool is not nil
		if storedProvider.GetBMCTool() == nil {
			logger.Error("BMC tool is nil after storing! Store may be performing a shallow copy")
		}
	}

	// Set the current node as the target node for all actions
	workflow.Store.Put(keys.CurrentNodeID, nodeID)

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

	// Add the target cluster's configuration directly to the workflow store
	clusterPrefix := fmt.Sprintf("turingpi.cluster.%d", clusterIndex)

	// Store cluster details
	workflow.Store.Put(fmt.Sprintf("%s.name", clusterPrefix), targetClusterConfig.Name)

	// Store BMC details
	bmcPrefix := fmt.Sprintf("%s.bmc", clusterPrefix)
	workflow.Store.Put(fmt.Sprintf("%s.ip", bmcPrefix), targetClusterConfig.BMC.IP)
	workflow.Store.Put(fmt.Sprintf("%s.user", bmcPrefix), targetClusterConfig.BMC.Username)
	workflow.Store.Put(fmt.Sprintf("%s.password", bmcPrefix), targetClusterConfig.BMC.Password)

	// Store the active cluster and tool provider for easy access
	workflow.Store.Put("turingpi.targetCluster", clusterName)
	workflow.Store.Put("turingpi.clusterIndex", clusterIndex)

	// Add specific node information if available in the config
	var foundNode bool
	for _, node := range targetClusterConfig.Nodes {
		if node.ID == nodeID {
			nodePrefix := fmt.Sprintf("turingpi.node.%d", nodeID)

			// Store node details
			workflow.Store.Put(fmt.Sprintf("%s.name", nodePrefix), node.Name)
			workflow.Store.Put(fmt.Sprintf("%s.ip", nodePrefix), node.IP)
			workflow.Store.Put(fmt.Sprintf("%s.board", nodePrefix), string(node.Board))
			workflow.Store.Put(fmt.Sprintf("%s.cluster", nodePrefix), targetClusterConfig.Name)

			foundNode = true
			break
		}
	}

	workflow.Store.Put("turingpi.cache.local.dir", provider.GetLocalCache().Location())

	// Add warning if node not found in config but still continue
	if !foundNode {
		logger.Warn("Node %d not found in cluster '%s' configuration, continuing with limited information",
			nodeID, clusterName)
	}

	logger.Info("Injected configuration values for cluster '%s', node %d",
		clusterName, nodeID)

	// Execute workflow
	return t.Runner.Execute(ctx, workflow, logger)
}

// GetToolProvider returns the tool provider for a specific cluster
func (t *TuringPiProvider) GetToolProvider(clusterName string) *tools.TuringPiToolProvider {
	return t.toolProviders[clusterName]
}

// GetClusterNodes returns the list of node IDs in the targeted cluster
func GetClusterNodes(ctx *gostage.ActionContext) ([]int, error) {
	nodeIDs, err := store.Get[[]int](ctx.Store(), "turingpi.clusterNodes")
	if err != nil {
		return nil, fmt.Errorf("failed to get cluster nodes: %w", err)
	}
	return nodeIDs, nil
}
