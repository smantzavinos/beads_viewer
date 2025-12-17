package main_test

import (
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportPages_IncludesHistoryAndRunsHooks(t *testing.T) {
	bv := buildBvBinary(t)
	stageViewerAssets(t, bv)

	repoDir, _ := createHistoryRepo(t)
	exportDir := filepath.Join(repoDir, "bv-pages")

	// Configure hooks to prove pre/post phases run.
	if err := os.MkdirAll(filepath.Join(repoDir, ".bv"), 0o755); err != nil {
		t.Fatalf("mkdir .bv: %v", err)
	}
	hooksYAML := `hooks:
  pre-export:
    - name: pre
      command: 'mkdir -p "$BV_EXPORT_PATH" && echo pre > "$BV_EXPORT_PATH/pre-hook.txt"'
  post-export:
    - name: post
      command: 'echo post > "$BV_EXPORT_PATH/post-hook.txt"'
`
	if err := os.WriteFile(filepath.Join(repoDir, ".bv", "hooks.yaml"), []byte(hooksYAML), 0o644); err != nil {
		t.Fatalf("write hooks.yaml: %v", err)
	}

	cmd := exec.Command(bv,
		"--export-pages", exportDir,
		"--pages-include-history",
		"--pages-include-closed",
	)
	cmd.Dir = repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("--export-pages failed: %v\n%s", err, out)
	}

	// Core artifacts.
	for _, p := range []string{
		filepath.Join(exportDir, "index.html"),
		filepath.Join(exportDir, "beads.sqlite3"),
		filepath.Join(exportDir, "beads.sqlite3.config.json"),
		filepath.Join(exportDir, "data", "meta.json"),
		filepath.Join(exportDir, "data", "triage.json"),
		filepath.Join(exportDir, "data", "history.json"),
		filepath.Join(exportDir, "pre-hook.txt"),
		filepath.Join(exportDir, "post-hook.txt"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing export artifact %s: %v", p, err)
		}
	}

	// Viewer fix: d3.js script should include crossorigin (bv-z14d).
	indexBytes, err := os.ReadFile(filepath.Join(exportDir, "index.html"))
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	if !strings.Contains(string(indexBytes), "crossorigin") {
		t.Fatalf("index.html missing crossorigin attribute")
	}

	// History JSON should include at least one commit entry.
	historyBytes, err := os.ReadFile(filepath.Join(exportDir, "data", "history.json"))
	if err != nil {
		t.Fatalf("read history.json: %v", err)
	}
	var history struct {
		Commits []struct {
			SHA string `json:"sha"`
		} `json:"commits"`
	}
	if err := json.Unmarshal(historyBytes, &history); err != nil {
		t.Fatalf("history.json decode: %v", err)
	}
	if len(history.Commits) == 0 || history.Commits[0].SHA == "" {
		t.Fatalf("expected at least one commit in history.json, got %+v", history.Commits)
	}
}

func stageViewerAssets(t *testing.T, bvPath string) {
	t.Helper()
	root := findRepoRoot(t)
	src := filepath.Join(root, "pkg", "export", "viewer_assets")
	dst := filepath.Join(filepath.Dir(bvPath), "pkg", "export", "viewer_assets")

	if err := copyDirRecursive(src, dst); err != nil {
		t.Fatalf("stage viewer assets: %v", err)
	}
}

func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("go.mod not found starting at %s", dir)
		}
		dir = parent
	}
}

func copyDirRecursive(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		if err := copyFile(srcPath, dstPath); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
