package doctor

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cristianargotti/golaw/internal/install"
	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/runner"
	"github.com/cristianargotti/golaw/internal/setup"
)

// CollectSetup trusts recorded paths only after the receipt identifies this root.
func CollectSetup(ctx context.Context, root, state string, env []string, ladder model.Ladder) (SetupFacts, error) {
	canonical, identity, err := install.ProjectIdentity(root)
	if err != nil {
		return SetupFacts{}, fmt.Errorf("identify setup repository: %w", err)
	}
	facts := SetupFacts{ReceiptPath: setup.ReceiptPath(state, identity), Ladder: ladder}
	facts.Receipt, err = setup.ReadReceipt(facts.ReceiptPath)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		facts.ReceiptState = setupAbsent
	case err != nil:
		facts.ReceiptState, facts.ReceiptError = setupInvalid, err.Error()
	case facts.Receipt.Identity != identity || facts.Receipt.Root != canonical:
		facts.ReceiptState, facts.ReceiptError = setupInvalid, "receipt belongs to a different repository"
	default:
		facts.ReceiptState = setupPresent
		collectSetupRouting(ctx, canonical, env, &facts)
		for _, hook := range facts.Receipt.Hooks {
			facts.Hooks = append(facts.Hooks, collectSetupHook(hook.Path))
		}
		facts.Tools = collectSetupTools(ctx, env, facts.Receipt.Tools)
	}
	return facts, nil
}

func collectSetupHook(path string) SetupHook {
	hook := SetupHook{Path: path}
	info, err := os.Lstat(path)
	if err != nil {
		hook.Error = setupHookError(err)
		return hook
	}
	if !info.Mode().IsRegular() {
		hook.Error = "hook is not a regular file"
		return hook
	}
	data, err := os.ReadFile(path) //nolint:gosec // The validated receipt records the hook path.
	if err != nil {
		hook.Error = setupHookError(err)
		return hook
	}
	hook.SHA256 = install.Digest(data)
	return hook
}

func setupHookError(err error) string {
	if errors.Is(err, fs.ErrNotExist) {
		return setupMissing + ": " + err.Error()
	}
	return err.Error()
}

func collectSetupRouting(ctx context.Context, root string, env []string, facts *SetupFacts) {
	local, localErr := setupGitPath(ctx, root, env, true)
	effective, effectiveErr := setupGitPath(ctx, root, env, false)
	facts.LocalHooksPath, facts.EffectiveHooksPath = local, effective
	if err := errors.Join(localErr, effectiveErr); err != nil {
		facts.HooksPathError = strings.NewReplacer("\r\n", "; ", "\n", "; ", "\r", "; ").Replace(err.Error())
	}
}

func setupGitPath(ctx context.Context, root string, env []string, local bool) (string, error) {
	args := []string{"rev-parse", "--path-format=absolute", "--git-path", "hooks"}
	if local {
		args = []string{"config", "--local", "--get", "core.hooksPath"}
	}
	out, err := runner.Run(ctx, runner.Command{Name: "git", Args: args, Dir: root, Env: env})
	if err != nil {
		return "", fmt.Errorf("read setup hook routing; put git on PATH and run golaw doctor again: %w", err)
	}
	if local && out.Code == 1 && len(out.Stdout) == 0 && len(out.Stderr) == 0 {
		return "", nil
	}
	path := strings.TrimSuffix(strings.TrimSuffix(string(out.Stdout), "\n"), "\r")
	if out.Code != 0 {
		return path, fmt.Errorf("%w: git %v exited %d: %s; repair the Git configuration and run golaw doctor again",
			install.ErrPath, args, out.Code, strings.TrimSpace(string(out.Stderr)))
	}
	if !local && !filepath.IsAbs(path) {
		return path, fmt.Errorf("%w: Git returned a relative hooks directory %s; inspect core.hooksPath and run golaw doctor again",
			install.ErrPath, path)
	}
	return path, nil
}

func setupRoutingLines(facts SetupFacts) []Line {
	if facts.ReceiptState != setupPresent {
		return nil
	}
	local := facts.LocalHooksPath
	if local == "" {
		local = "unset"
	}
	line := Line{
		ID: idSetupRouting, State: stateOK, Code: "setup-routing-matches",
		Detail: fmt.Sprintf("effective hooks path %s; local core.hooksPath %s", facts.EffectiveHooksPath, local),
	}
	switch {
	case facts.HooksPathError != "":
		line.State, line.Detail = stateUnknown, facts.HooksPathError
	case facts.EffectiveHooksPath == "":
		line.State, line.Detail = stateUnknown, "effective hooks path was not collected"
	case filepath.Clean(facts.EffectiveHooksPath) != filepath.Clean(facts.Receipt.HooksPath):
		line.State = setupDrift
	}
	if line.State != stateOK {
		line.Code = "setup-routing-drift"
		line.Detail += "; inspect core.hooksPath and restore the recorded routing or run golaw setup again"
	}
	return []Line{line}
}
