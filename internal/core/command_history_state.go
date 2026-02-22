package core

import (
	"os"
	"path/filepath"
	"strings"
)

type commandHistoryState struct {
	filePath string
	items    []string
	cursor   int
	draft    string
}

func newCommandHistoryState(path string) commandHistoryState {
	return commandHistoryState{
		filePath: strings.TrimSpace(path),
		items:    []string{},
		cursor:   -1,
		draft:    "",
	}
}

func (state *commandHistoryState) remember(value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}

	if len(state.items) == 0 || state.items[len(state.items)-1] != trimmed {
		state.items = append(state.items, trimmed)
		if err := state.appendToDisk(trimmed); err != nil {
			return err
		}
	}
	state.cursor = -1
	state.draft = ""
	return nil
}

func (state *commandHistoryState) moveUp(currentInput string) (string, bool) {
	if len(state.items) == 0 {
		return "", false
	}

	if state.cursor == -1 {
		state.draft = currentInput
		state.cursor = len(state.items) - 1
	} else if state.cursor > 0 {
		state.cursor--
	}

	return state.items[state.cursor], true
}

func (state *commandHistoryState) moveDown() (string, bool) {
	if len(state.items) == 0 || state.cursor == -1 {
		return "", false
	}

	if state.cursor < len(state.items)-1 {
		state.cursor++
		return state.items[state.cursor], true
	}

	state.cursor = -1
	return state.draft, true
}

func (state *commandHistoryState) reverseSearch(query string, currentInput string) (string, bool) {
	if len(state.items) == 0 {
		return "", false
	}

	normalizedQuery := strings.ToLower(strings.TrimSpace(query))
	for index := len(state.items) - 1; index >= 0; index-- {
		candidate := state.items[index]
		if normalizedQuery == "" || strings.Contains(strings.ToLower(candidate), normalizedQuery) {
			state.draft = currentInput
			state.cursor = index
			return candidate, true
		}
	}

	return "", false
}

func (state *commandHistoryState) loadFromDisk() error {
	path := strings.TrimSpace(state.filePath)
	if path == "" {
		return nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	loaded := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		loaded = append(loaded, trimmed)
	}
	state.items = loaded
	if state.cursor >= len(state.items) {
		state.cursor = -1
	}
	return nil
}

func (state *commandHistoryState) appendToDisk(value string) error {
	path := strings.TrimSpace(state.filePath)
	if path == "" {
		return nil
	}

	if parent := filepath.Dir(path); parent != "" && parent != "." {
		if err := os.MkdirAll(parent, 0o755); err != nil {
			return err
		}
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(value + "\n")
	return err
}
