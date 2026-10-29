package toolchain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/cristianargotti/golaw/internal/runner"
)

// State is how a pinned tool resolved on this machine.
type State string

const (
	// StateVerified is a tool of the tool directory whose whole file hashed
	// to its receipt's sha256: what lint and doctor trust.
	StateVerified State = "verified"
	// StateReceipt is the hook's cheap check: the file's size and time equal
	// the receipt's, and no process was started (M6.5.2).
	StateReceipt State = "receipt"
	// StatePath is a tool on PATH whose probe names the pin exactly; its
	// checksum is not verified (spec 4.2), which the detail says.
	StatePath State = "path"
	// StateMissing is a tool neither the tool directory nor PATH holds at
	// the pin: a notice, never a denial (M6.5.3).
	StateMissing State = "missing"
	// StateDrift is a file of the tool directory that no longer matches its
	// receipt, shown instead of bypassed so a changed binary never runs.
	StateDrift State = "drift"
	// StateRuntime is an npm or PyPI tool whose Node or Python is absent or
	// outside its bounds, which the kit never installs.
	StateRuntime State = "runtime-missing"
)

// probeTimeout bounds a version probe the caller gave no runner for; a
// probe answers in well under a second, and the bound is far above that so
// a loaded machine never reads a slow start as a missing tool.
const probeTimeout = 30 * time.Second

// fix is the one action that turns every unusable state into a usable one.
const fix = "run golaw setup --surface"

// Resolved is one tool as the resolver found it.
type Resolved struct {
	Tool   Tool
	Path   string
	State  State
	Detail string // English, names the path, the pin and the fix
}

// Usable reports whether the tool may be started.
func (r Resolved) Usable() bool {
	return r.State == StateVerified || r.State == StateReceipt || r.State == StatePath
}

// Notice answers the missing tool notice of the resolved tool, so the hook
// and the rung read the notice of the tool they resolved.
func (r Resolved) Notice(path, language string) (string, string) {
	return MissingNotice(r.Tool, path, language)
}

// Resolver finds pinned tools in one tool directory and on one PATH.
type Resolver struct {
	Dir    string   // policy.Dirs.Tools, empty when none resolves here
	GOOS   string   // the target the layout names
	GOARCH string   // the target the layout names
	Env    []string // the PATH a lookup and a probe use; never the process's own
	// Run starts a probe; nil runs it through Exec under probeTimeout.
	Run func(context.Context, runner.Command) (runner.Output, error)
}

// Hook resolves from the receipt alone: no process and no PATH lookup, so
// the hook stays inside its budget (M6.5.2).
func (r Resolver) Hook(t Tool) Resolved {
	rec, miss, ok := r.receipt(t)
	if !ok {
		return miss
	}
	info, err := os.Stat(rec.Path)
	if err != nil {
		return gone(t, rec.Path)
	}
	if info.Size() != rec.Size || !info.ModTime().Equal(rec.ModTime) {
		return drift(t, rec.Path)
	}
	return Resolved{Tool: t, Path: rec.Path, State: StateReceipt, Detail: fmt.Sprintf("%s %s at %s (receipt matches)", t.Name, t.Version, rec.Path)}
}

// Full rehashes the receipt's file, then falls back to PATH at the pin:
// what lint and doctor use, where a process start is affordable.
func (r Resolver) Full(ctx context.Context, t Tool) Resolved {
	if rt, ok := runtimeOf(t); ok {
		if status := r.toolRuntime(ctx, t, rt); !status.OK {
			return Resolved{Tool: t, State: StateRuntime, Detail: status.Detail}
		}
	}
	if rec, ok := r.verified(t); ok {
		return Resolved{Tool: t, Path: rec.Path, State: StateVerified, Detail: fmt.Sprintf("%s %s at %s (sha256 verified)", t.Name, t.Version, rec.Path)}
	}
	rec, miss, ok := r.receipt(t)
	if !ok {
		return r.onPath(ctx, t, miss)
	}
	if !r.venvLive(t) {
		return venvGone(t, VenvPython(r.Dir, r.GOOS))
	}
	// A receipt that accepts the tool never falls back to PATH: a changed
	// binary in the tool directory is shown, not bypassed.
	if _, err := os.Stat(rec.Path); err != nil {
		return gone(t, rec.Path)
	}
	return drift(t, rec.Path)
}

// receipt answers the receipt that installs this pin at the layout path,
// or the missing answer that names why none does.
func (r Resolver) receipt(t Tool) (Receipt, Resolved, bool) {
	if r.Dir == "" {
		return Receipt{}, missing(t, fmt.Sprintf("%s %s: no tools directory resolves on this machine (set XDG_DATA_HOME or HOME): %s", t.Name, t.Version, fix)), false
	}
	rec, found, err := ReadReceipt(r.Dir, t.Name)
	switch {
	case err != nil:
		// An ErrReceipt or an unreadable receipt names its file; either way
		// an unknown binary is never run.
		return Receipt{}, missing(t, fmt.Sprintf("%s %s is not installed: %v; %s", t.Name, t.Version, err, fix)), false
	case !found:
		return Receipt{}, missing(t, fmt.Sprintf("%s %s is not installed: %s", t.Name, t.Version, fix)), false
	case rec.Version != t.Version:
		return Receipt{}, missing(t, fmt.Sprintf("%s %s is not installed: the receipt records %s %s; %s", t.Name, t.Version, rec.Name, rec.Version, fix)), false
	}
	if want := ExecPath(r.Dir, t, r.GOOS, r.GOARCH); rec.Path != want {
		return Receipt{}, missing(t, fmt.Sprintf("%s %s is not installed: the receipt records %s, the layout names %s; %s", t.Name, t.Version, rec.Path, want, fix)), false
	}
	return rec, Resolved{}, true
}

// verified answers the receipt of the pin when the file's whole sha256
// still equals the one it records and its interpreter still resolves.
func (r Resolver) verified(t Tool) (Receipt, bool) {
	rec, _, ok := r.receipt(t)
	if !ok {
		return Receipt{}, false
	}
	sum, err := fileSHA256(rec.Path)
	return rec, err == nil && sum == rec.SHA256 && r.venvLive(t)
}

// onPath accepts a tool on PATH only when its probe names the pin exactly;
// why is the answer when PATH holds none.
func (r Resolver) onPath(ctx context.Context, t Tool, why Resolved) Resolved {
	name := t.Bin
	if name == "" {
		name = t.Name
	}
	path, found := lookPath(name, r.Env, r.GOOS)
	if !found {
		return why
	}
	out, err := r.run(ctx, runner.Command{Name: path, Args: t.Probe, Env: ToolEnv(r.Env)})
	if err == nil && out.Code == 0 && (runner.Tool{Name: t.Name, Version: t.Version}).Matches(string(out.Stdout)) {
		return Resolved{Tool: t, Path: path, State: StatePath, Detail: fmt.Sprintf("%s %s at %s (on PATH, version matches, checksum not verified)", t.Name, t.Version, path)}
	}
	return missing(t, fmt.Sprintf("%s %s is not installed: PATH has %s answering %q; %s", t.Name, t.Version, path, firstLine(out), fix))
}

// run starts a probe through r.Run, or else through Exec under
// probeTimeout, so a tool on PATH that never answers its version cannot
// hold lint or doctor.
func (r Resolver) run(ctx context.Context, c runner.Command) (runner.Output, error) {
	if r.Run != nil {
		return r.Run(ctx, c)
	}
	got, err := Exec(ctx, Request{Path: c.Name, Args: c.Args, Dir: c.Dir, Env: c.Env, Timeout: probeTimeout})
	if err == nil && got.Killed {
		return got.Output, fmt.Errorf("%w: %s within %v", ErrProbe, c.Name, probeTimeout)
	}
	return got.Output, err
}

func missing(t Tool, detail string) Resolved {
	return Resolved{Tool: t, State: StateMissing, Detail: detail}
}

func gone(t Tool, path string) Resolved {
	return missing(t, fmt.Sprintf("%s %s at %s is gone: %s", t.Name, t.Version, path, fix))
}

func drift(t Tool, path string) Resolved {
	return Resolved{Tool: t, Path: path, State: StateDrift, Detail: fmt.Sprintf("%s %s at %s no longer matches its receipt: %s", t.Name, t.Version, path, fix)}
}

// fileSHA256 answers the lower hex sha256 of a file.
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // the executable the kit's own receipt names
	if err != nil {
		return "", fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

// firstLine is what a refusal quotes: stderr stands in for an empty stdout,
// and one line keeps the reason short.
func firstLine(out runner.Output) string {
	text := strings.TrimSpace(string(out.Stdout))
	if text == "" {
		text = strings.TrimSpace(string(out.Stderr))
	}
	line, _, _ := strings.Cut(text, "\n")
	return line
}
