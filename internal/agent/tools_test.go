package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRepoFS_Resolve_RejectsEscapes(t *testing.T) {
	fs := repoFS{root: "/repo"}
	for _, p := range []string{"../escape", "../../etc/passwd", "/etc/passwd", "sub/../../escape"} {
		if _, err := fs.resolve(p); err == nil {
			t.Errorf("resolve(%q) should be rejected", p)
		}
	}
}

func TestRepoFS_Resolve_AllowsInRepo(t *testing.T) {
	fs := repoFS{root: "/repo"}
	want := map[string]string{
		"a.go":     "/repo/a.go",
		"sub/b.go": "/repo/sub/b.go",
		"./c.go":   "/repo/c.go",
	}
	for p, exp := range want {
		got, err := fs.resolve(p)
		if err != nil || got != exp {
			t.Errorf("resolve(%q) = %q, %v; want %q, nil", p, got, err, exp)
		}
	}
}

func TestRepoFS_WriteThenRead(t *testing.T) {
	dir := t.TempDir()
	fs := repoFS{root: dir}
	ctx := context.Background()

	if _, err := fs.write(ctx, &writeFileReq{Path: "pkg/x.go", Content: "package pkg\n"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "pkg", "x.go")); err != nil {
		t.Fatalf("file not created: %v", err)
	}
	got, err := fs.read(ctx, &readFileReq{Path: "pkg/x.go"})
	if err != nil || got != "package pkg\n" {
		t.Errorf("read = %q, %v; want %q", got, err, "package pkg\n")
	}
}

func TestRepoFS_WriteRejectsEscape(t *testing.T) {
	dir := t.TempDir()
	fs := repoFS{root: dir}
	if _, err := fs.write(context.Background(), &writeFileReq{Path: "../evil.go", Content: "x"}); err == nil {
		t.Error("write outside repo root should be rejected")
	}
}

func TestRepoTools_BuildsReadAndWrite(t *testing.T) {
	tools, err := RepoTools(t.TempDir())
	if err != nil {
		t.Fatalf("RepoTools: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("want 2 tools, got %d", len(tools))
	}
	names := map[string]bool{}
	for _, tl := range tools {
		info, err := tl.Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		names[info.Name] = true
	}
	for _, n := range []string{"read_file", "write_file"} {
		if !names[n] {
			t.Errorf("missing tool %q", n)
		}
	}
}
