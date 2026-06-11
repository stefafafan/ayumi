package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInjectAddsInstructionsSinceLastCommitWithConfiguredHeading(t *testing.T) {
	home := t.TempDir()
	writeConfig(t, home, `storage_dir = "~/prompts"
heading = "Prompt History"
`)
	repo := initRepo(t)
	runGit(t, repo, "remote", "add", "origin", "https://example.com/repo.git")
	writeFile(t, filepath.Join(repo, "README.md"), "initial\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")
	time.Sleep(1100 * time.Millisecond)

	if code := runCLI(t, repo, home, []string{"add"}, "JWT認証を追加して"); code != 0 {
		t.Fatalf("add #1 exit code = %d", code)
	}
	if code := runCLI(t, repo, home, []string{"add"}, `{"user_prompt":"middlewareに切り出して"}`); code != 0 {
		t.Fatalf("add #2 exit code = %d", code)
	}

	msg := filepath.Join(repo, "COMMIT_EDITMSG")
	writeFile(t, msg, "feat: add JWT middleware\n")
	if code := runCLI(t, repo, home, []string{"inject", msg}, ""); code != 0 {
		t.Fatalf("inject exit code = %d, want 0", code)
	}

	got := readFile(t, msg)
	want := "feat: add JWT middleware\n\nPrompt History:\n> JWT認証を追加して\n\n> middlewareに切り出して\n"
	if got != want {
		t.Fatalf("commit message:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestInjectDoesNotInsertWhenNoPromptsSinceLastCommit(t *testing.T) {
	home := t.TempDir()
	repo := initRepo(t)
	runGit(t, repo, "remote", "add", "origin", "https://example.com/repo.git")
	if code := runCLI(t, repo, home, []string{"add"}, "before commit"); code != 0 {
		t.Fatalf("add exit code = %d", code)
	}
	writeFile(t, filepath.Join(repo, "README.md"), "initial\n")
	runGit(t, repo, "add", "README.md")
	runGit(t, repo, "commit", "-m", "initial")

	msg := filepath.Join(repo, "COMMIT_EDITMSG")
	writeFile(t, msg, "docs: update readme\n")
	if code := runCLI(t, repo, home, []string{"inject", msg}, ""); code != 0 {
		t.Fatalf("inject exit code = %d", code)
	}
	if got := readFile(t, msg); got != "docs: update readme\n" {
		t.Fatalf("commit message changed unexpectedly: %q", got)
	}
}

func TestInjectSkipsRebaseCherryPickMergeAndRevertStates(t *testing.T) {
	states := map[string]string{
		"rebase-merge":     "dir",
		"rebase-apply":     "dir",
		"CHERRY_PICK_HEAD": "file",
		"MERGE_HEAD":       "file",
		"REVERT_HEAD":      "file",
	}
	for state, kind := range states {
		t.Run(state, func(t *testing.T) {
			home := t.TempDir()
			repo := initRepo(t)
			if code := runCLI(t, repo, home, []string{"add"}, "should not inject"); code != 0 {
				t.Fatalf("add exit code = %d", code)
			}
			gitDir := filepath.Join(repo, ".git")
			if kind == "dir" {
				if err := os.Mkdir(filepath.Join(gitDir, state), 0o755); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFile(t, filepath.Join(gitDir, state), "state\n")
			}
			msg := filepath.Join(repo, "COMMIT_EDITMSG")
			writeFile(t, msg, "commit subject\n")
			if code := runCLI(t, repo, home, []string{"inject", msg}, ""); code != 0 {
				t.Fatalf("inject exit code = %d", code)
			}
			if got := readFile(t, msg); got != "commit subject\n" {
				t.Fatalf("commit message changed during %s: %q", state, got)
			}
		})
	}
}

func TestInjectPreservesMultilinePromptText(t *testing.T) {
	home := t.TempDir()
	repo := initRepo(t)
	if code := runCLI(t, repo, home, []string{"add"}, "JWT認証を追加して\nissuer/audienceも検証して"); code != 0 {
		t.Fatalf("add exit code = %d", code)
	}
	msg := filepath.Join(repo, "COMMIT_EDITMSG")
	writeFile(t, msg, "feat: auth\n")
	if code := runCLI(t, repo, home, []string{"inject", msg}, ""); code != 0 {
		t.Fatalf("inject exit code = %d", code)
	}

	got := readFile(t, msg)
	want := "feat: auth\n\nAI Instructions:\n> JWT認証を追加して\n> issuer/audienceも検証して\n"
	if got != want {
		t.Fatalf("commit message:\n--- got ---\n%s--- want ---\n%s", got, want)
	}
}

func TestInjectHandlesLargePromptWithoutTruncation(t *testing.T) {
	home := t.TempDir()
	repo := initRepo(t)
	prompt := strings.Repeat("長い指示", 25000)
	if code := runCLI(t, repo, home, []string{"add"}, prompt); code != 0 {
		t.Fatalf("add exit code = %d", code)
	}

	msg := filepath.Join(repo, "COMMIT_EDITMSG")
	writeFile(t, msg, "feat: large prompt\n")
	if code := runCLI(t, repo, home, []string{"inject", msg}, ""); code != 0 {
		t.Fatalf("inject exit code = %d", code)
	}

	got := readFile(t, msg)
	if !strings.Contains(got, "> "+prompt+"\n") {
		t.Fatalf("large prompt was not preserved in commit message")
	}
}

func TestInjectReportsExtraArgumentsForConfigBasedHooks(t *testing.T) {
	home := t.TempDir()
	repo := initRepo(t)
	msg := filepath.Join(repo, "COMMIT_EDITMSG")

	code, stderr := runCLIWithStderr(t, repo, home, []string{"inject", msg, "message"}, "")
	if code != 2 {
		t.Fatalf("inject exit code = %d, want 2", code)
	}
	for _, want := range []string{
		"usage: ayumi inject <commit-message-file>",
		"got extra arguments: message",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr)
		}
	}
}
