package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
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
