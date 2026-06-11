package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
