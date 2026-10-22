package doctor

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// hooksListFixture is the file beside the test double that holds the
// result its hooks/list answers; lingerFixture, when present, makes the
// double start a child of its own first and write the child's pid beside
// it, as a server that starts helpers would.
const (
	hooksListFixture = "hooks-list.json"
	lingerFixture    = "linger"
)

// serveHooksList is the test binary standing in for codex app-server: it
// answers initialize, sends a notification, and answers hooks/list with
// the result held beside the executable, or an error when there is none.
func serveHooksList() {
	dir := filepath.Dir(os.Args[0])
	if _, statErr := os.Stat(filepath.Join(dir, lingerFixture)); statErr == nil {
		child := exec.Command(os.Args[0], lingerFixture)
		if child.Start() != nil || os.WriteFile(filepath.Join(dir, lingerFixture+".pid"), []byte(strconv.Itoa(child.Process.Pid)), 0o600) != nil {
			return
		}
	}
	fixture, err := os.ReadFile(filepath.Join(dir, hooksListFixture))
	lines := bufio.NewScanner(os.Stdin)
	for lines.Scan() {
		var request rpcRequest
		if json.Unmarshal(lines.Bytes(), &request) != nil {
			return
		}
		var writeErr error
		switch {
		case request.Method == "initialize":
			_, writeErr = fmt.Println(`{"id":1,"result":{"userAgent":"double"}}`)
		case request.Method == "hooks/list" && err != nil:
			_, writeErr = fmt.Println(`{"id":2,"error":{"code":-32603,"message":"no fixture"}}`)
		case request.Method == "hooks/list":
			_, writeErr = fmt.Printf("{\"method\":\"configWarning\",\"params\":{}}\n{\"id\":2,\"result\":%s}\n", bytes.TrimSpace(fixture))
		}
		if writeErr != nil {
			return
		}
	}
}

// linger is the child the stand-in server starts: it outlives the
// conversation unless the process group of the server is ended.
func linger() {
	time.Sleep(time.Minute)
}

// listResult is the result of hooks/list for one directory.
func listResult(hooks string) string {
	return `{"data":[{"cwd":"/repo","hooks":[` + hooks + `],"warnings":[],"errors":[]}]}`
}

func TestExchangeHoldsTheConversation(t *testing.T) {
	t.Parallel()
	var sent bytes.Buffer
	answers := strings.Join([]string{
		`{"id":1,"result":{}}`,
		`{"method":"thread/started","params":{}}`,
		`{"id":2,"result":` + listResult(`{"key":"k","sourcePath":"p","currentHash":"h","trustStatus":"trusted","enabled":true}`) + `}`,
	}, "\n")
	hooks, err := exchange(&sent, strings.NewReader(answers), "/repo", "v1")
	if err != nil || len(hooks) != 1 || hooks[0] != (listedHook{Key: "k", SourcePath: "p", CurrentHash: "h", TrustStatus: statusTrusted, Enabled: true}) {
		t.Fatalf("hooks = %+v, %v", hooks, err)
	}
	want := `{"id":1,"method":"initialize","params":{"clientInfo":{"name":"golaw","title":null,"version":"v1"}}}` + "\n" +
		`{"method":"initialized"}` + "\n" + `{"id":2,"method":"hooks/list","params":{"cwds":["/repo"]}}` + "\n"
	if sent.String() != want {
		t.Fatalf("sent %q", sent.String())
	}
}

func TestExchangeRefusesAnAnswerItCannotUse(t *testing.T) {
	t.Parallel()
	initialized := `{"id":1,"result":{}}` + "\n"
	for name, answers := range map[string]string{
		"refused initialize": `{"id":1,"error":{"message":"unsupported client"}}`,
		"not JSON":           "codex app-server starting",
		"closed":             initialized,
		"config errors":      initialized + `{"id":2,"result":{"data":[{"cwd":"/repo","hooks":[],"errors":[{"path":"config.toml","message":"bad key"}]}]}}`,
		"two directories":    initialized + `{"id":2,"result":{"data":[{"hooks":[]},{"hooks":[]}]}}`,
		"result not a list":  initialized + `{"id":2,"result":{"data":"none"}}`,
	} {
		if _, err := exchange(io.Discard, strings.NewReader(answers), "/repo", "v1"); !errors.Is(err, errAppServer) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := exchange(failingWriterOf(0), strings.NewReader(""), "/repo", "v1"); err == nil {
		t.Fatal("a failed send was ignored")
	}
	if _, err := exchange(failingWriterOf(1), strings.NewReader(initialized), "/repo", "v1"); err == nil {
		t.Fatal("a failed notification was ignored")
	}
	if _, err := exchange(failingWriterOf(2), strings.NewReader(initialized), "/repo", "v1"); err == nil {
		t.Fatal("a failed hooks/list was ignored")
	}
	long := initialized + `{"id":2,"result":"` + strings.Repeat("x", appServerLine) + `"}`
	if _, err := exchange(io.Discard, strings.NewReader(long), "/repo", "v1"); !errors.Is(err, errAppServer) {
		t.Fatalf("a line over the bound = %v", err)
	}
}

// failingWriterOf accepts a number of writes and fails the next one.
func failingWriterOf(left int) io.Writer { return &failingWriter{left: left} }

func TestAppServerListsTheHooksThroughTheObservedPath(t *testing.T) {
	t.Parallel()
	dir := doubleNamed(t, agentCodex)
	environment := AgentEnvironment{Source: sourceAgent, Path: dir, PathExt: ".EXE"}
	cwd := t.TempDir()
	if _, err := (appServer{version: "v1"}).ListHooks(t.Context(), environment, cwd); err == nil || !strings.Contains(err.Error(), "no fixture") {
		t.Fatalf("no fixture = %v", err)
	}
	writeFile(t, filepath.Join(dir, hooksListFixture), listResult(`{"key":"k","sourcePath":"p","trustStatus":"modified","enabled":true}`))
	hooks, err := (appServer{version: "v1"}).ListHooks(t.Context(), environment, cwd)
	if err != nil || len(hooks) != 1 || hooks[0].TrustStatus != statusModified {
		t.Fatalf("hooks = %+v, %v", hooks, err)
	}
	if _, err := (appServer{}).ListHooks(t.Context(), AgentEnvironment{}, cwd); !errors.Is(err, ErrEnvironment) {
		t.Fatalf("no observed environment = %v", err)
	}
	if _, err := (appServer{}).ListHooks(t.Context(), environment, filepath.Join(cwd, "absent")); err == nil {
		t.Fatal("a directory that does not exist started the server")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (appServer{}).ListHooks(ctx, environment, cwd); err == nil {
		t.Fatal("a canceled conversation answered")
	}
}

// TestAppServerLeavesNoProcessBehind runs a stand-in whose server starts a
// child of its own: once the conversation ends, the whole process group of
// the server is gone, so nothing the server started outlives doctor (added
// by the review of the plan, not replayed).
func TestAppServerLeavesNoProcessBehind(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("a process group is a POSIX notion: on Windows the children of codex app-server are a stated limit")
	}
	dir := doubleNamed(t, agentCodex)
	writeFile(t, filepath.Join(dir, hooksListFixture), listResult(""))
	writeFile(t, filepath.Join(dir, lingerFixture), "")
	environment := AgentEnvironment{Source: sourceAgent, Path: dir, PathExt: ".EXE"}
	if _, err := (appServer{version: "v1"}).ListHooks(t.Context(), environment, t.TempDir()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, lingerFixture+".pid"))
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(string(data))
	if err != nil {
		t.Fatal(err)
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for process.Signal(syscall.Signal(0)) == nil {
		if time.Now().After(deadline) {
			t.Fatalf("the child %d of the server outlived the conversation", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// TestEndGroupOfAServerThatNeverStarted holds the guard the cancel of the
// conversation relies on: a command without a process has nothing to end.
func TestEndGroupOfAServerThatNeverStarted(t *testing.T) {
	t.Parallel()
	if err := endGroup(&exec.Cmd{}); err != nil {
		t.Fatalf("a server that never started = %v", err)
	}
}
