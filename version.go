package main

import (
	"cmp"
	_ "embed"
	"strings"
)

//go:embed build/config.yml
var buildConfig string

// version is the app version: info.version in build/config.yml, the one place
// it is set. The platform build files are updated from the same value by the
// common:sync:build-assets task.
var version = cmp.Or(configInfo(buildConfig, "version"), "unknown")

// appID identifies the app to the desktop: info.productIdentifier in
// build/config.yml. On Linux it has to match the desktop entry, or the
// window gets no icon.
var appID = configInfo(buildConfig, "productIdentifier")

// configInfo reads a key from the info section of the Wails build config, or
// returns "" if it isn't there. It only understands simple "key: value"
// lines, which saves a YAML dependency.
func configInfo(config, name string) string {
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
		if key, value, ok := strings.Cut(trimmed, ":"); inInfo && ok && key == name {
			return strings.Trim(strings.TrimSpace(value), `"'`)
		}
	}
	return ""
}
