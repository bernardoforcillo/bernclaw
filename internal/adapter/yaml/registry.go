package yaml

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// CRDRegistry loads all bernclaw CRD resources from a root directory.
// It walks .bernclaw/ (or any provided root) and dispatches each YAML file
// by its `kind` field into the corresponding typed collection.
type CRDRegistry struct {
	rootDir string

	Agents []agentResource
	Teams  []teamResource
	Tools  []toolResource
	Souls  []soulResource
}

// NewCRDRegistry creates a registry rooted at dir (e.g. ".bernclaw").
func NewCRDRegistry(dir string) *CRDRegistry {
	return &CRDRegistry{rootDir: strings.TrimRight(dir, "/\\")}
}

// Load walks all *.yaml files under rootDir and classifies them by kind.
func (r *CRDRegistry) Load() error {
	if _, err := os.Stat(r.rootDir); os.IsNotExist(err) {
		return fmt.Errorf("bernclaw directory not found: %s", r.rootDir)
	}

	return filepath.WalkDir(r.rootDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".yaml") {
			return nil
		}

		// Peek at the kind field without fully deserialising.
		var envelope struct {
			Kind string `yaml:"kind"`
		}
		if readErr := readYAML(path, &envelope); readErr != nil {
			// Skip unreadable files silently.
			return nil
		}

		switch envelope.Kind {
		case kindAgent:
			var res agentResource
			if readErr := readYAML(path, &res); readErr == nil {
				r.Agents = append(r.Agents, res)
			}
		case kindTeam:
			var res teamResource
			if readErr := readYAML(path, &res); readErr == nil {
				r.Teams = append(r.Teams, res)
			}
		case kindTool:
			var res toolResource
			if readErr := readYAML(path, &res); readErr == nil {
				r.Tools = append(r.Tools, res)
			}
		case kindSoul:
			var res soulResource
			if readErr := readYAML(path, &res); readErr == nil {
				r.Souls = append(r.Souls, res)
			}
		}
		return nil
	})
}

// AgentDir returns the canonical subdirectory for Agent resources.
func (r *CRDRegistry) AgentDir() string {
	return filepath.Join(r.rootDir, "agents")
}

// TeamDir returns the canonical subdirectory for Team resources.
func (r *CRDRegistry) TeamDir() string {
	return filepath.Join(r.rootDir, "teams")
}

// ToolDir returns the canonical subdirectory for Tool resources.
func (r *CRDRegistry) ToolDir() string {
	return filepath.Join(r.rootDir, "tools")
}

// SaveTool writes a Tool CRD file into the tools subdirectory.
func (r *CRDRegistry) SaveTool(res toolResource) error {
	if err := os.MkdirAll(r.ToolDir(), 0o755); err != nil {
		return err
	}
	name := strings.ToLower(strings.ReplaceAll(res.Metadata.Name, " ", "-"))
	return writeYAML(filepath.Join(r.ToolDir(), name+".yaml"), res)
}

// SaveSoul writes the Soul CRD at the root of the registry directory.
func (r *CRDRegistry) SaveSoul(res soulResource) error {
	if err := os.MkdirAll(r.rootDir, 0o755); err != nil {
		return err
	}
	return writeYAML(filepath.Join(r.rootDir, "soul.yaml"), res)
}
