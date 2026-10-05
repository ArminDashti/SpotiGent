package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseStartArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
		fail bool
	}{
		{name: "default", want: "9090"},
		{name: "custom", args: []string{"--port=12345"}, want: "12345"},
		{name: "zero", args: []string{"--port=0"}, fail: true},
		{name: "too high", args: []string{"--port=65536"}, fail: true},
		{name: "not numeric", args: []string{"--port=abc"}, fail: true},
		{name: "unknown option", args: []string{"--verbose"}, fail: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseStartArgs(tt.args)
			if tt.fail {
				if err == nil {
					t.Fatalf("parseStartArgs(%v) expected error", tt.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseStartArgs(%v): %v", tt.args, err)
			}
			if got != tt.want {
				t.Errorf("parseStartArgs(%v) = %q, want %q", tt.args, got, tt.want)
			}
		})
	}
}

func TestInstallDirUsesRequestedWindowsPath(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows path assertion")
	}
	got, err := installDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Spotigent")
	if !strings.EqualFold(got, want) {
		t.Fatalf("installDir() = %q, want %q", got, want)
	}
}

func TestRunHelpAndVersion(t *testing.T) {
	for _, command := range [][]string{{"help"}, {"version"}} {
		if err := run(command); err != nil {
			t.Errorf("run(%v): %v", command, err)
		}
	}
}
