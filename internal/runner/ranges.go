package runner

import (
	"context"
	"fmt"
	"strings"
)

type rangeError struct {
	reason string
}

func (e *rangeError) Error() string { return "cannot resolve push range: " + e.reason }

// PushRange keeps the advertised tip so a tag is scanned at its own object.
type PushRange struct {
	Update RefUpdate
	Base   string
	Tip    string
	Delete bool
}

type rangeGit struct {
	dir string
	env []string
}

// Ranges resolves every update without substituting the checked-out HEAD.
func Ranges(
	ctx context.Context, dir, remote string,
	updates []RefUpdate, env []string,
) ([]PushRange, error) {
	git := rangeGit{dir: dir, env: env}
	ranges := make([]PushRange, 0, len(updates))
	for i := range updates {
		resolved, err := git.resolve(ctx, remote, updates[i])
		if err != nil {
			return nil, fmt.Errorf("push %q to %q: %w", updates[i].LocalRef, updates[i].RemoteRef, err)
		}
		ranges = append(ranges, resolved)
	}
	return ranges, nil
}

func (g rangeGit) resolve(ctx context.Context, remote string, update RefUpdate) (PushRange, error) {
	result := PushRange{Update: update, Tip: update.LocalSHA}
	if err := validateUpdate(update); err != nil {
		return PushRange{}, err
	}
	if zeroObject(update.LocalSHA) {
		result.Delete = true
		return result, nil
	}
	tip, err := g.commit(ctx, update.LocalSHA)
	if err != nil {
		return PushRange{}, err
	}
	if tip == "" {
		return PushRange{}, &rangeError{reason: fmt.Sprintf("pushed object %q is not a locally available commit", update.LocalSHA)}
	}
	if zeroObject(update.RemoteSHA) {
		result.Base, err = g.defaultBase(ctx, remote, tip)
	} else {
		var base string
		base, err = g.commit(ctx, update.RemoteSHA)
		if base != "" {
			result.Base = update.RemoteSHA
		}
	}
	return result, err
}

func (g rangeGit) commit(ctx context.Context, revision string) (string, error) {
	return g.verify(ctx, revision, rangeObject)
}

func (g rangeGit) verify(ctx context.Context, revision string, judge func([]string, Output) (string, error)) (string, error) {
	args := []string{"rev-parse", "--verify", "--quiet", "--end-of-options", revision + "^{commit}"}
	out, err := g.run(ctx, args...)
	if err != nil {
		return "", err
	}
	return judge(args, out)
}

func (g rangeGit) run(ctx context.Context, args ...string) (Output, error) {
	out, err := Run(ctx, Command{
		Name: "git", Args: append([]string{"-c", "core.quotePath=false"}, args...),
		Dir: g.dir, Env: g.env,
	})
	if err != nil {
		return out, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func rangeObject(args []string, out Output) (string, error) {
	if out.Code == 1 && len(out.Stdout) == 0 && len(out.Stderr) == 0 {
		return "", nil
	}
	if out.Code != 0 {
		return "", rangeCommandError(args, out)
	}
	object := strings.TrimSpace(string(out.Stdout))
	if !validObject(object) || zeroObject(object) {
		return "", &rangeError{reason: fmt.Sprintf("git %s returned invalid object %q", strings.Join(args, " "), out.Stdout)}
	}
	return object, nil
}

func rangeCommandError(args []string, out Output) error {
	return &rangeError{reason: fmt.Sprintf("git %s exited %d: stdout %q, stderr %q",
		strings.Join(args, " "), out.Code, out.Stdout, out.Stderr)}
}
