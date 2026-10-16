package main

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/cristianargotti/golaw/internal/buildinfo"
)

// errBoom is the failure every fake seam of these tests raises.
var errBoom = errors.New("boom")

// testApp is an app whose every seam is a fake; a test overrides the ones
// it exercises and reads stdout and stderr back.
type testApp struct {
	*app

	stdout, stderr *bytes.Buffer
}

func newTestApp(t *testing.T) testApp {
	t.Helper()
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	a := &app{
		stdout: stdout,
		stderr: stderr,
		buildInfo: func() buildinfo.Info {
			return buildinfo.Info{Version: "9.9.9", Module: "example.test/kit", Stamped: true}
		},
		now:      func() time.Time { return time.Date(2026, time.October, 14, 12, 0, 0, 0, time.UTC) },
		home:     func() (string, error) { return t.TempDir(), nil },
		env:      func(string) string { return "" },
		stdin:    strings.NewReader(""),
		discover: func(_ context.Context, dir string) (string, error) { return dir, nil },
	}
	return testApp{app: a, stdout: stdout, stderr: stderr}
}

func TestExitCodeMapsNilAndExitErrorAndFailure(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	if got := ta.exitCode(nil); got != exitClean {
		t.Fatalf("nil = %d", got)
	}
	if got := ta.exitCode(exitError{code: exitFindings}); got != exitFindings {
		t.Fatalf("exit error = %d", got)
	}
	if got := ta.exitCode(errBoom); got != exitFailure {
		t.Fatalf("failure = %d", got)
	}
	if got := ta.stderr.String(); got != "golaw: boom\n" {
		t.Fatalf("stderr = %q", got)
	}
	if got := (exitError{code: 1}).Error(); got != "exit 1" {
		t.Fatalf("exit error text = %q", got)
	}
}

func TestRunRefusesAnUnknownCommandWithExitTwo(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	if got := ta.run(t.Context(), []string{"nope"}); got != exitFailure {
		t.Fatalf("exit = %d", got)
	}
	if !strings.Contains(ta.stderr.String(), `unknown command "nope"`) {
		t.Fatalf("stderr = %q", ta.stderr.String())
	}
}

func TestRunPrintsHelpOnStdout(t *testing.T) {
	t.Parallel()
	ta := newTestApp(t)
	if got := ta.run(t.Context(), []string{"--help"}); got != exitClean {
		t.Fatalf("exit = %d", got)
	}
	if !strings.Contains(ta.stdout.String(), "version") || ta.stderr.Len() != 0 {
		t.Fatalf("stdout = %q, stderr = %q", ta.stdout.String(), ta.stderr.String())
	}
}

// TestNewAppBindsEverySeam walks the fields of the app, so a seam added
// later and left unbound in newApp fails here before a command calls it.
func TestNewAppBindsEverySeam(t *testing.T) {
	t.Parallel()
	a := newApp(&bytes.Buffer{}, &bytes.Buffer{})
	fields := reflect.ValueOf(a).Elem()
	for i := range fields.NumField() {
		field, name := fields.Field(i), fields.Type().Field(i).Name
		if kind := field.Kind(); kind != reflect.Func && kind != reflect.Interface {
			t.Errorf("the field %s is not a seam", name)
			continue
		}
		if field.IsNil() {
			t.Errorf("the seam %s is not bound", name)
		}
	}
	if got := a.buildInfo(); !got.Stamped {
		t.Fatalf("build info = %+v", got)
	}
}

// TestNewAppExplainsThePolicyOfARepository runs config explain through the
// production composition, over a home the test owns, which is why it sets
// the environment and stays serial.
func TestNewAppExplainsThePolicyOfARepository(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_STATE_HOME", filepath.Join(home, "state"))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "config"))
	root := repository(t, true)
	var stdout, stderr bytes.Buffer
	if got := newApp(&stdout, &stderr).run(t.Context(), []string{"config", "explain", root}); got != exitClean {
		t.Fatalf("exit = %d, stderr = %q", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), "kit.version") || !strings.Contains(stdout.String(), testVersion) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
