package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

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
