package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os/exec"
	"strings"
)

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
