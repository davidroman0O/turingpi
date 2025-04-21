package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/davidroman0O/gostage/store"
	"gopkg.in/yaml.v3"
)

// LoadConfigFile loads a configuration file and returns the parsed ConfigFile struct
func LoadConfigFile(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := &ConfigFile{}
	ext := filepath.Ext(path)

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse YAML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, config); err != nil {
			return nil, fmt.Errorf("failed to parse JSON config: %w", err)
		}
	default:
		return nil, fmt.Errorf("unsupported config file format: %s", ext)
	}

	return config, nil
}

// MapConfigToStore maps the configuration to a KVStore using the key naming convention
func MapConfigToStore(config *ConfigFile, kvStore *store.KVStore) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Process each cluster
	for i, cluster := range config.Clusters {
		clusterPrefix := fmt.Sprintf("turingpi.cluster.%d", i+1)

		// Store cluster details
		kvStore.Put(fmt.Sprintf("%s.name", clusterPrefix), cluster.Name)

		// Store BMC details
		bmcPrefix := fmt.Sprintf("%s.bmc", clusterPrefix)
		kvStore.Put(fmt.Sprintf("%s.ip", bmcPrefix), cluster.BMC.IP)
		kvStore.Put(fmt.Sprintf("%s.user", bmcPrefix), cluster.BMC.Username)
		kvStore.Put(fmt.Sprintf("%s.password", bmcPrefix), cluster.BMC.Password)

		// Process each node
		for j, node := range cluster.Nodes {
			// Determine node ID - use the explicit ID if available, otherwise use index+1
			nodeID := j + 1
			if node.ID > 0 {
				nodeID = node.ID
			}

			nodePrefix := fmt.Sprintf("turingpi.node.%d", nodeID)

			// Store node details
			kvStore.Put(fmt.Sprintf("%s.name", nodePrefix), node.Name)
			kvStore.Put(fmt.Sprintf("%s.ip", nodePrefix), node.IP)
			kvStore.Put(fmt.Sprintf("%s.board", nodePrefix), string(node.Board))
			kvStore.Put(fmt.Sprintf("%s.cluster", nodePrefix), cluster.Name)

			// Add node to cluster's nodes list
			nodesList := []int{}
			existingNodes, err := store.Get[[]int](kvStore, fmt.Sprintf("%s.nodes", clusterPrefix))
			if err == nil {
				nodesList = existingNodes
			}
			nodesList = append(nodesList, nodeID)
			kvStore.Put(fmt.Sprintf("%s.nodes", clusterPrefix), nodesList)
		}

		// Store cluster index for reference
		kvStore.Put(fmt.Sprintf("turingpi.clusters.%s.index", cluster.Name), i+1)
	}

	// Store a list of all cluster names
	clusterNames := make([]string, len(config.Clusters))
	for i, cluster := range config.Clusters {
		clusterNames[i] = cluster.Name
	}
	kvStore.Put("turingpi.clusters.list", clusterNames)

	return nil
}

// MapClusterToStore maps a single cluster configuration to a KVStore
func MapClusterToStore(cluster *ClusterConfig, clusterIndex int, kvStore *store.KVStore) error {
	if cluster == nil {
		return fmt.Errorf("cluster config cannot be nil")
	}

	clusterPrefix := fmt.Sprintf("turingpi.cluster.%d", clusterIndex)

	// Store cluster details
	kvStore.Put(fmt.Sprintf("%s.name", clusterPrefix), cluster.Name)

	// Store BMC details
	bmcPrefix := fmt.Sprintf("%s.bmc", clusterPrefix)
	kvStore.Put(fmt.Sprintf("%s.ip", bmcPrefix), cluster.BMC.IP)
	kvStore.Put(fmt.Sprintf("%s.user", bmcPrefix), cluster.BMC.Username)
	kvStore.Put(fmt.Sprintf("%s.password", bmcPrefix), cluster.BMC.Password)

	// Process each node
	nodesList := []int{}
	for j, node := range cluster.Nodes {
		// Determine node ID
		nodeID := j + 1
		if node.ID > 0 {
			nodeID = node.ID
		}

		nodePrefix := fmt.Sprintf("turingpi.node.%d", nodeID)

		// Store node details
		kvStore.Put(fmt.Sprintf("%s.name", nodePrefix), node.Name)
		kvStore.Put(fmt.Sprintf("%s.ip", nodePrefix), node.IP)
		kvStore.Put(fmt.Sprintf("%s.board", nodePrefix), string(node.Board))
		kvStore.Put(fmt.Sprintf("%s.cluster", nodePrefix), cluster.Name)

		nodesList = append(nodesList, nodeID)
	}

	// Store nodes list
	kvStore.Put(fmt.Sprintf("%s.nodes", clusterPrefix), nodesList)

	// Store cluster index for reference
	kvStore.Put(fmt.Sprintf("turingpi.clusters.%s.index", cluster.Name), clusterIndex)

	return nil
}
