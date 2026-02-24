// Package workspace provides file system access to workspace configuration
package workspace

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bernardoforcillo/bernclaw/internal/domain"
	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// FileSystemWorkspace provides access to workspace files on disk
type FileSystemWorkspace struct {
	rootPath string
	tools    []port.Tool
	skills   map[string]port.Skill
	agents   []domain.Spec

	mu sync.RWMutex
}

// NewFileSystemWorkspace creates a workspace backed by the file system
func NewFileSystemWorkspace(rootPath string) *FileSystemWorkspace {
	return &FileSystemWorkspace{
		rootPath: rootPath,
		tools:    make([]port.Tool, 0),
		skills:   make(map[string]port.Skill),
		agents:   make([]domain.Spec, 0),
	}
}

func checkPath(absPath, absRoot string) bool {
	return absPath == absRoot || strings.HasPrefix(absPath, absRoot+string(os.PathSeparator))
}

// ReadFile reads a file from the workspace
func (w *FileSystemWorkspace) ReadFile(relativePath string) (string, error) {
	fullPath := filepath.Join(w.rootPath, relativePath)

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("invalid path: %w", err)
	}

	absRoot, _ := filepath.Abs(w.rootPath)
	if !checkPath(absPath, absRoot) {
		return "", fmt.Errorf("path traversal not allowed: %s", relativePath)
	}

	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", relativePath, err)
	}

	return string(content), nil
}

// WriteFile writes a file to the workspace
func (w *FileSystemWorkspace) WriteFile(relativePath string, content string) error {
	fullPath := filepath.Join(w.rootPath, relativePath)

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	absRoot, _ := filepath.Abs(w.rootPath)
	if !checkPath(absPath, absRoot) {
		return fmt.Errorf("path traversal not allowed: %s", relativePath)
	}

	err = os.WriteFile(absPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", relativePath, err)
	}

	return nil
}

// ListFiles lists files in the workspace
func (w *FileSystemWorkspace) ListFiles(relativePath string) ([]string, error) {
	fullPath := filepath.Join(w.rootPath, relativePath)

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	absRoot, _ := filepath.Abs(w.rootPath)
	if !checkPath(absPath, absRoot) {
		return nil, fmt.Errorf("path traversal not allowed: %s", relativePath)
	}

	files, err := os.ReadDir(absPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list files in %s: %w", relativePath, err)
	}

	var names []string
	for _, f := range files {
		name := f.Name()
		if f.IsDir() {
			name += "/"
		}
		names = append(names, name)
	}

	return names, nil
}

// DeleteFile deletes a file or directory from the workspace
func (w *FileSystemWorkspace) DeleteFile(relativePath string) error {
	fullPath := filepath.Join(w.rootPath, relativePath)

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	absRoot, _ := filepath.Abs(w.rootPath)
	if !checkPath(absPath, absRoot) {
		return fmt.Errorf("path traversal not allowed: %s", relativePath)
	}

	if absPath == absRoot {
		return fmt.Errorf("cannot delete workspace root")
	}

	err = os.RemoveAll(absPath)
	if err != nil {
		return fmt.Errorf("failed to delete %s: %w", relativePath, err)
	}

	return nil
}

// MoveFile moves or renames a file in the workspace
func (w *FileSystemWorkspace) MoveFile(source, dest string) error {
	fullSource := filepath.Join(w.rootPath, source)
	fullDest := filepath.Join(w.rootPath, dest)

	absSource, err := filepath.Abs(fullSource)
	if err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}

	absDest, err := filepath.Abs(fullDest)
	if err != nil {
		return fmt.Errorf("invalid dest path: %w", err)
	}

	absRoot, _ := filepath.Abs(w.rootPath)
	if !checkPath(absSource, absRoot) || !checkPath(absDest, absRoot) {
		return fmt.Errorf("path traversal not allowed")
	}

	err = os.Rename(absSource, absDest)
	if err != nil {
		return fmt.Errorf("failed to move %s to %s: %w", source, dest, err)
	}

	return nil
}

// CopyFile copies a file or directory in the workspace
func (w *FileSystemWorkspace) CopyFile(source, dest string) error {
	fullSource := filepath.Join(w.rootPath, source)
	fullDest := filepath.Join(w.rootPath, dest)

	absSource, err := filepath.Abs(fullSource)
	if err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}

	absDest, err := filepath.Abs(fullDest)
	if err != nil {
		return fmt.Errorf("invalid dest path: %w", err)
	}

	absRoot, _ := filepath.Abs(w.rootPath)
	if !checkPath(absSource, absRoot) || !checkPath(absDest, absRoot) {
		return fmt.Errorf("path traversal not allowed")
	}

	info, err := os.Stat(absSource)
	if err != nil {
		return err
	}

	if info.IsDir() {
		return copyDir(absSource, absDest)
	}
	return copyFile(absSource, absDest)
}

func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}

func copyDir(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	err = os.MkdirAll(dst, info.Mode())
	if err != nil {
		return err
	}

	infos, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, f := range infos {
		srcPath := filepath.Join(src, f.Name())
		dstPath := filepath.Join(dst, f.Name())

		if f.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// CreateDirectory creates a new directory in the workspace
func (w *FileSystemWorkspace) CreateDirectory(relativePath string) error {
	fullPath := filepath.Join(w.rootPath, relativePath)

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	absRoot, _ := filepath.Abs(w.rootPath)
	if !checkPath(absPath, absRoot) {
		return fmt.Errorf("path traversal not allowed: %s", relativePath)
	}

	err = os.MkdirAll(absPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", relativePath, err)
	}

	return nil
}

// GetAgentsByRole returns agents with a specific role
func (w *FileSystemWorkspace) GetAgentsByRole(role string) []domain.Spec {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var result []domain.Spec
	for _, agent := range w.agents {
		if agent.Name == role {
			result = append(result, agent)
		}
	}

	return result
}

// GetTools returns all available tools
func (w *FileSystemWorkspace) GetTools() []port.Tool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return append([]port.Tool{}, w.tools...)
}

// GetSkills returns all available skills
func (w *FileSystemWorkspace) GetSkills() map[string]port.Skill {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Return a copy to prevent modification
	result := make(map[string]port.Skill)
	for k, v := range w.skills {
		result[k] = v
	}

	return result
}

// RegisterTool adds a tool to the workspace
func (w *FileSystemWorkspace) RegisterTool(tool port.Tool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.tools = append(w.tools, tool)
}

// RegisterSkill adds a skill to the workspace
func (w *FileSystemWorkspace) RegisterSkill(name string, skill port.Skill) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.skills[name] = skill
}

// RegisterAgent adds an agent spec to the workspace
func (w *FileSystemWorkspace) RegisterAgent(agent domain.Spec) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.agents = append(w.agents, agent)
}

// MockWorkspace for testing without file system
type MockWorkspace struct {
	files  map[string]string
	tools  []port.Tool
	skills map[string]port.Skill
	agents []domain.Spec

	mu sync.RWMutex
}

// NewMockWorkspace creates an empty mock workspace
func NewMockWorkspace() *MockWorkspace {
	return &MockWorkspace{
		files:  make(map[string]string),
		tools:  make([]port.Tool, 0),
		skills: make(map[string]port.Skill),
		agents: make([]domain.Spec, 0),
	}
}

// ReadFile reads a file from the mock workspace
func (m *MockWorkspace) ReadFile(relativePath string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	content, exists := m.files[relativePath]
	if !exists {
		return "", fmt.Errorf("file not found: %s", relativePath)
	}

	return content, nil
}

// WriteFile writes a file to the mock workspace
func (m *MockWorkspace) WriteFile(relativePath string, content string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[relativePath] = content
	return nil
}

// GetAgentsByRole returns agents with a specific role
func (m *MockWorkspace) GetAgentsByRole(role string) []domain.Spec {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []domain.Spec
	for _, agent := range m.agents {
		if agent.Name == role {
			result = append(result, agent)
		}
	}

	return result
}

// GetTools returns all available tools
func (m *MockWorkspace) GetTools() []port.Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]port.Tool{}, m.tools...)
}

// GetSkills returns all available skills
func (m *MockWorkspace) GetSkills() map[string]port.Skill {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]port.Skill)
	for k, v := range m.skills {
		result[k] = v
	}

	return result
}

// SetFile sets a file in the mock workspace (for testing)
func (m *MockWorkspace) SetFile(path string, content string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[path] = content
}

// RegisterTool adds a tool
func (m *MockWorkspace) RegisterTool(tool port.Tool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tools = append(m.tools, tool)
}

// RegisterSkill adds a skill
func (m *MockWorkspace) RegisterSkill(name string, skill port.Skill) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.skills[name] = skill
}

// RegisterAgent adds an agent spec
func (m *MockWorkspace) RegisterAgent(agent domain.Spec) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.agents = append(m.agents, agent)
}
