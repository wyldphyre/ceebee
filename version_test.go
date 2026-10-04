package main

import "testing"

func TestConfigVersion(t *testing.T) {
	config := `# This file contains the configuration for this project.
version: '3' # the config format, not the app version

info:
  companyName: "Craig Reynolds"
  # version: "0.0.1"
  version: "1.2.3" # The application version

other:
  version: "9.9.9"
`
	if got := configVersion(config); got != "1.2.3" {
		t.Errorf("configVersion = %q, want 1.2.3", got)
	}
	if got := configVersion("info:\n  version: '2.0.0'\n"); got != "2.0.0" {
		t.Errorf("single quotes: %q", got)
	}
	if got := configVersion("version: '3'\n"); got != "unknown" {
		t.Errorf("no info section: %q", got)
	}
}

func TestEmbeddedVersion(t *testing.T) {
	if version == "unknown" || version == "" {
		t.Errorf("version from build/config.yml = %q", version)
	}
}
