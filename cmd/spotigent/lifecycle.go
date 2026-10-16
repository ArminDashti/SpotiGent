package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Exit codes reported by `spotigent status`.
const (
	exitRunning      = 0
	exitStopped      = 3
	exitNotInstalled = 4
)

// exitCodeError makes run() return a specific process exit code without extra output.
type exitCodeError struct{ code int }

func (e exitCodeError) Error() string { return fmt.Sprintf("exit code %d", e.code) }

// appConfig is the per-install configuration stored in config.json.
type appConfig struct {
	Port string `json:"port"`
}

func configPath() (string, error) {
	dir, err := installDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

func loadConfig() (appConfig, error) {
	path, err := configPath()
	if err != nil {
		return appConfig{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return appConfig{}, nil
	}
	if err != nil {
		return appConfig{}, err
	}
	var cfg appConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return appConfig{}, fmt.Errorf("invalid config.json: %w", err)
	}
	return cfg, nil
}

func saveConfig(cfg appConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// savedPort returns the default port saved by `spotigent port`, or the built-in default.
func savedPort() string {
	cfg, err := loadConfig()
	if err != nil || cfg.Port == "" {
		return defaultPort
	}
	return cfg.Port
}

func installedExecutable() (string, error) {
	dir, err := installDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, executableName()), nil
}

// runningPID returns the PID from the PID file when that process is alive.
func runningPID() (int, bool) {
	pidFile, err := pidPath()
	if err != nil {
		return 0, false
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || !processAlive(pid) {
		return 0, false
	}
	return pid, true
}

func installedVersion(exe string) string {
	out, err := exec.Command(exe, "version").Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

func urlReachable(url string) bool {
	client := http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return true
}

// showStatus prints the install state and returns an exitCodeError for scripts:
// 0 running, 3 stopped, 4 not installed.
func showStatus() error {
	exe, err := installedExecutable()
	if err != nil {
		return err
	}
	if _, err := os.Stat(exe); err != nil {
		fmt.Println("SpotiGent is not installed")
		return exitCodeError{exitNotInstalled}
	}
	port := savedPort()
	fmt.Printf("install:  %s\n", filepath.Dir(exe))
	fmt.Printf("version:  %s\n", installedVersion(exe))
	fmt.Printf("port:     %s\n", port)

	pid, running := runningPID()
	if !running {
		fmt.Println("status:   stopped")
		return exitCodeError{exitStopped}
	}
	fmt.Printf("status:   running (PID %d)\n", pid)
	url := "http://127.0.0.1:" + port
	if urlReachable(url) {
		fmt.Printf("url:      %s (reachable)\n", url)
	} else {
		fmt.Printf("url:      %s (not responding)\n", url)
	}
	return nil
}

// handlePort prints the saved default port, or saves a new one and restarts the server if it is running.
func handlePort(args []string) error {
	if len(args) == 0 {
		fmt.Printf("port: %s\n", savedPort())
		return nil
	}
	current := savedPort()
	port, err := parseStartArgs(args, current)
	if err != nil {
		return err
	}
	if port != current && portBusy(port) {
		return fmt.Errorf("port %s is already in use", port)
	}
	_, running := runningPID()
	if running {
		if err := stopServer(); err != nil {
			return err
		}
	}
	if err := saveConfig(appConfig{Port: port}); err != nil {
		return err
	}
	fmt.Printf("default port set to %s\n", port)
	if running {
		return startServer(port)
	}
	return nil
}
