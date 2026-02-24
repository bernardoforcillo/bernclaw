package port

import "github.com/bernardoforcillo/bernclaw/internal/domain"

// AgentRepository defines persistence operations for teams and agents.
type AgentRepository interface {
	SaveTeam(name string) error
	DeleteTeam(name string) error
	ListTeams() ([]domain.StoredTeam, error)

	SaveAgent(spec domain.Spec, teamName string) error
	DeleteAgent(name string, teamName string) error
	ListAgents() ([]domain.StoredAgent, error)
}

// ConnectorRepository defines persistence operations for connectors.
type ConnectorRepository interface {
	SaveConnector(connector domain.Connector) error
	DeleteConnector(name string) error
	ListConnectors() ([]domain.Connector, error)
	GetConnector(name string) (domain.Connector, error)
}
