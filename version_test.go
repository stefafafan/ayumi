package main

import (
	"strings"
	"testing"
)

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
