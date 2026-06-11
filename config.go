package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

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
