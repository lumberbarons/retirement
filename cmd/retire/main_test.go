package main

import (
	"strings"
	"testing"
)

const validConfigPath = "../../household.example.yaml"

func TestRun_NoCommandReportsUsage(t *testing.T) {
	err := run([]string{})
	if err == nil {
		t.Fatal("expected an error for no command, got nil")
	}
	if !strings.Contains(err.Error(), "no command given") {
		t.Fatalf("error %q should mention no command given", err.Error())
	}
}

func TestRun_UnknownCommandFails(t *testing.T) {
	err := run([]string{"retire"})
	if err == nil {
		t.Fatal("expected an error for an unknown command, got nil")
	}
	if !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("error %q should mention unknown command", err.Error())
	}
}

func TestRun_ProjectMissingConfigFails(t *testing.T) {
	err := run([]string{"project", "--config", "/nonexistent/nope.yaml"})
	if err == nil {
		t.Fatal("expected an error for a missing config, got nil")
	}
}

func TestRun_ProjectBadFlagFails(t *testing.T) {
	err := run([]string{"project", "--bogus"})
	if err == nil {
		t.Fatal("expected an error for an unknown flag, got nil")
	}
	if !strings.Contains(err.Error(), "usage") {
		t.Fatalf("error %q should include usage", err.Error())
	}
}

func TestRun_ProjectValidConfigSucceeds(t *testing.T) {
	if err := run([]string{"project", "--config", validConfigPath}); err != nil {
		t.Fatalf("run(project valid): %v", err)
	}
}

func TestRun_ProjectDefaultConfigPathFailsWhenAbsent(t *testing.T) {
	// No household.yaml in cmd/retire, so the default path must fail cleanly.
	err := run([]string{"project"})
	if err == nil {
		t.Fatal("expected an error when default household.yaml is absent, got nil")
	}
}
