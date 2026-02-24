package yaml

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
)

// ConnectorRepo is a file-system-backed implementation of port.ConnectorRepository.
// Each connector is stored as connector-<name>.yaml inside BaseDir.
type ConnectorRepo struct {
	baseDir string
}

// NewConnectorRepo creates a ConnectorRepo rooted at baseDir.
func NewConnectorRepo(baseDir string) ConnectorRepo {
	return ConnectorRepo{baseDir: strings.TrimSpace(baseDir)}
}

func (r ConnectorRepo) ensureDir() error {
	if strings.TrimSpace(r.baseDir) == "" {
		return fmt.Errorf("connector repository base directory is empty")
	}
	return os.MkdirAll(r.baseDir, 0o755)
}

// SaveConnector validates and persists a connector resource file.
func (r ConnectorRepo) SaveConnector(connector domain.Connector) error {
	cleanName := strings.TrimSpace(connector.Name)
	if cleanName == "" {
		return fmt.Errorf("connector name is required")
	}

	provider := strings.TrimSpace(connector.Provider)
	if provider == "" {
		return fmt.Errorf("connector provider is required")
	}
	if !domain.IsSupportedProvider(provider) {
		return fmt.Errorf("unsupported connector provider: %s", provider)
	}

	if err := r.ensureDir(); err != nil {
		return err
	}

	resource := connectorResource{
		APIVersion: apiVersion,
		Kind:       kindConnector,
		Metadata:   metadata{Name: domain.NormalizeName(cleanName)},
		Spec: connectorSpecRecord{
			DisplayName: cleanName,
			Provider:    provider,
			APIKey:      strings.TrimSpace(connector.APIKey),
			BaseURL:     strings.TrimSpace(connector.BaseURL),
		},
	}
	return writeYAML(r.connectorPath(cleanName), resource)
}

// DeleteConnector removes a connector resource file.
func (r ConnectorRepo) DeleteConnector(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("connector name is required")
	}
	if err := r.ensureDir(); err != nil {
		return err
	}
	if err := os.Remove(r.connectorPath(name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// ListConnectors returns all persisted connectors sorted alphabetically.
func (r ConnectorRepo) ListConnectors() ([]domain.Connector, error) {
	if err := r.ensureDir(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(r.baseDir)
	if err != nil {
		return nil, err
	}

	items := make([]domain.Connector, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "connector-") || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		var res connectorResource
		if err := readYAML(filepath.Join(r.baseDir, entry.Name()), &res); err != nil {
			continue
		}
		if res.Kind != kindConnector || strings.TrimSpace(res.Metadata.Name) == "" {
			continue
		}

		name := strings.TrimSpace(res.Spec.DisplayName)
		if name == "" {
			name = strings.TrimSpace(res.Metadata.Name)
		}

		items = append(items, domain.Connector{
			Name:     name,
			Provider: strings.TrimSpace(res.Spec.Provider),
			APIKey:   strings.TrimSpace(res.Spec.APIKey),
			BaseURL:  strings.TrimSpace(res.Spec.BaseURL),
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	return items, nil
}

// GetConnector retrieves a single connector by name.
func (r ConnectorRepo) GetConnector(name string) (domain.Connector, error) {
	cleanName := strings.TrimSpace(name)
	if cleanName == "" {
		return domain.Connector{}, fmt.Errorf("connector name is required")
	}

	var res connectorResource
	if err := readYAML(r.connectorPath(cleanName), &res); err != nil {
		return domain.Connector{}, err
	}

	if res.Kind != kindConnector {
		return domain.Connector{}, fmt.Errorf("resource is not a connector")
	}

	displayName := strings.TrimSpace(res.Spec.DisplayName)
	if displayName == "" {
		displayName = strings.TrimSpace(res.Metadata.Name)
	}

	return domain.Connector{
		Name:     displayName,
		Provider: strings.TrimSpace(res.Spec.Provider),
		APIKey:   strings.TrimSpace(res.Spec.APIKey),
		BaseURL:  strings.TrimSpace(res.Spec.BaseURL),
	}, nil
}

// ---- path helpers ---------------------------------------------------------- //

func (r ConnectorRepo) connectorPath(name string) string {
	return filepath.Join(r.baseDir, fmt.Sprintf("connector-%s.yaml", domain.NormalizeName(name)))
}
