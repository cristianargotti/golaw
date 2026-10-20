package rung

import (
	"context"
	"slices"
	"strconv"

	"github.com/cristianargotti/golaw/internal/runner"
)

// secretsStep names the gitleaks step, and leaksCode is the exit status it
// asks gitleaks to answer for a leak, apart from the status of a failure.
const (
	secretsStep = "secrets"
	leaksCode   = 3
)

// secrets runs gitleaks over the staged diff of the index git commits
// through. While a merge is in progress that diff holds what the merge
// brought, which the pipeline judges, so the step leaves it alone.
func (p *precommit) secrets(ctx context.Context) Outcome {
	switch {
	case p.scan.Index.MergeHead() != "":
		return passed(secretsStep, "skipped, a merge is in progress and its staged diff holds what the merge brought, which the pipeline judges")
	case len(p.scan.Input.Changes) == 0:
		return passed(secretsStep, "skipped, nothing staged")
	}
	env := runner.UserEnv(p.deps.Environ)
	path, err := verified(ctx, "gitleaks", env)
	if err != nil {
		return stepFailed(secretsStep, err)
	}
	out, err := runner.Run(ctx, secretsCommand(path, p.deps.Dir, env, indexFile(p.deps)))
	switch {
	case err != nil:
		return stepFailed(secretsStep, err)
	case out.Code == 0:
		return passed(secretsStep, "gitleaks found no secret in the staged changes")
	case out.Code == leaksCode:
		return refusedBy(secretsStep, "gitleaks found a secret in the staged changes; take it out of the commit", out)
	default:
		return broke(secretsStep, "gitleaks", out)
	}
}

// secretsCommand is the gitleaks run over the staged diff. Gitleaks asks
// git for that diff itself, so it runs with the user environment and, to
// read the index git commits through, GIT_INDEX_FILE as an absolute path.
func secretsCommand(path, dir string, env []string, index string) runner.Command {
	if index != "" {
		env = append(slices.Clone(env), "GIT_INDEX_FILE="+index)
	}
	return runner.Command{
		Name: path, Dir: dir, Env: env,
		Args: []string{"git", "--pre-commit", "--staged", "--redact", "--no-banner", "--no-color", "--verbose", "--exit-code", strconv.Itoa(leaksCode), "."},
	}
}
