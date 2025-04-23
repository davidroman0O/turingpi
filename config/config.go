// Package config provides a configuration management system using a type-safe store
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/davidroman0O/gostage/store"
)

// NodeID represents a compute node identifier
type NodeID int

// BoardType identifies the type of compute module
type BoardType string

// Predefined board types
const (
	RK1 BoardType = "rk1"
	CM4 BoardType = "cm4"
)

// Node ID constants
const (
	Node1 NodeID = 1
	Node2 NodeID = 2
	Node3 NodeID = 3
	Node4 NodeID = 4

	// Special node ID for image preparation
	NodePrepareID NodeID = 0
)

// NodeConfig holds the configuration for a single node
type NodeConfig struct {
	IP string `yaml:"ip"` // Static IP for the node
}

// TPIConfig holds the global configuration for the Turing Pi toolkit
type TPIConfig struct {
	// Cache directories
	CacheDir      string `yaml:"cacheDir,omitempty"`      // Base directory for all caching
	PrepImageDir  string `yaml:"prepImageDir,omitempty"`  // Directory for image preparation
	StateFileName string `yaml:"stateFileName,omitempty"` // Name of the state file

	// BMC configuration
	BMCIP       string `yaml:"bmcIP"`       // IP address of the BMC
	BMCUser     string `yaml:"bmcUser"`     // Username for BMC access
	BMCPassword string `yaml:"bmcPassword"` // Password for BMC access

	// Node configurations
	Node1 *NodeConfig `yaml:"node1,omitempty"`
	Node2 *NodeConfig `yaml:"node2,omitempty"`
	Node3 *NodeConfig `yaml:"node3,omitempty"`
	Node4 *NodeConfig `yaml:"node4,omitempty"`
}

// Config provides a configuration management system
type Config struct {
	store     *store.KVStore
	filePath  string
	autoSave  bool
	namespace string
}

// Option defines a configuration option
type Option func(*Config)

// WithAutoSave enables automatic saving of config changes to disk
func WithAutoSave() Option {
	return func(c *Config) {
		c.autoSave = true
	}
}

// WithNamespace sets a namespace prefix for all keys
func WithNamespace(ns string) Option {
	return func(c *Config) {
		c.namespace = ns
	}
}

// New creates a new configuration manager
func New(opts ...Option) (*Config, error) {
	cfg := &Config{
		store: store.NewKVStore(),
	}

	// Apply options
	for _, opt := range opts {
		opt(cfg)
	}

	// Create directory if it doesn't exist
	if cfg.filePath != "" {
		dir := filepath.Dir(cfg.filePath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create config directory: %w", err)
		}

		// Try to load existing config
		if err := cfg.Load(); err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load config: %w", err)
		}
	}

	return cfg, nil
}

// prefixKey adds namespace prefix to key if namespace is set
func (c *Config) prefixKey(key string) string {
	if c.namespace != "" {
		return c.namespace + "." + key
	}
	return key
}

// Get retrieves a typed configuration value
func Get[T any](c *Config, key string) (T, error) {
	return store.Get[T](c.store, c.prefixKey(key))
}

// GetOrDefault retrieves a configuration value with a default
func GetOrDefault[T any](c *Config, key string, defaultValue T) (T, error) {
	return store.GetOrDefault[T](c.store, c.prefixKey(key), defaultValue)
}

// Set stores a configuration value
func (c *Config) Set(key string, value interface{}) error {
	if err := c.store.Put(c.prefixKey(key), value); err != nil {
		return err
	}

	if c.autoSave {
		return c.Save()
	}
	return nil
}

// SetWithTTL stores a configuration value with expiration
func (c *Config) SetWithTTL(key string, value interface{}, ttl time.Duration) error {
	if err := c.store.PutWithTTL(c.prefixKey(key), value, ttl); err != nil {
		return err
	}

	if c.autoSave {
		return c.Save()
	}
	return nil
}

// Delete removes a configuration value
func (c *Config) Delete(key string) bool {
	deleted := c.store.Delete(c.prefixKey(key))
	if deleted && c.autoSave {
		_ = c.Save() // Best effort save
	}
	return deleted
}

// Clear removes all configuration values
func (c *Config) Clear() {
	c.store.Clear()
	if c.autoSave {
		_ = c.Save() // Best effort save
	}
}

// ListKeys returns all configuration keys
func (c *Config) ListKeys() []string {
	return c.store.ListKeys()
}

// Save persists the configuration to disk
func (c *Config) Save() error {
	if c.filePath == "" {
		return nil // No persistence requested
	}

	// Create a map of all current values
	values := make(map[string]interface{})
	for _, key := range c.store.ListKeys() {
		schema, err := c.store.GetTypeSchema(key)
		if err != nil {
			continue // Skip problematic entries
		}
		values[key] = schema
	}

	data, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(c.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Load reads the configuration from disk
func (c *Config) Load() error {
	if c.filePath == "" {
		return nil // No persistence requested
	}

	data, err := os.ReadFile(c.filePath)
	if err != nil {
		return err
	}

	var values map[string]interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Clear existing config
	c.store.Clear()

	// Load values
	for key, value := range values {
		if err := c.store.Put(key, value); err != nil {
			return fmt.Errorf("failed to load key %s: %w", key, err)
		}
	}

	return nil
}

// UpdateField updates a single field in a configuration object
func (c *Config) UpdateField(key string, fieldPath string, fieldValue interface{}) error {
	if err := c.store.UpdateField(c.prefixKey(key), fieldPath, fieldValue); err != nil {
		return err
	}

	if c.autoSave {
		return c.Save()
	}
	return nil
}

// UpdateFields updates multiple fields in a configuration object
func (c *Config) UpdateFields(key string, fields map[string]interface{}) error {
	if err := c.store.UpdateFields(c.prefixKey(key), fields); err != nil {
		return err
	}

	if c.autoSave {
		return c.Save()
	}
	return nil
}

// GetSchema returns the JSON schema for a configuration value
func (c *Config) GetSchema(key string) (interface{}, error) {
	return c.store.GetTypeSchema(c.prefixKey(key))
}

// FindBySchema returns all keys whose type matches the given schema pattern
func (c *Config) FindBySchema(pattern interface{}) []string {
	return c.store.FindKeysBySchema(pattern)
}
