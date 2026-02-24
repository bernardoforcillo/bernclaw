package tui

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	tokenNow    = "{{Now}}"
	tokenSystem = "{{System}}"
	tokenDate   = "{{Date}}"
)

func applyPromptVars(raw string, now time.Time) string {
	if strings.TrimSpace(raw) == "" {
		return raw
	}

	resolvers := map[string]func() string{
		tokenNow:    func() string { return now.Format(time.RFC3339) },
		tokenSystem: systemDescriptor,
		tokenDate:   func() string { return now.Format("2006-01-02") },
	}

	replaced := raw
	for token, resolver := range resolvers {
		replaced = strings.ReplaceAll(replaced, token, resolver())
	}
	return replaceDateOffsets(replaced, now)
}

func replaceDateOffsets(input string, now time.Time) string {
	const prefix = "{{Date:"
	const suffix = "}}"

	output := input
	for {
		start := strings.Index(output, prefix)
		if start < 0 {
			break
		}

		rest := output[start+len(prefix):]
		end := strings.Index(rest, suffix)
		if end < 0 {
			break
		}

		expr := strings.TrimSpace(rest[:end])
		offset, ok := parseDayOffset(expr)
		replacement := "{{Date:" + expr + "}}"
		if ok {
			replacement = now.AddDate(0, 0, offset).Format("2006-01-02")
		}

		targetEnd := start + len(prefix) + end + len(suffix)
		output = output[:start] + replacement + output[targetEnd:]
	}

	return output
}

func parseDayOffset(raw string) (int, bool) {
	if raw == "" {
		return 0, false
	}

	normalized := strings.TrimPrefix(raw, "+")

	value, err := strconv.Atoi(normalized)
	if err != nil {
		return 0, false
	}

	return value, true
}

func systemDescriptor() string {
	return fmt.Sprintf("os=%s arch=%s", runtime.GOOS, runtime.GOARCH)
}
