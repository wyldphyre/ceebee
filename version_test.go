package main

import "testing"

func TestConfigInfo(t *testing.T) {
	config := `# This file contains the configuration for this project.
version: '3' # the config format, not the app version

info:
  companyName: "Craig Reynolds"
  # version: "0.0.1"
  version: "1.2.3" # The application version

other:
  version: "9.9.9"
`
	if got := configInfo(config, "version"); got != "1.2.3" {
		t.Errorf("version = %q, want 1.2.3", got)
	}
	if got := configInfo(config, "companyName"); got != "Craig Reynolds" {
		t.Errorf("companyName = %q, want Craig Reynolds", got)
	}
	if got := configInfo("info:\n  version: '2.0.0'\n", "version"); got != "2.0.0" {
		t.Errorf("single quotes: %q", got)
	}
	if got := configInfo("version: '3'\n", "version"); got != "" {
		t.Errorf("no info section: %q", got)
	}
}

func TestEmbeddedConfig(t *testing.T) {
	if version == "unknown" || version == "" {
		t.Errorf("version from build/config.yml = %q", version)
	}
	if appID == "" {
		t.Error("no productIdentifier in build/config.yml")
	}
}
