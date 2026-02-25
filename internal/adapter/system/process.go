package system

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
)

// SystemService provides access to system processes
type SystemService struct{}

// NewSystemService creates a new SystemService
func NewSystemService() *SystemService {
	return &SystemService{}
}

// RunProcess executes a command and returns its output
func (s *SystemService) RunProcess(command string, args []string) (string, error) {
	cmd := exec.Command(command, args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// If the command fails, we still want to return the output so far + error
		return out.String(), fmt.Errorf("command failed: %v, stderr: %s", err, stderr.String())
	}

	return out.String(), nil
}

// ListProcesses lists running processes
func (s *SystemService) ListProcesses() ([]domain.ProcessInfo, error) {
	// Simple implementation using ps
	// This might need adjustments for Windows
	cmd := exec.Command("ps", "-e", "-o", "pid,comm")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list processes: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	var processes []domain.ProcessInfo

	// Skip header
	if len(lines) > 0 {
		lines = lines[1:]
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}

		pidStr := parts[0]
		pid, err := strconv.Atoi(pidStr)
		if err != nil {
			continue
		}

		command := strings.Join(parts[1:], " ")
		processes = append(processes, domain.ProcessInfo{
			PID:     pid,
			Command: command,
		})
	}

	return processes, nil
}

// KillProcess terminates a process by PID
func (s *SystemService) KillProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}
	return proc.Kill()
}
