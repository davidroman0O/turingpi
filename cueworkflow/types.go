// Package cueworkflow handles CUE-based workflow execution
package cueworkflow

// ActionDefinition represents a decoded action from CUE
type ActionDefinition struct {
	Type        string                 `json:"type"`
	Description string                 `json:"description,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"`
}

// StageDefinition represents a decoded stage from CUE
type StageDefinition struct {
	Name        string             `json:"name"`
	Title       string             `json:"title,omitempty"`
	Description string             `json:"description,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	Actions     []ActionDefinition `json:"actions"`
}

// WorkflowDefinition represents a decoded workflow from CUE
type WorkflowDefinition struct {
	Name        string                 `json:"name"`
	Title       string                 `json:"title,omitempty"`
	Description string                 `json:"description,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"` // Decoded input params schema
	Stages      []StageDefinition      `json:"stages"`
}

// BMCConfig represents BMC configuration from CUE
type BMCConfig struct {
	IP       string `json:"ip"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// CacheConfig represents cache configuration from CUE
type CacheConfig struct {
	LocalCachePath string `json:"localCachePath,omitempty"`
	TempCachePath  string `json:"tempCachePath,omitempty"`
}

// ClusterConfig represents the complete cluster configuration from CUE
type ClusterConfig struct {
	BMC   BMCConfig   `json:"bmc"`
	Cache CacheConfig `json:"cache,omitempty"`
}
