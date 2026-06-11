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
)

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
