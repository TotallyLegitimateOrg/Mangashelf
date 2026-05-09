package buildinfo

import (
	"runtime/debug"
	"strings"
	"sync"
)

var (
	Version = "dev"
	Commit  = "unknown"
	BuiltAt = "unknown"

	currentOnce sync.Once
	currentInfo Info
)

type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	BuiltAt string `json:"builtAt"`
}

func Current() Info {
	currentOnce.Do(func() {
		currentInfo = Info{
			Version: normalizeValue(Version, "dev"),
			Commit:  normalizeValue(Commit, "unknown"),
			BuiltAt: normalizeValue(BuiltAt, "unknown"),
		}

		if build, ok := debug.ReadBuildInfo(); ok {
			applyRuntimeBuildInfo(&currentInfo, build)
		}
	})

	return currentInfo
}

func applyRuntimeBuildInfo(info *Info, build *debug.BuildInfo) {
	for _, setting := range build.Settings {
		switch setting.Key {
		case "vcs.revision":
			if info.Commit == "unknown" {
				info.Commit = normalizeValue(setting.Value, info.Commit)
			}
		case "vcs.time":
			if info.BuiltAt == "unknown" {
				info.BuiltAt = normalizeValue(setting.Value, info.BuiltAt)
			}
		}
	}
}

func normalizeValue(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == "(devel)" {
		return fallback
	}
	return value
}
