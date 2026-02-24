// Package app contains application-level services that orchestrate
// domain logic and coordinate between ports (repositories, LLM clients).
// It sits between the inbound adapters (TUI) and the outbound adapters
// (YAML persistence, LLM HTTP clients).
package app

import (
	"path/filepath"

	"github.com/bernardoforcillo/bernclaw/internal/adapter/graph"
	yamlrepo "github.com/bernardoforcillo/bernclaw/internal/adapter/yaml"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// Workspace holds the repositories and services that together represent the on-disk
// state of a bernclaw workspace. Agents, connectors, and team coordination all live
// in the workspace, enabling multi-agent orchestration.
type Workspace struct {
	Agents       port.AgentRepository
	Connectors   port.ConnectorRepository
	Teams        port.GraphStore       // Stores team coordination, roles, relationships
	Orchestrator port.TeamOrchestrator // Routes tasks to agents based on role/expertise
}

// NewWorkspace creates a Workspace backed by YAML files for agents/connectors
// and in-memory graph store for team coordination.
// agentDir and connectorDir are the paths (relative or absolute) to the
// directories that contain agent/team and connector resource files respectively.
func NewWorkspace(agentDir string, connectorDir string) Workspace {
	graphStore := defaultGraphStore()
	teamService := &TeamService{graphStore: graphStore}

	return Workspace{
		Agents:       yamlrepo.NewAgentRepo(agentDir),
		Connectors:   yamlrepo.NewConnectorRepo(connectorDir),
		Teams:        graphStore,
		Orchestrator: teamService,
	}
}

// defaultGraphStore returns an in-memory graph store for team coordination.
func defaultGraphStore() port.GraphStore {
	return graph.NewStore()
}

// DefaultWorkspace returns a Workspace rooted at the conventional
// .bernclaw/ sub-directories relative to the current working directory.
func DefaultWorkspace() Workspace {
	return NewWorkspace(
		filepath.Join(".bernclaw", "agents"),
		filepath.Join(".bernclaw", "connectors"),
	)
}
