//go:build windows

package install

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"testing"
	"time"
)

const cinStandIn = `package main

import (
	"io"
	"os"
	"strconv"
)

func main() {
	if len(os.Args) != 4 || os.Args[1] != "hook" || os.Args[2] != "codex" {
		os.Exit(3)
	}
	status, err := strconv.Atoi(os.Args[3])
	if err != nil {
		os.Exit(4)
	}
	if _, err = io.Copy(os.Stdout, os.Stdin); err != nil {
		os.Exit(5)
	}
	if _, err = io.WriteString(os.Stderr, "launcher-stderr-á\n"); err != nil {
		os.Exit(6)
	}
	os.Exit(status)
}
`

func TestWindowsLauncherStreamsAndStatus(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	binary := cinBuildStandIn(t, home)
	payload := []byte("{\r\n  \"text\": \"á $ & quote'\", \"path\": \"C:\\\\kit\\\\file\"\r\n}")
	for _, row := range []struct {
		name, shell  string
		status, want int
	}{
		{"cmd-allow-keeps-0", "cmd.exe", 0, 0},
		{"cmd-post-cannot-judge-keeps-2", "cmd.exe", 2, 2},
		{"powershell-command-allow-keeps-0", "powershell.exe", 0, 0},
		{"powershell-command-post-cannot-judge-becomes-1", "powershell.exe", 2, 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			command := HookCommand(binary, "windows", "hook", "codex", strconv.Itoa(row.status))
			run := cinWindowsShell(ctx, row.shell, command)
			run.Env = append(os.Environ(), cinEnvironment(home)...)
			run.Stdin = bytes.NewReader(payload)
			var stdout, stderr bytes.Buffer
			run.Stdout, run.Stderr = &stdout, &stderr
			err := run.Run()
			var exitErr *exec.ExitError
			if err != nil && !errors.As(err, &exitErr) {
				t.Fatalf("launch: %v", err)
			}
			if run.ProcessState.ExitCode() != row.want {
				t.Fatalf("exit = %d, want %d; stderr = %q",
					run.ProcessState.ExitCode(), row.want, stderr.Bytes())
			}
			if !bytes.Equal(stdout.Bytes(), payload) {
				t.Fatalf("stdin/stdout changed: got %q, want %q", stdout.Bytes(), payload)
			}
			if stderr.String() != "launcher-stderr-á\n" {
				t.Fatalf("stderr changed: %q", stderr.Bytes())
			}
		})
	}
}

func cinWindowsShell(ctx context.Context, shell, command string) *exec.Cmd {
	if shell == "cmd.exe" {
		run := exec.CommandContext(ctx, shell)
		run.SysProcAttr = &syscall.SysProcAttr{
			CmdLine: `cmd.exe /c "` + command + `"`,
		}
		return run
	}
	return exec.CommandContext(ctx, shell, "-NoProfile", "-Command", command)
}

func cinBuildStandIn(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, "á space ' $ &")
	must(t, os.MkdirAll(dir, 0o700))
	source := filepath.Join(dir, "main.go")
	cinWrite(t, source, []byte(cinStandIn))
	binary := filepath.Join(dir, "stand-in.exe")
	build := exec.CommandContext(t.Context(), "go", "build", "-o", binary, source)
	build.Env = append(os.Environ(), cinEnvironment(home)...)
	build.Env = append(build.Env,
		"GOTOOLCHAIN=local",
		"GOPROXY=off",
		"GOSUMDB=off",
		"GOWORK=off",
		"GOENV=off",
		"GOFLAGS=",
		"GOCACHE="+filepath.Join(home, "go-cache"),
		"GOPATH="+filepath.Join(home, "go-path"),
	)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build stand-in: %v\n%s", err, output)
	}
	return binary
}
