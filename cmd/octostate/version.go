package main

import (
	"runtime/debug"
	"strings"
)

// buildVersion is populated by release builds with the verified release tag.
// Ordinary builds leave it empty and use Go's embedded build information.
var buildVersion string

const developmentVersion = "devel"

func currentVersion() string {
	info, _ := debug.ReadBuildInfo()
	return resolveVersion(buildVersion, info)
}

func resolveVersion(injectedVersion string, info *debug.BuildInfo) string {
	if version := strings.TrimSpace(injectedVersion); version != "" {
		return version
	}
	if info != nil {
		if version := strings.TrimSpace(info.Main.Version); version != "" && version != "(devel)" {
			return version
		}
	}
	return developmentVersion
}
