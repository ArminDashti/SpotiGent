// SpotiGent — manage your Spotify account with AI.
package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"spotigent/internal/config"
	"spotigent/internal/logs"
	"spotigent/internal/server"
	"spotigent/internal/store"
)

// version is stamped at release time, e.g. -ldflags "-X main.version=1.1.0".
var version = "dev"

const defaultPort = "9090"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "_serve" {
		port, err := parseStartArgs(os.Args[2:])
		if err == nil {
			var pidFile string
			pidFile, err = pidPath()
			if err == nil {
				err = serve(port, pidFile)
			}
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "SpotiGent:", err)
			os.Exit(1)
		}
		return
	}
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "SpotiGent:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printHelp()
		return nil
	}
	switch args[0] {
	case "help", "-h", "--help":
		printHelp()
		return nil
	case "version", "--version":
		fmt.Println(version)
		return nil
	case "start":
		port, err := parseStartArgs(args[1:])
		if err != nil {
			return err
		}
		return startServer(port)
	case "stop":
		return stopServer()
	case "remove":
		return removeInstallation()
	case "update":
		return updateInstallation()
	default:
		return fmt.Errorf("unknown command %q; run 'spotigent help'", args[0])
	}
}

func printHelp() {
	fmt.Printf(`SpotiGent %s

Usage:
  spotigent start [--port=<port>]
  spotigent stop
  spotigent update
  spotigent remove
  spotigent version
  spotigent help

The server listens on 127.0.0.1:%s by default.
`, version, defaultPort)
}

func parseStartArgs(args []string) (string, error) {
	port := defaultPort
	for _, arg := range args {
		if strings.HasPrefix(arg, "--port=") {
			port = strings.TrimPrefix(arg, "--port=")
		} else {
			return "", fmt.Errorf("unknown start option %q", arg)
		}
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return "", fmt.Errorf("invalid port %q: expected 1–65535", port)
	}
	return port, nil
}

func dataDir() (string, error) {
	if value := os.Getenv("SPOTIGENT_DATA_DIR"); value != "" {
		return value, nil
	}
	dir, err := installDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "data"), nil
}

func installDir() (string, error) {
	if runtime.GOOS == "windows" {
		base := os.Getenv("USERPROFILE")
		if base == "" {
			return "", errors.New("USERPROFILE is not set")
		}
		return filepath.Join(base, "AppData", "Spotigent"), nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Spotigent"), nil
}

func executableName() string {
	if runtime.GOOS == "windows" {
		return "spotigent.exe"
	}
	return "spotigent"
}

func pidPath() (string, error) {
	dir, err := installDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "spotigent.pid"), nil
}

func startServer(port string) error {
	pidFile, err := pidPath()
	if err != nil {
		return err
	}
	if data, err := os.ReadFile(pidFile); err == nil {
		if pid, parseErr := strconv.Atoi(strings.TrimSpace(string(data))); parseErr == nil && processAlive(pid) {
			return fmt.Errorf("already running (PID %d)", pid)
		}
		_ = os.Remove(pidFile)
	}
	if portBusy(port) {
		return fmt.Errorf("port %s is already in use", port)
	}
	if runtime.GOOS == "windows" {
		return startWindows(port, pidFile)
	}
	return runServer(port, pidFile)
}

func portBusy(port string) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", port), 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

func startWindows(port, pidFile string) error {
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(binary, "_serve", "--port="+port)
	cmd.Dir, err = installDir()
	if err != nil {
		return err
	}
	cmd.Stdout, cmd.Stderr = nil, nil
	if err := cmd.Start(); err != nil {
		return err
	}
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0o600); err != nil {
		_ = cmd.Process.Kill()
		return err
	}
	fmt.Printf("SpotiGent started (PID %d) at http://127.0.0.1:%s\n", cmd.Process.Pid, port)
	return nil
}

func runServer(port, pidFile string) error {
	return serve(port, pidFile)
}

func serve(port, pidFile string) error {
	data, err := dataDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(data, 0o700); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o600); err != nil {
			return err
		}
		defer os.Remove(pidFile)
	}

	cfg := config.Load()
	cfg.Port = port
	if cfg.SpotifyClientID != "" {
		logs.LogInfo("SPOTIFY_CLIENT_ID detected in environment — Settings boxes prefilled")
	}
	if cfg.SpotifyClientSecret != "" {
		logs.LogInfo("SPOTIFY_CLIENT_SECRET detected in environment — Settings boxes prefilled")
	}
	st, err := store.New(filepath.Join(data, "settings.json"))
	if err != nil {
		return fmt.Errorf("settings store: %w", err)
	}
	chats, err := store.NewChatStore(filepath.Join(data, "chats.json"))
	if err != nil {
		return fmt.Errorf("chat store: %w", err)
	}
	srv := server.New(cfg, st, chats)
	addr := cfg.Addr()
	logs.LogInfo("SpotiGent %s starting on http://%s", version, addr)
	fmt.Printf("SpotiGent %s listening on http://%s\n", version, addr)
	if runtime.GOOS == "windows" {
		return srv.Router().Run(addr)
	}
	httpServer := &http.Server{Addr: addr, Handler: srv.Router()}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdown)
	}()
	return httpServer.ListenAndServe()
}

func stopServer() error {
	pidFile, err := pidPath()
	if err != nil {
		return err
	}
	data, err := os.ReadFile(pidFile)
	if errors.Is(err, os.ErrNotExist) {
		fmt.Println("SpotiGent is not running")
		return nil
	}
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		_ = os.Remove(pidFile)
		return errors.New("invalid PID file; removed it")
	}
	if !processAlive(pid) {
		_ = os.Remove(pidFile)
		fmt.Println("SpotiGent was not running; cleaned stale PID file")
		return nil
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("stop server: %s: %w", strings.TrimSpace(string(output)), err)
		}
	} else {
		proc, err := os.FindProcess(pid)
		if err != nil {
			return err
		}
		if err := proc.Signal(syscall.SIGTERM); err != nil {
			return err
		}
	}
	_ = os.Remove(pidFile)
	fmt.Println("SpotiGent stopped")
	return nil
}

func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if runtime.GOOS == "windows" {
		out, err := exec.Command("tasklist", "/FI", "PID eq "+strconv.Itoa(pid), "/FO", "CSV", "/NH").Output()
		return err == nil && strings.Contains(string(out), strconv.Quote(strconv.Itoa(pid)))
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

func removeInstallation() error {
	if err := stopServer(); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		return errors.New("remove is supported by the Windows installer only")
	}
	ps := `$ErrorActionPreference='Stop'; $base=Join-Path $env:USERPROFILE 'AppData\Spotigent'; $path=[Environment]::GetEnvironmentVariable('Path','User'); $parts=$path -split ';' | Where-Object { $_ -and $_.TrimEnd('\') -ine $base.TrimEnd('\') }; [Environment]::SetEnvironmentVariable('Path',($parts -join ';'),'User'); [Environment]::SetEnvironmentVariable('SPOTIGENT_DATA_DIR',$null,'User'); [Environment]::SetEnvironmentVariable('SPOTIGENT_SOURCE_DIR',$null,'User'); Start-Sleep -Milliseconds 500; Remove-Item -LiteralPath $base -Recurse -Force`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", ps)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("remove installation: %s: %w", strings.TrimSpace(string(output)), err)
	}
	fmt.Println("SpotiGent removed (user PATH cleaned); open a new terminal to refresh PATH")
	return nil
}

func updateInstallation() error {
	if runtime.GOOS != "windows" {
		return errors.New("update is supported by the Windows installer only")
	}
	binary, err := os.Executable()
	if err != nil {
		return err
	}
	install, err := installDir()
	if err != nil {
		return err
	}
	current, err := filepath.Abs(binary)
	if err != nil {
		return err
	}
	installed, err := filepath.Abs(filepath.Join(install, executableName()))
	if err != nil {
		return err
	}
	if !strings.EqualFold(current, installed) {
		return fmt.Errorf("run update using the installed binary: %s", installed)
	}
	root := os.Getenv("SPOTIGENT_SOURCE_DIR")
	if root == "" {
		return errors.New("SPOTIGENT_SOURCE_DIR is not set; reinstall from the project checkout")
	}
	installer := filepath.Join(root, "scripts", "installer-win-x64.ps1")
	if _, err := os.Stat(installer); err != nil {
		return fmt.Errorf("cannot find installer script at %s; set SPOTIGENT_SOURCE_DIR to the project checkout or install a new release", installer)
	}
	cmd := exec.Command("powershell.exe", "-NoProfile", "-ExecutionPolicy", "Bypass", "-File", installer, "-Update")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	return cmd.Run()
}
