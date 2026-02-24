package yaml

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
)

// AgentRepo is a file-system-backed implementation of port.AgentRepository.
// Each team and agent is stored as a separate YAML file inside BaseDir.
type AgentRepo struct {
	baseDir string
}

// NewAgentRepo creates an AgentRepo rooted at baseDir.
func NewAgentRepo(baseDir string) AgentRepo {
	return AgentRepo{baseDir: strings.TrimSpace(baseDir)}
}

func (r AgentRepo) ensureDir() error {
	if strings.TrimSpace(r.baseDir) == "" {
		return fmt.Errorf("agent repository base directory is empty")
	}
	return os.MkdirAll(r.baseDir, 0o755)
}

// ---- team operations ------------------------------------------------------- //

// SaveTeam persists a team resource file.
func (r AgentRepo) SaveTeam(name string) error {
	clean := strings.TrimSpace(name)
	if clean == "" {
		return fmt.Errorf("team name is required")
	}
	if err := r.ensureDir(); err != nil {
		return err
	}

	resource := teamResource{
		APIVersion: apiVersion,
		Kind:       kindTeam,
		Metadata:   metadata{Name: domain.NormalizeName(clean)},
		Spec:       teamSpecBody{DisplayName: clean},
	}
	return writeYAML(r.teamPath(clean), resource)
}

// DeleteTeam removes the team resource file.
func (r AgentRepo) DeleteTeam(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("team name is required")
	}
	if err := r.ensureDir(); err != nil {
		return err
	}
	if err := os.Remove(r.teamPath(name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ListTeams returns all persisted teams sorted alphabetically.
func (r AgentRepo) ListTeams() ([]domain.StoredTeam, error) {
	if err := r.ensureDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		return nil, err
	}

	items := make([]domain.StoredTeam, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "team-") || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		var res teamResource
		if err := readYAML(filepath.Join(r.baseDir, entry.Name()), &res); err != nil {
			continue
		}
		if res.Kind != kindTeam || strings.TrimSpace(res.Spec.DisplayName) == "" {
			continue
		}
		items = append(items, domain.StoredTeam{Name: res.Spec.DisplayName})
	}

	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items, nil
}

// ---- agent operations ------------------------------------------------------ //

// SaveAgent persists an agent resource file.
func (r AgentRepo) SaveAgent(spec domain.Spec, teamName string) error {
	cleanName := strings.TrimSpace(spec.Name)
	if cleanName == "" {
		return fmt.Errorf("agent name is required")
	}
	if err := r.ensureDir(); err != nil {
		return err
	}

	resource := agentResource{
		APIVersion: apiVersion,
		Kind:       kindAgent,
		Metadata: metadata{
			Name:   domain.NormalizeName(cleanName),
			Labels: map[string]string{},
		},
		Spec: agentSpecRecord{
			Team:         strings.TrimSpace(teamName),
			DisplayName:  cleanName,
			Model:        modelRef{Name: strings.TrimSpace(spec.ModelName)},
			Connector:    connectorRef{Name: strings.TrimSpace(spec.Connector)},
			Default:      spec.IsDefault,
			SystemPrompt: spec.SystemPrompt,
		},
	}
	if resource.Spec.Team != "" {
		resource.Metadata.Labels["team"] = domain.NormalizeName(resource.Spec.Team)
	}
	if len(resource.Metadata.Labels) == 0 {
		resource.Metadata.Labels = nil
	}

	return writeYAML(r.agentPath(cleanName, teamName), resource)
}

// DeleteAgent removes an agent resource file.
func (r AgentRepo) DeleteAgent(name string, teamName string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("agent name is required")
	}
	if err := r.ensureDir(); err != nil {
		return err
	}
	if err := os.Remove(r.agentPath(name, teamName)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ListAgents returns all persisted agents sorted alphabetically.
func (r AgentRepo) ListAgents() ([]domain.StoredAgent, error) {
	if err := r.ensureDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		return nil, err
	}

	items := make([]domain.StoredAgent, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") || strings.HasPrefix(entry.Name(), "team-") {
			continue
		}
		currentPath := filepath.Join(r.baseDir, entry.Name())

		var res agentResource
		if err := readYAML(currentPath, &res); err != nil {
			continue
		}
		if res.Kind != kindAgent || strings.TrimSpace(res.Spec.DisplayName) == "" {
			continue
		}

		// Migrate files to canonical path on-the-fly.
		targetPath := r.agentPath(res.Spec.DisplayName, res.Spec.Team)
		if currentPath != targetPath {
			if _, statErr := os.Stat(targetPath); os.IsNotExist(statErr) {
				_ = writeYAML(targetPath, res)
			}
			_ = os.Remove(currentPath)
		}

		items = append(items, domain.StoredAgent{
			Team: strings.TrimSpace(res.Spec.Team),
			Spec: domain.Spec{
				Name:         res.Spec.DisplayName,
				ModelName:    strings.TrimSpace(res.Spec.Model.Name),
				Connector:    strings.TrimSpace(res.Spec.Connector.Name),
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

// ---- path helpers ---------------------------------------------------------- //

func (r AgentRepo) teamPath(name string) string {
	return filepath.Join(r.baseDir, fmt.Sprintf("team-%s.yaml", domain.NormalizeName(name)))
}

func (r AgentRepo) agentPath(name string, _ string) string {
	return filepath.Join(r.baseDir, fmt.Sprintf("%s.yaml", domain.NormalizeName(name)))
}
