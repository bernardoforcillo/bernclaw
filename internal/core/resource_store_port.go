package core

import "github.com/bernardoforcillo/bernclaw/internal/agent"

type agentResourceStore interface {
	ListTeams() ([]agent.StoredTeam, error)
	ListAgents() ([]agent.StoredAgent, error)
	SaveTeam(name string) error
	DeleteTeam(name string) error
	SaveAgent(spec agent.Spec, teamName string) error
	DeleteAgent(name string, teamName string) error
}
