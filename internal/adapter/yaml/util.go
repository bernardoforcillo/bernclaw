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
	kindTool      = "Tool"
	kindSoul      = "Soul"
)

// ---- Tool CRD -------------------------------------------------------------- //

type toolResource struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   metadata       `yaml:"metadata"`
	Spec       toolSpecRecord `yaml:"spec"`
}

type toolSpecRecord struct {
	Category     string             `yaml:"category"`
	Description  string             `yaml:"description"`
	Parameters   []toolParam        `yaml:"parameters,omitempty"`
	ReturnType   string             `yaml:"returnType,omitempty"`
	Policies     toolPoliciesRecord `yaml:"policies"`
	TokenCost    int                `yaml:"tokenCost,omitempty"`
	Experimental bool               `yaml:"experimental,omitempty"`
}

type toolParam struct {
	Name        string      `yaml:"name"`
	Type        string      `yaml:"type"`
	Description string      `yaml:"description"`
	Required    bool        `yaml:"required"`
	Default     interface{} `yaml:"default,omitempty"`
}

type toolPoliciesRecord struct {
	Main  toolPolicyRecord `yaml:"main"`
	DM    toolPolicyRecord `yaml:"dm"`
	Group toolPolicyRecord `yaml:"group"`
}

type toolPolicyRecord struct {
	Allowed        bool   `yaml:"allowed"`
	Sandboxed      bool   `yaml:"sandboxed"`
	TimeoutSeconds int    `yaml:"timeoutSeconds"`
	Description    string `yaml:"description,omitempty"`
}

// ---- Soul CRD -------------------------------------------------------------- //

type soulResource struct {
	APIVersion string         `yaml:"apiVersion"`
	Kind       string         `yaml:"kind"`
	Metadata   metadata       `yaml:"metadata"`
	Spec       soulSpecRecord `yaml:"spec"`
}

type soulSpecRecord struct {
	Values           []soulValue `yaml:"values"`
	Personality      []string    `yaml:"personality"`
	DecisionProcess  []string    `yaml:"decisionProcess"`
	InteractionStyle []string    `yaml:"interactionStyle"`
}

type soulValue struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Principles  []string `yaml:"principles"`
}

// ---- Extended Agent CRD ---------------------------------------------------- //
// agentSpecRecord is augmented with role, capabilities, tools and expertise.

type agentRelationshipRecord struct {
	From string `yaml:"from"`
	To   string `yaml:"to"`
	Type string `yaml:"type"`
}

type teamMemberRecord struct {
	Agent string `yaml:"agent"`
	Role  string `yaml:"role"`
}

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
	DisplayName   string                    `yaml:"displayName"`
	Description   string                    `yaml:"description,omitempty"`
	Purpose       string                    `yaml:"purpose,omitempty"`
	Members       []teamMemberRecord        `yaml:"members,omitempty"`
	Relationships []agentRelationshipRecord `yaml:"relationships,omitempty"`
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
	Role         string       `yaml:"role,omitempty"`
	Model        modelRef     `yaml:"model"`
	Connector    connectorRef `yaml:"connector,omitempty"`
	Default      bool         `yaml:"default,omitempty"`
	SystemPrompt string       `yaml:"systemPrompt,omitempty"`
	Capabilities []string     `yaml:"capabilities,omitempty"`
	Tools        []string     `yaml:"tools,omitempty"`
	Expertise    []string     `yaml:"expertise,omitempty"`
	TrustLevel   string       `yaml:"trustLevel,omitempty"`
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
