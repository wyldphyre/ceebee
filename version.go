package main

import (
	_ "embed"
	"strings"
)

//go:embed build/config.yml
var buildConfig string

// version is the app version: info.version in build/config.yml, the one place
// it is set. The platform build files are updated from the same value by the
// common:sync:build-assets task.
var version = configVersion(buildConfig)

// configVersion reads info.version from the Wails build config. It only
// understands that one key, which saves a YAML dependency.
func configVersion(config string) string {
	inInfo := false
	for _, line := range strings.Split(config, "\n") {
		line, _, _ = strings.Cut(line, "#")
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			inInfo = trimmed == "info:"
			continue
		}
		if key, value, ok := strings.Cut(trimmed, ":"); inInfo && ok && key == "version" {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return "unknown"
}
