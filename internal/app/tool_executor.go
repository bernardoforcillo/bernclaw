package app

import (
	"fmt"

	"github.com/bernardoforcillo/bernclaw/internal/port"
)

// ToolExecutor executes tools against the workspace
type ToolExecutor struct {
	files  port.FileService
	system port.SystemService
}

// NewToolExecutor creates a new ToolExecutor
func NewToolExecutor(files port.FileService, system port.SystemService) *ToolExecutor {
	return &ToolExecutor{files: files, system: system}
}

// Execute runs the named tool with the given parameters
func (e *ToolExecutor) Execute(toolName string, args map[string]interface{}) (interface{}, error) {
	switch toolName {
	// File operations
	case "read-file":
		path, ok := args["path"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'path'")
		}
		return e.files.ReadFile(path)

	case "write-file":
		path, ok := args["path"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'path'")
		}
		content, ok := args["content"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'content'")
		}
		return true, e.files.WriteFile(path, content)

	case "list-files":
		path, ok := args["path"].(string)
		if !ok {
			path = "."
		}
		return e.files.ListFiles(path)

	case "delete-file":
		path, ok := args["path"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'path'")
		}
		return true, e.files.DeleteFile(path)

	case "move-file":
		src, ok := args["source"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'source'")
		}
		dst, ok := args["destination"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'destination'")
		}
		return true, e.files.MoveFile(src, dst)

	case "copy-file":
		src, ok := args["source"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'source'")
		}
		dst, ok := args["destination"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'destination'")
		}
		return true, e.files.CopyFile(src, dst)

	case "create-directory":
		path, ok := args["path"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'path'")
		}
		return true, e.files.CreateDirectory(path)

	// System operations
	case "run-process":
		cmd, ok := args["command"].(string)
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'command'")
		}
		argsListRaw, ok := args["args"].([]interface{})
		var argsList []string
		if ok {
			for _, arg := range argsListRaw {
				if s, ok := arg.(string); ok {
					argsList = append(argsList, s)
				}
			}
		}
		return e.system.RunProcess(cmd, argsList)

	case "list-processes":
		return e.system.ListProcesses()

	case "kill-process":
		pidRaw, ok := args["pid"]
		if !ok {
			return nil, fmt.Errorf("missing or invalid 'pid'")
		}
		// Handle float64 which is common for JSON numbers
		var pid int
		switch v := pidRaw.(type) {
		case int:
			pid = v
		case float64:
			pid = int(v)
		default:
			return nil, fmt.Errorf("pid must be a number")
		}
		return true, e.system.KillProcess(pid)

	default:
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}
}
