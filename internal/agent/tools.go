// Package agent holds the bounded, reversible LLM-backed recovery steps. It
// implements interfaces defined in internal/pipeline (e.g. Recoverable); the
// dependency only ever points agent -> pipeline, never the reverse.
package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	toolutils "github.com/cloudwego/eino/components/tool/utils"
)

type repoFS struct{ root string }

type readFileReq struct {
	Path string `json:"path" jsonschema_description:"repository-relative path of the file to read"`
}

type writeFileReq struct {
	Path    string `json:"path" jsonschema_description:"repository-relative path of the file to overwrite"`
	Content string `json:"content" jsonschema_description:"the full new contents of the file"`
}

// resolve cleans a model-supplied path and confirms it stays within the repo
// root. Model-generated paths are untrusted (requirements §4: path-traversal
// guard).
func (fs repoFS) resolve(p string) (string, error) {
	clean := filepath.Clean(p)
	if filepath.IsAbs(clean) {
		return "", fmt.Errorf("absolute path not allowed: %q", p)
	}
	full := filepath.Join(fs.root, clean)
	rel, err := filepath.Rel(fs.root, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes repo root: %q", p)
	}
	return full, nil
}

func (fs repoFS) read(_ context.Context, req *readFileReq) (string, error) {
	full, err := fs.resolve(req.Path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (fs repoFS) write(_ context.Context, req *writeFileReq) (string, error) {
	full, err := fs.resolve(req.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(full, []byte(req.Content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(req.Content), req.Path), nil
}

// RepoTools returns the repo-scoped read_file / write_file tools rooted at root.
func RepoTools(root string) ([]tool.BaseTool, error) {
	fs := repoFS{root: root}
	rt, err := toolutils.InferTool("read_file", "Read a UTF-8 text file at a repository-relative path.", fs.read)
	if err != nil {
		return nil, err
	}
	wt, err := toolutils.InferTool("write_file", "Overwrite a text file at a repository-relative path with new contents.", fs.write)
	if err != nil {
		return nil, err
	}
	return []tool.BaseTool{rt, wt}, nil
}
