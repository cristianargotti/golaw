package runner

import (
	"context"
	"fmt"
)

func (g rangeGit) defaultBase(ctx context.Context, remote, tip string) (string, error) {
	known, err := g.trackingNamespace(ctx, remote)
	if err != nil || !known {
		return "", err
	}
	for _, branch := range []string{"HEAD", "main", "master"} {
		base, lookupErr := g.verify(ctx, "refs/remotes/"+remote+"/"+branch, trackingObject)
		if lookupErr != nil {
			return "", lookupErr
		}
		if base != "" {
			return g.mergeBase(ctx, base, tip)
		}
	}
	return "", nil
}

// trackingObject takes an exit 1 without an object as an absent default
// whatever stderr says: git answers a dangling or broken tracking ref as it
// answers a missing one, with a warning in the user's language, and the
// next candidate or an empty base is what a missing default gives.
func trackingObject(args []string, out Output) (string, error) {
	if out.Code == 1 && len(out.Stdout) == 0 {
		return "", nil
	}
	return rangeObject(args, out)
}

func (g rangeGit) trackingNamespace(ctx context.Context, remote string) (bool, error) {
	// Git may echo a remote URL containing credentials, so only status is safe.
	out, err := Run(ctx, Command{
		Name: "git", Args: []string{"-c", "core.quotePath=false", "check-ref-format", "refs/remotes/" + remote + "/HEAD"},
		Dir: g.dir, Env: g.env,
	})
	if err != nil {
		return false, fmt.Errorf("check the tracking namespace of the remote: %w", err)
	}
	if (out.Code != 0 && out.Code != 1) || len(out.Stdout) != 0 || len(out.Stderr) != 0 {
		return false, &rangeError{reason: fmt.Sprintf("check the tracking namespace of the remote: git exited %d with an unexpected response", out.Code)}
	}
	return out.Code == 0, nil
}

func (g rangeGit) mergeBase(ctx context.Context, base, tip string) (string, error) {
	args := []string{"merge-base", "--", base, tip}
	out, err := g.run(ctx, args...)
	if err != nil {
		return "", err
	}
	return rangeObject(args, out)
}
