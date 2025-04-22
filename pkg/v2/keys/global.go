// Package keys provides standardized key constants for workflow data stores
package keys

import (
	"fmt"
)

// Global constants for store keys with consistent formatting
const (
	// Node-specific keys (parameterized with node ID)
	NodePower       = "turingpi.node.%d.power"       // State: on/off
	NodeBootMode    = "turingpi.node.%d.boot.mode"   // Boot mode: usb/network/etc
	NodeUSBMode     = "turingpi.node.%d.usb.mode"    // USB mode: device/host
	NodeConsole     = "turingpi.node.%d.console"     // Console connection object
	NodeStatus      = "turingpi.node.%d.status"      // Full status object
	NodeDiagnostics = "turingpi.node.%d.diagnostics" // Diagnostic results
	NodeIP          = "turingpi.node.%d.ip"          // Node IP address

	// BMC-specific keys
	BMCInfo     = "turingpi.bmc.info"     // BMC info object
	BMCFirmware = "turingpi.bmc.firmware" // BMC firmware info
	BMCHealth   = "turingpi.bmc.health"   // BMC health status

	// Cluster-wide keys
	ClusterNodes  = "turingpi.cluster.nodes"  // List of active nodes
	ClusterHealth = "turingpi.cluster.health" // Overall cluster health

	// Container-related keys (parameterized with container ID)
	ContainerState = "turingpi.container.%s.state" // Container state
	ContainerList  = "turingpi.containers.list"    // List of all containers

	// Image-related keys
	ImageSource = "turingpi.image.source" // Source image path
	ImageTarget = "turingpi.image.target" // Target device/path
	ImageMounts = "turingpi.image.mounts" // Map of mounted partitions

	// Workflow control keys
	CurrentNodeID = "turingpi.workflow.current_node" // Currently targeted node ID
	TargetNodes   = "turingpi.workflow.target_nodes" // List of nodes to operate on
	WorkflowState = "turingpi.workflow.state"        // Overall workflow state

	// Tool access keys
	ToolsProvider = "turingpi.tools"       // Main tool provider
	CacheTool     = "turingpi.tools.cache" // Cache tool for content caching
	FSTool        = "turingpi.tools.fs"    // Filesystem operations tool
)

// FormatKey formats a key with its parameters
func FormatKey(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}

// NodeKey formats a node-specific key
func NodeKey(format string, nodeID int) string {
	return fmt.Sprintf(format, nodeID)
}
