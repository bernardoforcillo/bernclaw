package port

import "github.com/bernardoforcillo/bernclaw/internal/domain"

// SystemService defines operations for system process management
type SystemService interface {
	RunProcess(command string, args []string) (string, error)
	ListProcesses() ([]domain.ProcessInfo, error)
	KillProcess(pid int) error
}
