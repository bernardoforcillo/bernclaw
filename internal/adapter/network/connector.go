// Package network provides federation and remote agent coordination.
// This stub supports local mesh and federation patterns consistent with OpenClaw.
package network

import (
	"context"
	"fmt"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// Connector implements port.NetworkConnector for local mesh and federation.
// For now, this is a stub that handles local node registration.
// Remote invocation and federation will expand this.
type Connector struct {
	localNodeID string
	localNodes  map[string]*domain.NetworkNode
}

// NewConnector creates a network connector for the given local node.
func NewConnector(localNodeID string) port.NetworkConnector {
	return &Connector{
		localNodeID: localNodeID,
		localNodes:  make(map[string]*domain.NetworkNode),
	}
}

// RegisterLocalNode adds this node to the local mesh.
func (c *Connector) RegisterLocalNode(ctx context.Context, node domain.NetworkNode) error {
	if node.ID == "" {
		return fmt.Errorf("node ID is required")
	}
	c.localNodes[node.ID] = &node
	return nil
}

// DiscoverRemoteNodes returns known remote nodes.
// In a real implementation, this would use mDNS, Consul, or similar.
func (c *Connector) DiscoverRemoteNodes(ctx context.Context) ([]domain.NetworkNode, error) {
	var result []domain.NetworkNode
	for _, node := range c.localNodes {
		if node.ID != c.localNodeID {
			result = append(result, *node)
		}
	}
	return result, nil
}

// InvokeRemoteAgent sends a request to invoke an agent on a remote node.
// This is a stub and would require JSON-RPC or gRPC in a real implementation.
func (c *Connector) InvokeRemoteAgent(ctx context.Context, nodeID string, agentName string, input any) (any, error) {
	node, ok := c.localNodes[nodeID]
	if !ok {
		return nil, fmt.Errorf("remote node not found: %s", nodeID)
	}

	// Check if node has the agent
	found := false
	for _, a := range node.Agents {
		if domain.NormalizeName(a) == domain.NormalizeName(agentName) {
			found = true
			break
		}
	}
	if !found {
		return nil, fmt.Errorf("agent %s not found on node %s", agentName, nodeID)
	}

	// TODO: Implement actual RPC call to node.Endpoint
	// For now, return a placeholder
	return map[string]any{"status": "remote invocation not yet implemented"}, nil
}

// ForwardMessage sends a message to an agent on a remote node.
func (c *Connector) ForwardMessage(ctx context.Context, nodeID string, msg domain.AgentMessage) error {
	_, ok := c.localNodes[nodeID]
	if !ok {
		return fmt.Errorf("remote node not found: %s", nodeID)
	}

	// TODO: Implement actual message forwarding
	return nil
}
