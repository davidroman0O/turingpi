package tools

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/davidroman0O/turingpi/pkg/v2/node"
)

// NodeToolImpl is the implementation of the NodeTool interface
type NodeToolImpl struct {
	bmcTool     BMCTool
	nodeConfigs map[int]*NodeConfig
	nodes       map[int]node.Node
	mu          sync.RWMutex
}

// NewNodeTool creates a new NodeTool
func NewNodeTool(bmcTool BMCTool, nodeConfigs map[int]*NodeConfig) NodeTool {
	return &NodeToolImpl{
		bmcTool:     bmcTool,
		nodeConfigs: nodeConfigs,
		nodes:       make(map[int]node.Node),
	}
}

// getNode gets or creates a node instance
func (t *NodeToolImpl) getNode(nodeID int) (node.Node, error) {
	t.mu.RLock()
	n, exists := t.nodes[nodeID]
	t.mu.RUnlock()

	if exists {
		return n, nil
	}

	// Create new node
	t.mu.Lock()
	defer t.mu.Unlock()

	// Double-check after acquiring lock
	if n, exists := t.nodes[nodeID]; exists {
		return n, nil
	}

	// Get node config
	config, exists := t.nodeConfigs[nodeID]
	if !exists {
		return nil, fmt.Errorf("no configuration found for node %d", nodeID)
	}

	// Create SSH config
	sshConfig := &node.SSHConfig{
		Host:           config.Host,
		User:           config.User,
		Password:       config.Password,
		Timeout:        10 * time.Second,
		MaxRetries:     3,
		RetryDelay:     1 * time.Second,
		RetryIncrement: 1 * time.Second,
	}

	// Create node instance
	n = node.NewNode(nodeID, sshConfig, nil) // No BMC client for SSH operations

	t.nodes[nodeID] = n
	return n, nil
}

// ExecuteCommand runs a non-interactive command on the target node via SSH
func (t *NodeToolImpl) ExecuteCommand(ctx context.Context, nodeID int, command string) (stdout string, stderr string, err error) {
	n, err := t.getNode(nodeID)
	if err != nil {
		return "", "", err
	}
	return n.ExecuteCommand(ctx, command)
}

// ExpectAndSend performs a sequence of expect/send interactions over an SSH session
func (t *NodeToolImpl) ExpectAndSend(ctx context.Context, nodeID int, steps []node.InteractionStep, timeout time.Duration) (string, error) {
	n, err := t.getNode(nodeID)
	if err != nil {
		return "", err
	}
	return n.ExpectAndSend(ctx, steps, timeout)
}

// CopyFile copies a file to or from the node
func (t *NodeToolImpl) CopyFile(ctx context.Context, nodeID int, localPath, remotePath string, toNode bool) error {
	n, err := t.getNode(nodeID)
	if err != nil {
		return err
	}
	return n.CopyFile(ctx, localPath, remotePath, toNode)
}

// GetInfo retrieves detailed information about the node
func (t *NodeToolImpl) GetInfo(ctx context.Context, nodeID int) (*node.NodeInfo, error) {
	n, err := t.getNode(nodeID)
	if err != nil {
		return nil, err
	}
	return n.GetInfo(ctx)
}
