package main

import (
	"fmt"
	"runtime"
	"runtime/debug"
)

// BuildInfo holds version metadata populated at link time via -ldflags.
type BuildInfo struct {
	Version   string
	Commit    string
	BuildDate string
	GoVersion string
	OS        string
	Arch      string
}

// newBuildInfo assembles BuildInfo from the package-level ldflags variables
// and runtime/debug.ReadBuildInfo as a fallback.
func newBuildInfo(version, commit, buildDate string) BuildInfo {
	bi := BuildInfo{
		Version:   version,
		Commit:    commit,
		BuildDate: buildDate,
		GoVersion: runtime.Version(),
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
	}

	// If commit or buildDate weren't injected, try to infer from debug info.
	if bi.Commit == "unknown" || bi.BuildDate == "unknown" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, setting := range info.Settings {
				switch setting.Key {
				case "vcs.revision":
					if bi.Commit == "unknown" {
						bi.Commit = setting.Value
					}
				case "vcs.time":
					if bi.BuildDate == "unknown" {
						bi.BuildDate = setting.Value
					}
				}
			}
		}
	}

	return bi
}

// String returns a human-readable version string.
func (b BuildInfo) String() string {
	return fmt.Sprintf("GhostCLI %s (commit: %s, built: %s, %s/%s, %s)",
		b.Version, b.Commit, b.BuildDate, b.OS, b.Arch, b.GoVersion)
}

// Short returns a compact version identifier.
func (b BuildInfo) Short() string {
	commitShort := b.Commit
	if len(commitShort) > 7 {
		commitShort = commitShort[:7]
	}
	return fmt.Sprintf("%s-%s", b.Version, commitShort)
}
