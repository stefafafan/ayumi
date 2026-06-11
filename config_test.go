package main

import (
	"path/filepath"
	"testing"
)

func TestConfigParsesTomlLiteralStringsAndComments(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, `# ayumi configuration
storage_dir = '~/literal-prompts' # TOML literal string with an inline comment
heading = 'Prompt History'
`)
	repo := initRepo(t)

	if code := runCLI(t, repo, home, []string{"add"}, "literal string config"); code != 0 {
		t.Fatalf("add exit code = %d", code)
	}

	logs := findJSONLFiles(t, filepath.Join(home, "literal-prompts"))
	if len(logs) != 1 {
		t.Fatalf("jsonl files = %d, want 1: %v", len(logs), logs)
	}
}
