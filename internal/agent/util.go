package agent

import (
	"encoding/json"
	"strings"
)

// extractJSON strips common non-JSON wrapping from model output:
// - <think>...</think> blocks emitted by reasoning models (e.g. deepseek-r1)
// - markdown code fences (```json ... ```)
// - preamble/postamble text before/after the JSON object
func extractJSON(content string) string {
	content = strings.TrimSpace(content)

	// Strip complete <think>...</think> blocks
	for {
		start := strings.Index(content, "<think>")
		end := strings.Index(content, "</think>")
		if start == -1 || end == -1 || end < start {
			break
		}
		content = strings.TrimSpace(content[:start] + content[end+len("</think>"):])
	}

	// Strip markdown code fences: ```json\n...\n```
	if strings.HasPrefix(content, "```") {
		if nl := strings.Index(content, "\n"); nl != -1 {
			content = content[nl+1:]
		}
		if idx := strings.LastIndex(content, "```"); idx != -1 {
			content = content[:idx]
		}
		content = strings.TrimSpace(content)
	}

	// If still not JSON, find the first '{' … last '}' block (handles preamble text).
	if !json.Valid([]byte(content)) {
		if start := strings.IndexByte(content, '{'); start != -1 {
			if end := strings.LastIndexByte(content, '}'); end > start {
				content = content[start : end+1]
			}
		}
	}

	return content
}
