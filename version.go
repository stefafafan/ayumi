package main

import (
	"regexp"
	"runtime/debug"
)

const developmentVersion = "dev"

var version = developmentVersion

var releaseVersionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

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
