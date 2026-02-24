// Package app contains application-level services that orchestrate
// domain logic and coordinate between ports (repositories, LLM clients).
// It sits between the inbound adapters (TUI) and the outbound adapters
// (YAML persistence, LLM HTTP clients).
package app

import (
	"log"
	"path/filepath"

	"github.com/bernardoforcillo/bernclaw/internal/adapter/graph"
	"github.com/bernardoforcillo/bernclaw/internal/adapter/system"
	fs "github.com/bernardoforcillo/bernclaw/internal/adapter/workspace"
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
	CRDs         *yamlrepo.CRDRegistry // Loaded view of all .bernclaw/ CRD resources
	System       port.SystemService    // Access to system processes
	Files        port.FileService      // Access to workspace files
	Tools        *ToolExecutor         // Executes tools against the workspace
}

// NewWorkspace creates a Workspace backed by YAML files for agents/connectors
// and in-memory graph store for team coordination.
// agentDir and connectorDir are the paths (relative or absolute) to the
// directories that contain agent/team and connector resource files respectively.
func NewWorkspace(agentDir string, connectorDir string) Workspace {
	graphStore := defaultGraphStore()
	teamService := &TeamService{graphStore: graphStore}

	sys := system.NewSystemService()
	files := fs.NewFileSystemWorkspace(".")

	return Workspace{
		Agents:       yamlrepo.NewAgentRepo(agentDir),
		Connectors:   yamlrepo.NewConnectorRepo(connectorDir),
		Teams:        graphStore,
		Orchestrator: teamService,
		System:       sys,
		Files:        files,
		Tools:        NewToolExecutor(files, sys),
	}
}

// defaultGraphStore returns an in-memory graph store for team coordination.
func defaultGraphStore() port.GraphStore {
	return graph.NewStore()
}

// DefaultWorkspace returns a Workspace rooted at the conventional
// .bernclaw/ sub-directories relative to the current working directory.
// All CRD resources are loaded from .bernclaw/ on startup.
func DefaultWorkspace() Workspace {
	ws := NewWorkspace(
		filepath.Join(".bernclaw", "agents"),
		filepath.Join(".bernclaw", "connectors"),
	)

	// Load all CRD resources (agents, teams, tools, souls) from .bernclaw/
	registry := yamlrepo.NewCRDRegistry(".bernclaw")
	if err := registry.Load(); err != nil {
		log.Printf("[workspace] CRD registry load warning: %v", err)
	} else {
		log.Printf("[workspace] CRD registry loaded: %d agents, %d teams, %d tools, %d souls",
			len(registry.Agents), len(registry.Teams), len(registry.Tools), len(registry.Souls))
	}
	ws.CRDs = registry

	return ws
}
