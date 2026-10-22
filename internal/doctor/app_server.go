package doctor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"
)

// The bounds of one conversation with codex app-server: its own deadline,
// below anything a person waits for, and the longest line it may answer.
const (
	appServerBudget = 5 * time.Second
	appServerWait   = time.Second
	appServerLine   = 1 << 20
)

// listedHook is one hook codex app-server lists for a directory, with the
// hash and the trust status Codex computed itself (survey section 6.3).
type listedHook struct {
	Key         string `json:"key"`
	SourcePath  string `json:"sourcePath"`
	CurrentHash string `json:"currentHash"`
	TrustStatus string `json:"trustStatus"`
	Enabled     bool   `json:"enabled"`
}

// hookLister asks Codex which hooks it would run for a directory.
type hookLister interface {
	ListHooks(ctx context.Context, environment AgentEnvironment, cwd string) ([]listedHook, error)
}

// appServer is the client of codex app-server over JSON lines on stdio:
// initialize, initialized, then hooks/list, never through a shell.
type appServer struct{ version string }

// rpcRequest is one message the client sends; a notification has no id.
type rpcRequest struct {
	ID     int    `json:"id,omitempty"`
	Method string `json:"method"`
	Params any    `json:"params,omitempty"`
}

// clientInfo names the client in initialize; Codex reads a null title.
type clientInfo struct {
	Name    string  `json:"name"`
	Title   *string `json:"title"`
	Version string  `json:"version"`
}

// rpcResponse is one message the server sends; a notification has no id.
type rpcResponse struct {
	ID     *int            `json:"id"`
	Result json.RawMessage `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// hooksListResult is the answer of hooks/list, one entry per directory.
type hooksListResult struct {
	Data []struct {
		Hooks  []listedHook `json:"hooks"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	} `json:"data"`
}

// ListHooks runs the codex the hook observed with app-server in cwd and
// asks for the hooks of cwd, within its own deadline.
func (s appServer) ListHooks(ctx context.Context, environment AgentEnvironment, cwd string) ([]listedHook, error) {
	path, err := ResolveAgentBinary(agentCodex, environment)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, appServerBudget)
	defer cancel()
	//nolint:gosec // The path was resolved through the agent's observed search path.
	command := exec.CommandContext(ctx, path, "app-server")
	command.Dir, command.WaitDelay = cwd, appServerWait
	detach(command)
	command.Cancel = func() error { return endGroup(command) }
	stdin, err := command.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("open the stdin of codex app-server: %w", err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("open the stdout of codex app-server: %w", err)
	}
	if startErr := command.Start(); startErr != nil {
		return nil, fmt.Errorf("start codex app-server: %w", startErr)
	}
	hooks, err := exchange(stdin, stdout, cwd, s.version)
	_ = stdin.Close()
	_ = command.Wait()    //nolint:errcheck // the answer is read; how the server leaves changes nothing
	_ = endGroup(command) //nolint:errcheck // what the server started ends with it
	return hooks, err
}

// exchange holds the conversation: initialize and its answer, the
// initialized notification, then hooks/list and its answer.
func exchange(in io.Writer, out io.Reader, cwd, version string) ([]listedHook, error) {
	lines := bufio.NewScanner(out)
	lines.Buffer(make([]byte, 0, bufio.MaxScanTokenSize), appServerLine)
	info := map[string]any{"clientInfo": clientInfo{Name: kitName, Version: version}}
	if err := send(in, rpcRequest{ID: 1, Method: "initialize", Params: info}); err != nil {
		return nil, err
	}
	if _, err := await(lines, 1); err != nil {
		return nil, err
	}
	if err := send(in, rpcRequest{Method: "initialized"}); err != nil {
		return nil, err
	}
	if err := send(in, rpcRequest{ID: 2, Method: "hooks/list", Params: map[string]any{"cwds": []string{cwd}}}); err != nil {
		return nil, err
	}
	result, err := await(lines, 2)
	if err != nil {
		return nil, err
	}
	return listedHooks(result)
}

// send writes one message as one line.
func send(in io.Writer, request rpcRequest) error {
	data, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("encode %s: %w", request.Method, err)
	}
	if _, err := in.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("send %s to codex app-server: %w", request.Method, err)
	}
	return nil
}

// await reads lines until the answer to id, skipping notifications.
func await(lines *bufio.Scanner, id int) (json.RawMessage, error) {
	for lines.Scan() {
		var response rpcResponse
		if err := json.Unmarshal(lines.Bytes(), &response); err != nil {
			return nil, fmt.Errorf("%w: a line is not JSON: %w", errAppServer, err)
		}
		switch {
		case response.ID == nil || *response.ID != id:
			continue
		case response.Error != nil:
			return nil, fmt.Errorf("%w: request %d: %s", errAppServer, id, response.Error.Message)
		default:
			return response.Result, nil
		}
	}
	if err := lines.Err(); err != nil {
		return nil, fmt.Errorf("%w: %w", errAppServer, err)
	}
	return nil, fmt.Errorf("%w: it closed before answering request %d", errAppServer, id)
}

// listedHooks are the hooks of the one directory asked for; an error Codex
// met loading its configuration makes the answer unusable.
func listedHooks(result json.RawMessage) ([]listedHook, error) {
	var listed hooksListResult
	if err := json.Unmarshal(result, &listed); err != nil {
		return nil, fmt.Errorf("%w: hooks/list: %w", errAppServer, err)
	}
	if len(listed.Data) != 1 {
		return nil, fmt.Errorf("%w: hooks/list answered %d directories", errAppServer, len(listed.Data))
	}
	if errs := listed.Data[0].Errors; len(errs) > 0 {
		return nil, fmt.Errorf("%w: hooks/list: %s", errAppServer, errs[0].Message)
	}
	return listed.Data[0].Hooks, nil
}
