package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// isolateInstall points the install directory at a temp folder so tests never touch a real install.
func isolateInstall(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", home)
	dir, err := installDir()
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestSavedPortPersistsAndDefaults(t *testing.T) {
	isolateInstall(t)
	if got := savedPort(); got != defaultPort {
		t.Fatalf("savedPort() without config = %q, want %q", got, defaultPort)
	}
	if err := saveConfig(appConfig{Port: "9191"}); err != nil {
		t.Fatal(err)
	}
	if got := savedPort(); got != "9191" {
		t.Fatalf("savedPort() = %q, want 9191", got)
	}
}

func TestSavedPortFallsBackOnInvalidConfig(t *testing.T) {
	dir := isolateInstall(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := savedPort(); got != defaultPort {
		t.Fatalf("savedPort() with invalid config = %q, want %q", got, defaultPort)
	}
}

func TestParseStartArgsUsesFallback(t *testing.T) {
	got, err := parseStartArgs(nil, "9191")
	if err != nil || got != "9191" {
		t.Fatalf("parseStartArgs(nil, 9191) = %q, %v; want 9191", got, err)
	}
}

func TestStatusNotInstalledExitCode(t *testing.T) {
	isolateInstall(t)
	err := showStatus()
	var code exitCodeError
	if !errors.As(err, &code) || code.code != exitNotInstalled {
		t.Fatalf("showStatus() = %v, want exit code %d", err, exitNotInstalled)
	}
}

func TestStatusStoppedExitCode(t *testing.T) {
	dir := isolateInstall(t)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	exe, err := installedExecutable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(exe, []byte("stub"), 0o600); err != nil {
		t.Fatal(err)
	}
	err = showStatus()
	var code exitCodeError
	if !errors.As(err, &code) || code.code != exitStopped {
		t.Fatalf("showStatus() = %v, want exit code %d", err, exitStopped)
	}
}

func TestHandlePortRejectsInvalidPort(t *testing.T) {
	isolateInstall(t)
	if err := handlePort([]string{"--port=70000"}); err == nil {
		t.Fatal("handlePort(--port=70000) expected error")
	}
	if got := savedPort(); got != defaultPort {
		t.Fatalf("invalid port was saved: %q", got)
	}
}
