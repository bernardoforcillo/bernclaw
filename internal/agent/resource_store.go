package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

const (
	apiVersion = "bernclaw.dev/v1alpha1"
	kindAgent  = "Agent"
	kindTeam   = "Team"
)

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
	Team         string   `yaml:"team,omitempty"`
	DisplayName  string   `yaml:"displayName"`
	Model        modelRef `yaml:"model"`
	Default      bool     `yaml:"default,omitempty"`
	SystemPrompt string   `yaml:"systemPrompt,omitempty"`
}

type modelRef struct {
	Name string `yaml:"name"`
}

type StoredTeam struct {
	Name string
}

type StoredAgent struct {
	Team string
	Spec Spec
}

type ResourceStore struct {
	baseDir string
}

func NewResourceStore(baseDir string) ResourceStore {
	return ResourceStore{baseDir: strings.TrimSpace(baseDir)}
}

func (store ResourceStore) EnsureDir() error {
	if strings.TrimSpace(store.baseDir) == "" {
		return fmt.Errorf("resource base directory is empty")
	}
	return os.MkdirAll(store.baseDir, 0o755)
}

func (store ResourceStore) SaveTeam(name string) error {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return fmt.Errorf("team name is required")
	}
	if err := store.EnsureDir(); err != nil {
		return err
	}

	resource := teamResource{
		APIVersion: apiVersion,
		Kind:       kindTeam,
		Metadata: metadata{
			Name: normalizeName(clean),
		},
		Spec: teamSpecBody{DisplayName: clean},
	}

	return writeYAML(store.teamPath(clean), resource)
}

func (store ResourceStore) DeleteTeam(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("team name is required")
	}
	if err := store.EnsureDir(); err != nil {
		return err
	}
	if err := os.Remove(store.teamPath(name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (store ResourceStore) ListTeams() ([]StoredTeam, error) {
	if err := store.EnsureDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(store.baseDir)
	if err != nil {
		return nil, err
	}

	items := make([]StoredTeam, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "team-") || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		var res teamResource
		if err := readYAML(filepath.Join(store.baseDir, entry.Name()), &res); err != nil {
			continue
		}
		if res.Kind != kindTeam || strings.TrimSpace(res.Spec.DisplayName) == "" {
			continue
		}
		items = append(items, StoredTeam{Name: res.Spec.DisplayName})
	}

	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items, nil
}

func (store ResourceStore) SaveAgent(spec Spec, teamName string) error {
	cleanName := strings.TrimSpace(spec.Name)
	if cleanName == "" {
		return fmt.Errorf("agent name is required")
	}
	if err := store.EnsureDir(); err != nil {
		return err
	}

	resource := agentResource{
		APIVersion: apiVersion,
		Kind:       kindAgent,
		Metadata: metadata{
			Name:   normalizeName(cleanName),
			Labels: map[string]string{},
		},
		Spec: agentSpecRecord{
			Team:         strings.TrimSpace(teamName),
			DisplayName:  cleanName,
			Model:        modelRef{Name: strings.TrimSpace(spec.ModelName)},
			Default:      spec.IsDefault,
			SystemPrompt: spec.SystemPrompt,
		},
	}
	if resource.Spec.Team != "" {
		resource.Metadata.Labels["team"] = normalizeName(resource.Spec.Team)
	}
	if len(resource.Metadata.Labels) == 0 {
		resource.Metadata.Labels = nil
	}

	return writeYAML(store.agentPath(cleanName, teamName), resource)
}

func (store ResourceStore) DeleteAgent(name string, teamName string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("agent name is required")
	}
	if err := store.EnsureDir(); err != nil {
		return err
	}
	if err := os.Remove(store.agentPath(name, teamName)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (store ResourceStore) ListAgents() ([]StoredAgent, error) {
	if err := store.EnsureDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(store.baseDir)
	if err != nil {
		return nil, err
	}

	items := make([]StoredAgent, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") || strings.HasPrefix(entry.Name(), "team-") {
			continue
		}
		currentPath := filepath.Join(store.baseDir, entry.Name())

		var res agentResource
		if err := readYAML(currentPath, &res); err != nil {
			continue
		}
		if res.Kind != kindAgent || strings.TrimSpace(res.Spec.DisplayName) == "" {
			continue
		}

		targetPath := store.agentPath(res.Spec.DisplayName, res.Spec.Team)
		if currentPath != targetPath {
			if _, statErr := os.Stat(targetPath); os.IsNotExist(statErr) {
				_ = writeYAML(targetPath, res)
			}
			_ = os.Remove(currentPath)
		}

		items = append(items, StoredAgent{
			Team: strings.TrimSpace(res.Spec.Team),
			Spec: Spec{
				Name:         res.Spec.DisplayName,
				ModelName:    strings.TrimSpace(res.Spec.Model.Name),
				IsDefault:    res.Spec.Default,
				SystemPrompt: res.Spec.SystemPrompt,
			},
		})
	}

	sort.Slice(items, func(i, j int) bool {
		left := strings.ToLower(items[i].Spec.Name)
		right := strings.ToLower(items[j].Spec.Name)
		if left == right {
			return strings.ToLower(items[i].Team) < strings.ToLower(items[j].Team)
		}
		return left < right
	})
	return items, nil
}

func (store ResourceStore) teamPath(name string) string {
	return filepath.Join(store.baseDir, fmt.Sprintf("team-%s.yaml", normalizeName(name)))
}

func (store ResourceStore) agentPath(name string, teamName string) string {
	_ = teamName
	return filepath.Join(store.baseDir, fmt.Sprintf("%s.yaml", normalizeName(name)))
}

func writeYAML(path string, value any) error {
	content, err := yaml.Marshal(value)
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
	return yaml.Unmarshal(content, target)
}
