// Package yaml provides YAML-file-backed implementations of the AgentRepository
// and ConnectorRepository ports.  Each resource is stored as a separate .yaml
// file inside a configurable base directory.
package yaml

import (
	"os"

	yamlenc "go.yaml.in/yaml/v3"
)

const (
	apiVersion    = "bernclaw.dev/v1alpha1"
	kindAgent     = "Agent"
	kindTeam      = "Team"
	kindConnector = "Connector"
)

// ---- shared YAML envelope types ------------------------------------------ //

type metadata struct {
	Name   string            `yaml:"name"`
	Labels map[string]string `yaml:"labels,omitempty"`
}

type teamResource struct {
	APIVersion string       `yaml:"apiVersion"`
	Kind       string       `yaml:"kind"`
	Metadata   metadata     `yaml:"metadata"`
	Spec       teamSpecBody `yaml:"spec"`
}

type teamSpecBody struct {
	DisplayName string `yaml:"displayName"`
}

type agentResource struct {
	APIVersion string          `yaml:"apiVersion"`
	Kind       string          `yaml:"kind"`
	Metadata   metadata        `yaml:"metadata"`
	Spec       agentSpecRecord `yaml:"spec"`
}

type agentSpecRecord struct {
	Team         string       `yaml:"team,omitempty"`
	DisplayName  string       `yaml:"displayName"`
	Model        modelRef     `yaml:"model"`
	Connector    connectorRef `yaml:"connector,omitempty"`
	Default      bool         `yaml:"default,omitempty"`
	SystemPrompt string       `yaml:"systemPrompt,omitempty"`
}

type modelRef struct {
	Name string `yaml:"name"`
}

type connectorRef struct {
	Name string `yaml:"name"`
}

type connectorResource struct {
	APIVersion string              `yaml:"apiVersion"`
	Kind       string              `yaml:"kind"`
	Metadata   metadata            `yaml:"metadata"`
	Spec       connectorSpecRecord `yaml:"spec"`
}

type connectorSpecRecord struct {
	DisplayName string `yaml:"displayName"`
	Provider    string `yaml:"provider"`
	APIKey      string `yaml:"apiKey,omitempty"`
	BaseURL     string `yaml:"baseURL,omitempty"`
}

// ---- YAML I/O helpers ------------------------------------------------------ //

func writeYAML(path string, value any) error {
	content, err := yamlenc.Marshal(value)
	if err != nil {
		return err
	}
	return os.WriteFile(path, content, 0o644)
}

func readYAML(path string, target any) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yamlenc.Unmarshal(content, target)
}
