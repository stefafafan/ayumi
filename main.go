package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHeading     = "AI Instructions"
	developmentVersion = "dev"
)

var version = developmentVersion

var releaseVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

type config struct {
	StorageDir string
	Heading    string
}

type promptEntry struct {
	Timestamp string `json:"timestamp"`
	Prompt    string `json:"prompt"`
}

type gitContext struct {
	Root     string
	GitDir   string
	RepoID   string
	BranchID string
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: ayumi <add|inject|version> [commit-message-file]")
		return 2
	}

	if args[0] == "version" {
		if len(args) > 1 {
			printVersionUsage(stderr)
			fmt.Fprintf(stderr, "\ngot extra arguments: %s\n\n", strings.Join(args[1:], " "))
			return 2
		}
		fmt.Fprintln(stdout, currentVersion())
		return 0
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(stderr, "ayumi: %v\n", err)
		return 1
	}

	switch args[0] {
	case "add":
		if err := addPrompt(stdin, cfg); err != nil {
			fmt.Fprintf(stderr, "ayumi add: %v\n", err)
			return 1
		}
		return 0
	case "inject":
		if len(args) < 2 {
			printInjectUsage(stderr)
			return 2
		}
		if len(args) > 2 {
			printInjectUsage(stderr)
			fmt.Fprintf(stderr, "\ngot extra arguments: %s\n\n", strings.Join(args[2:], " "))
			return 2
		}
		if err := injectInstructions(args[1], cfg); err != nil {
			fmt.Fprintf(stderr, "ayumi inject: %v\n", err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return 2
	}
}

func printVersionUsage(stderr io.Writer) {
	fmt.Fprintln(stderr, "usage: ayumi version")
}

func currentVersion() string {
	buildInfoVersion := ""
	info, ok := debug.ReadBuildInfo()
	if ok {
		buildInfoVersion = info.Main.Version
	}
	return resolveVersion(version, buildInfoVersion)
}

func resolveVersion(configuredVersion, buildInfoVersion string) string {
	if isReleaseVersion(configuredVersion) {
		return configuredVersion
	}
	if isReleaseVersion(buildInfoVersion) {
		return buildInfoVersion
	}
	return developmentVersion
}

func isReleaseVersion(value string) bool {
	return releaseVersionPattern.MatchString(value)
}

func printInjectUsage(stderr io.Writer) {
	fmt.Fprintln(stderr, "usage: ayumi inject <commit-message-file>")
}

func addPrompt(stdin io.Reader, cfg config) error {
	data, err := io.ReadAll(stdin)
	if err != nil {
		return err
	}

	prompt, err := extractPrompt(data)
	if err != nil {
		return err
	}
	if strings.TrimSpace(prompt) == "" {
		return errors.New("empty prompt")
	}

	ctx, err := currentGitContext()
	if err != nil {
		return err
	}
	if err := validateStorageDir(cfg.StorageDir, ctx); err != nil {
		return err
	}

	entry := promptEntry{
		Timestamp: time.Now().Format(time.RFC3339),
		Prompt:    prompt,
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	path := logPath(cfg, ctx)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return nil
}

func injectInstructions(messagePath string, cfg config) error {
	ctx, err := currentGitContext()
	if err != nil {
		return err
	}
	if err := validateStorageDir(cfg.StorageDir, ctx); err != nil {
		return err
	}
	if gitOperationInProgress(ctx.GitDir) {
		return nil
	}

	cutoff, err := lastCommitTime()
	if err != nil {
		return err
	}
	entries, err := readEntriesSince(logPath(cfg, ctx), cutoff)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return nil
	}

	b, err := os.ReadFile(messagePath)
	if err != nil {
		return err
	}
	message := string(b)
	if hasHeading(message, cfg.Heading) {
		return nil
	}

	updated := appendInstructionSection(message, cfg.Heading, entries)
	return os.WriteFile(messagePath, []byte(updated), 0o644)
}

func loadConfig() (config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return config{}, err
	}
	cfg := config{
		StorageDir: filepath.Join(home, ".local", "share", "ayumi"),
		Heading:    defaultHeading,
	}

	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	path := filepath.Join(configHome, "ayumi", "config.toml")
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return config{}, err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(b)))
	for scanner.Scan() {
		key, parsed, ok, err := parseConfigLine(scanner.Text())
		if err != nil {
			return config{}, fmt.Errorf("parse config %s: %w", path, err)
		}
		if !ok {
			continue
		}
		switch key {
		case "storage_dir":
			cfg.StorageDir = expandHome(parsed, home)
		case "heading":
			cfg.Heading = parsed
		}
	}
	if err := scanner.Err(); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(cfg.Heading) == "" {
		return config{}, errors.New("heading must not be empty")
	}
	return cfg, nil
}

func parseConfigLine(line string) (string, string, bool, error) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false, nil
	}
	key, value, ok := strings.Cut(line, "=")
	if !ok {
		return "", "", false, nil
	}
	key = strings.TrimSpace(key)
	if key != "storage_dir" && key != "heading" {
		return "", "", false, nil
	}
	parsed, err := parseTomlString(strings.TrimSpace(value))
	if err != nil {
		return "", "", false, err
	}
	return key, parsed, true, nil
}

func parseTomlString(value string) (string, error) {
	if value == "" {
		return "", errors.New("expected quoted string")
	}
	switch value[0] {
	case '\'':
		end := strings.IndexByte(value[1:], '\'')
		if end < 0 {
			return "", fmt.Errorf("unterminated literal string: %q", value)
		}
		closing := end + 1
		parsed := value[1:closing]
		if err := validateTomlStringRest(value[closing+1:]); err != nil {
			return "", err
		}
		return parsed, nil
	case '"':
		closing := closingBasicStringQuote(value)
		if closing < 0 {
			return "", fmt.Errorf("unterminated string: %q", value)
		}
		parsed, err := strconv.Unquote(value[:closing+1])
		if err != nil {
			return "", err
		}
		if err := validateTomlStringRest(value[closing+1:]); err != nil {
			return "", err
		}
		return parsed, nil
	default:
		return "", fmt.Errorf("expected quoted string, got %q", value)
	}
}

func closingBasicStringQuote(value string) int {
	escaped := false
	for i := 1; i < len(value); i++ {
		switch {
		case escaped:
			escaped = false
		case value[i] == '\\':
			escaped = true
		case value[i] == '"':
			return i
		}
	}
	return -1
}

func validateTomlStringRest(rest string) error {
	rest = strings.TrimSpace(rest)
	if rest == "" || strings.HasPrefix(rest, "#") {
		return nil
	}
	return fmt.Errorf("unexpected content after string: %q", rest)
}

func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

func extractPrompt(data []byte) (string, error) {
	raw := string(data)
	var decoded any
	if err := json.Unmarshal(data, &decoded); err == nil {
		if prompt, ok := promptFromDecodedHookInput(decoded); ok {
			return prompt, nil
		}
		return "", errors.New("prompt field not found in hook input")
	}
	return raw, nil
}

func promptFromDecodedHookInput(value any) (string, bool) {
	if prompt, ok := value.(string); ok {
		return prompt, true
	}
	return promptFieldFromJSON(value)
}

func promptFieldFromJSON(value any) (string, bool) {
	switch v := value.(type) {
	case map[string]any:
		for _, key := range []string{"prompt", "user_prompt", "input"} {
			if s, ok := v[key].(string); ok {
				return s, true
			}
		}
		for _, child := range v {
			if s, ok := promptFieldFromJSON(child); ok {
				return s, true
			}
		}
	case []any:
		for _, child := range v {
			if s, ok := promptFieldFromJSON(child); ok {
				return s, true
			}
		}
	}
	return "", false
}

func currentGitContext() (gitContext, error) {
	root, err := gitOutput("rev-parse", "--show-toplevel")
	if err != nil {
		return gitContext{}, errors.New("not inside a git repository")
	}
	gitDir, err := gitOutput("rev-parse", "--absolute-git-dir")
	if err != nil {
		return gitContext{}, err
	}
	remote, err := gitOutput("config", "--get", "remote.origin.url")
	if err != nil || remote == "" {
		remote = root
	}
	branch, err := gitOutput("branch", "--show-current")
	if err != nil || branch == "" {
		branch, err = gitOutput("rev-parse", "--short", "HEAD")
		if err != nil || branch == "" {
			branch = "detached"
		}
	}

	return gitContext{
		Root:     root,
		GitDir:   gitDir,
		RepoID:   hashID(remote),
		BranchID: hashID(branch),
	}, nil
}

func gitOutput(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func hashID(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func logPath(cfg config, ctx gitContext) string {
	return filepath.Join(cfg.StorageDir, "repositories", ctx.RepoID, "branches", ctx.BranchID+".jsonl")
}

func validateStorageDir(storageDir string, ctx gitContext) error {
	storageAbs, err := canonicalPath(storageDir)
	if err != nil {
		return err
	}
	rootAbs, err := canonicalPath(ctx.Root)
	if err != nil {
		return err
	}
	gitAbs, err := canonicalPath(ctx.GitDir)
	if err != nil {
		return err
	}

	if pathInside(rootAbs, storageAbs) || pathInside(gitAbs, storageAbs) {
		return fmt.Errorf("storage_dir must be outside the git repository: %s", storageDir)
	}
	return nil
}

func canonicalPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved, nil
	}
	parent := filepath.Dir(abs)
	if parent == abs {
		return filepath.Clean(abs), nil
	}
	parentResolved, err := canonicalPath(parent)
	if err != nil {
		return "", err
	}
	return filepath.Join(parentResolved, filepath.Base(abs)), nil
}

func pathInside(base, target string) bool {
	rel, err := filepath.Rel(filepath.Clean(base), filepath.Clean(target))
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func lastCommitTime() (time.Time, error) {
	out, err := gitOutput("log", "-1", "--format=%cI")
	if err != nil || out == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, out)
	if err != nil {
		return time.Time{}, err
	}
	return t, nil
}

func readEntriesSince(path string, cutoff time.Time) ([]promptEntry, error) {
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var entries []promptEntry
	reader := bufio.NewReader(f)
	for {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, err
		}
		line = strings.TrimRight(line, "\n")
		if line == "" && errors.Is(err, io.EOF) {
			break
		}
		var entry promptEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, err
		}
		t, err := time.Parse(time.RFC3339, entry.Timestamp)
		if err != nil {
			return nil, err
		}
		if cutoff.IsZero() || t.After(cutoff) {
			entries = append(entries, entry)
		}
		if errors.Is(err, io.EOF) {
			break
		}
	}
	return entries, nil
}

func gitOperationInProgress(gitDir string) bool {
	for _, state := range []string{
		"rebase-merge",
		"rebase-apply",
		"CHERRY_PICK_HEAD",
		"MERGE_HEAD",
		"REVERT_HEAD",
	} {
		if _, err := os.Stat(filepath.Join(gitDir, state)); err == nil {
			return true
		}
	}
	return false
}

func hasHeading(message, heading string) bool {
	target := heading + ":"
	for _, line := range strings.Split(message, "\n") {
		if strings.TrimSpace(line) == target {
			return true
		}
	}
	return false
}

func appendInstructionSection(message, heading string, entries []promptEntry) string {
	base := strings.TrimRight(message, "\r\n")
	if base != "" {
		base += "\n\n"
	}
	base += heading + ":\n"
	for i, entry := range entries {
		if i > 0 {
			base += "\n"
		}
		base += quotePrompt(entry.Prompt)
	}
	return base
}

func quotePrompt(prompt string) string {
	prompt = strings.ReplaceAll(prompt, "\r\n", "\n")
	prompt = strings.ReplaceAll(prompt, "\r", "\n")
	lines := strings.Split(prompt, "\n")
	var b strings.Builder
	for _, line := range lines {
		b.WriteString("> ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
