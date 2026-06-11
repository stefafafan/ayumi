package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAddStoresPromptScopedByRepositoryAndBranch(t *testing.T) {
	home := t.TempDir()
	repo := initRepo(t)
	runGit(t, repo, "remote", "add", "origin", "git@github.com:owner/repo.git")
	runGit(t, repo, "checkout", "-b", "feature/auth")

	stdin := `{"prompt":"JWT認証を追加して\nmiddlewareに切り出して"}`
	if code := runCLI(t, repo, home, []string{"add"}, stdin); code != 0 {
		t.Fatalf("add exit code = %d, want 0", code)
	}

	logs := findJSONLFiles(t, filepath.Join(home, ".local", "share", "ayumi"))
	if len(logs) != 1 {
		t.Fatalf("jsonl files = %d, want 1: %v", len(logs), logs)
	}
	if strings.Contains(logs[0], ".git") {
		t.Fatalf("log path must not be under .git: %s", logs[0])
	}

	entries := readEntries(t, logs[0])
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0]["prompt"] != "JWT認証を追加して\nmiddlewareに切り出して" {
		t.Fatalf("stored prompt = %q", entries[0]["prompt"])
	}
	if _, err := time.Parse(time.RFC3339, entries[0]["timestamp"].(string)); err != nil {
		t.Fatalf("timestamp is not RFC3339: %v", err)
	}
}

func TestAddRejectsEmptyPrompt(t *testing.T) {
	home := t.TempDir()
	repo := initRepo(t)
	if code := runCLI(t, repo, home, []string{"add"}, `{"prompt":""}`); code == 0 {
		t.Fatalf("add exit code = %d, want non-zero", code)
	}
	if logs := findJSONLFiles(t, filepath.Join(home, ".local", "share", "ayumi")); len(logs) != 0 {
		t.Fatalf("unexpected logs: %v", logs)
	}
}

func TestAddRejectsJSONWithoutPromptField(t *testing.T) {
	home := t.TempDir()
	repo := initRepo(t)
	if code := runCLI(t, repo, home, []string{"add"}, `{"event":"UserPromptSubmit"}`); code == 0 {
		t.Fatalf("add exit code = %d, want non-zero", code)
	}
	if logs := findJSONLFiles(t, filepath.Join(home, ".local", "share", "ayumi")); len(logs) != 0 {
		t.Fatalf("unexpected logs: %v", logs)
	}
}

func TestAddRejectsStorageDirectoryInsideRepository(t *testing.T) {
	home := t.TempDir()
	repo := initRepo(t)
	writeConfig(t, home, "storage_dir = "+quoteForToml(filepath.Join(repo, ".git", "ayumi"))+"\n")

	if code := runCLI(t, repo, home, []string{"add"}, "must stay external"); code == 0 {
		t.Fatalf("add exit code = %d, want non-zero", code)
	}
	if logs := findJSONLFiles(t, filepath.Join(repo, ".git", "ayumi")); len(logs) != 0 {
		t.Fatalf("unexpected logs under .git: %v", logs)
	}
}
