// Package config provides configuration structures and loading utilities
package config

// ClusterConfig represents a cluster configuration from a config file
type ClusterConfig struct {
	Name  string              `yaml:"name" json:"name"`
	BMC   BMCConfig           `yaml:"bmc" json:"bmc"`
	Nodes []ClusterNodeConfig `yaml:"nodes" json:"nodes"`
	// Optional cluster-specific cache settings
	Cache *CacheConfig `yaml:"cache,omitempty" json:"cache,omitempty"`
}

// BMCConfig contains BMC connection details
type BMCConfig struct {
	IP       string `yaml:"ip" json:"ip"`
	Username string `yaml:"username" json:"username"`
	Password string `yaml:"password" json:"password"`
}

// ClusterNodeConfig contains node-specific configuration in a cluster
type ClusterNodeConfig struct {
	Name  string    `yaml:"name" json:"name"`
	IP    string    `yaml:"ip" json:"ip"`
	Board BoardType `yaml:"board" json:"board"`
	ID    int       `yaml:"id" json:"id"` // Optional explicit ID
	// SSH configuration for remote operations on this node
	SSH *SSHConfig `yaml:"ssh,omitempty" json:"ssh,omitempty"`
}

// SSHConfig contains SSH connection details for a node
type SSHConfig struct {
	User     string `yaml:"user" json:"user"`
	Password string `yaml:"password,omitempty" json:"password,omitempty"`
	KeyFile  string `yaml:"keyFile,omitempty" json:"keyFile,omitempty"`
	Port     int    `yaml:"port" json:"port"`
}

// CacheConfig contains caching configuration
type CacheConfig struct {
	LocalDir  string `yaml:"localDir" json:"localDir"`
	RemoteDir string `yaml:"remoteDir,omitempty" json:"remoteDir,omitempty"`
}

// ConfigFile represents the top-level configuration file structure
type ConfigFile struct {
	Clusters []ClusterConfig `yaml:"clusters" json:"clusters"`
	// Global settings
	Global GlobalConfig `yaml:"global,omitempty" json:"global,omitempty"`
}

// GlobalConfig contains global configuration settings
type GlobalConfig struct {
	// Default cache configuration
	Cache CacheConfig `yaml:"cache,omitempty" json:"cache,omitempty"`
	// Skip Docker operations if true
	SkipDocker bool `yaml:"skipDocker,omitempty" json:"skipDocker,omitempty"`
	// Default SSH configuration
	DefaultSSH *SSHConfig `yaml:"defaultSSH,omitempty" json:"defaultSSH,omitempty"`
}
