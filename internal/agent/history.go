package agent

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type HistoryItem struct {
	Stage     string    `json:"stage"`
	Error     string    `json:"error"`
	Diff      string    `json:"diff"`
	Timestamp time.Time `json:"timestamp"`
}

// appendHistory writes a successful fix to .agents/fix-history.jsonl.
// Persistence failures are logged, not fatal — losing a history entry must
// not fail the recovery that already succeeded.
func appendHistory(log *slog.Logger, root string, item HistoryItem) {
	dir := filepath.Join(root, ".agents")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Warn("fix-history: mkdir failed", "dir", dir, "err", err)
		return
	}

	f, err := os.OpenFile(filepath.Join(dir, "fix-history.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		log.Warn("fix-history: open failed", "err", err)
		return
	}
	defer f.Close()

	b, err := json.Marshal(item)
	if err != nil {
		log.Warn("fix-history: marshal failed", "err", err)
		return
	}
	if _, err := f.Write(append(b, '\n')); err != nil {
		log.Warn("fix-history: write failed", "err", err)
	}
}

// buildHistoryPrompt reads recent successful fixes for the given stage and formats them as a few-shot prompt.
func buildHistoryPrompt(root, stage string) string {
	f, err := os.Open(filepath.Join(root, ".agents", "fix-history.jsonl"))
	if err != nil {
		return ""
	}
	defer f.Close()

	var items []HistoryItem
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024) // diffs can exceed the 64KB default
	for scanner.Scan() {
		var item HistoryItem
		if err := json.Unmarshal(scanner.Bytes(), &item); err == nil {
			if item.Stage == stage {
				items = append(items, item)
			}
		}
	}

	if len(items) == 0 {
		return ""
	}

	// Keep only the last 3 items to avoid blowing up the context
	if len(items) > 3 {
		items = items[len(items)-3:]
	}

	var sb strings.Builder
	sb.WriteString("\n\n## History of successful fixes for this repository\n")
	sb.WriteString("Below are examples of how similar errors were successfully fixed in the past:\n\n")

	for i, item := range items {
		// Truncate error if too long
		errStr := item.Error
		if len(errStr) > 500 {
			errStr = errStr[:500] + "...\n(truncated)"
		}

		fmt.Fprintf(&sb, "### Example %d\n**Error:**\n```\n%s\n```\n**Successful Fix (git diff):**\n```diff\n%s\n```\n\n", i+1, strings.TrimSpace(errStr), strings.TrimSpace(item.Diff))
	}

	sb.WriteString("Use these patterns if applicable.\n")
	return sb.String()
}
