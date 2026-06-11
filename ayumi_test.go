package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func TestVersionPrintsVersion(t *testing.T) {
	home := t.TempDir()
	repo := initRepo(t)

	code, stdout, stderr := runCLIWithOutput(t, repo, home, []string{"version"}, "")
	if code != 0 {
		t.Fatalf("version exit code = %d, want 0\nstderr:\n%s", code, stderr)
	}
	if got := strings.TrimSpace(stdout); got == "" || strings.Contains(got, "\n") {
		t.Fatalf("stdout = %q, want a single version line", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty", stderr)
	}
}

func TestVersionRejectsExtraArguments(t *testing.T) {
	home := t.TempDir()
	repo := initRepo(t)

	code, stdout, stderr := runCLIWithOutput(t, repo, home, []string{"version", "extra"}, "")
	if code != 2 {
		t.Fatalf("version exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Fatalf("stdout = %q, want empty", stdout)
	}
	for _, want := range []string{
		"usage: ayumi version",
		"got extra arguments: extra",
	} {
		if !strings.Contains(stderr, want) {
			t.Fatalf("stderr missing %q:\n%s", want, stderr)
		}
	}
}

func TestIsReleaseVersion(t *testing.T) {
	tests := map[string]bool{
		"v0.1.0":                               true,
		"v10.20.30":                            true,
		"0.1.0":                                false,
		"v0.1":                                 false,
		"v0.1.0-rc.1":                          false,
		"v0.1.1-0.20260611013905-11a21639a27c": false,
		"v0.1.1-0.20260611013905-11a21639a27c+dirty": false,
		"dev": false,
	}
	for value, want := range tests {
		t.Run(value, func(t *testing.T) {
			if got := isReleaseVersion(value); got != want {
				t.Fatalf("isReleaseVersion(%q) = %v, want %v", value, got, want)
			}
		})
	}
}

func TestResolveVersion(t *testing.T) {
	tests := []struct {
		name              string
		configuredVersion string
		buildInfoVersion  string
		want              string
	}{
		{
			name:              "configured release version wins",
			configuredVersion: "v0.1.1",
			buildInfoVersion:  "v0.1.0",
			want:              "v0.1.1",
		},
		{
			name:              "build info release version",
			configuredVersion: developmentVersion,
			buildInfoVersion:  "v0.1.0",
			want:              "v0.1.0",
		},
		{
			name:              "pseudo version falls back to dev",
			configuredVersion: developmentVersion,
			buildInfoVersion:  "v0.1.1-0.20260611013905-11a21639a27c+dirty",
			want:              developmentVersion,
		},
		{
			name:              "invalid configured version falls back to dev",
			configuredVersion: "v0.1.1-rc.1",
			buildInfoVersion:  "(devel)",
			want:              developmentVersion,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.configuredVersion, tt.buildInfoVersion); got != tt.want {
				t.Fatalf("resolveVersion(%q, %q) = %q, want %q", tt.configuredVersion, tt.buildInfoVersion, got, tt.want)
			}
		})
	}
}

func runCLI(t *testing.T, repo, home string, args []string, stdin string) int {
	t.Helper()
	code, _ := runCLIWithStderr(t, repo, home, args, stdin)
	return code
}

func runCLIWithStderr(t *testing.T, repo, home string, args []string, stdin string) (int, string) {
	t.Helper()
	code, _, stderr := runCLIWithOutput(t, repo, home, args, stdin)
	return code, stderr
}

func runCLIWithOutput(t *testing.T, repo, home string, args []string, stdin string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(testBinary(t), args...)
	cmd.Dir = repo
	cmd.Stdin = strings.NewReader(stdin)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = append(os.Environ(),
		"HOME="+home,
		"XDG_CONFIG_HOME="+filepath.Join(home, ".config"),
	)
	err := cmd.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), stdout.String(), stderr.String()
	}
	t.Fatalf("run ayumi: %v", err)
	return -1, stdout.String(), stderr.String()
}

func testBinary(t *testing.T) string {
	t.Helper()
	exe := os.Getenv("AYUMI_TEST_BINARY")
	if exe == "" {
		t.Fatal("AYUMI_TEST_BINARY is not set")
	}
	return exe
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func writeConfig(t *testing.T, home, content string) {
	t.Helper()
	path := filepath.Join(home, ".config", "ayumi", "config.toml")
	writeFile(t, path, content)
}

func quoteForToml(value string) string {
	b, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func findJSONLFiles(t *testing.T, root string) []string {
	t.Helper()
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".jsonl") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func readEntries(t *testing.T, path string) []map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	var entries []map[string]any
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var entry map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			t.Fatalf("invalid jsonl: %v", err)
		}
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	return entries
}

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "ayumi-test-bin-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmp)

	exe := filepath.Join(tmp, "ayumi")
	if runtime.GOOS == "windows" {
		exe += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", exe, ".")
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Stderr.Write(out)
		os.Exit(1)
	}
	os.Setenv("AYUMI_TEST_BINARY", exe)
	os.Exit(m.Run())
}
