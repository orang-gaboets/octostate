package main

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersionPrefersInjectedReleaseVersion(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v1.3.0"}}

	if got := resolveVersion("v1.4.0", info); got != "v1.4.0" {
		t.Fatalf("resolveVersion() = %q, want %q", got, "v1.4.0")
	}
}

func TestResolveVersionUsesModuleBuildInformation(t *testing.T) {
	info := &debug.BuildInfo{Main: debug.Module{Version: "v1.4.0"}}

	if got := resolveVersion("", info); got != "v1.4.0" {
		t.Fatalf("resolveVersion() = %q, want %q", got, "v1.4.0")
	}
}

func TestResolveVersionUsesDevelopmentFallback(t *testing.T) {
	tests := []struct {
		name string
		info *debug.BuildInfo
	}{
		{name: "nil build information"},
		{name: "empty module version", info: &debug.BuildInfo{}},
		{name: "devel module version", info: &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion("", tt.info); got != developmentVersion {
				t.Fatalf("resolveVersion() = %q, want %q", got, developmentVersion)
			}
		})
	}
}
