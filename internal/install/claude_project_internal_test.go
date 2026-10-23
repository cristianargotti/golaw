package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// projectOptions are the options of a project installation in a root that
// holds a git directory, so the install has an exclude file to write.
func projectOptions(t *testing.T) Options {
	t.Helper()
	options := testOptions(t)
	options.Scope = scopeProject
	must(t, os.MkdirAll(filepath.Join(options.Root, ".git"), 0o700))
	return options
}

func applyPlan(t *testing.T, plan Plan) {
	t.Helper()
	var output bytes.Buffer
	must(t, Apply(t.Context(), plan, &output, &enrollmentDouble{output: &output}))
}

func applyRemoval(t *testing.T, receiptPath string) {
	t.Helper()
	removal, err := PlanUninstall(receiptPath)
	must(t, err)
	var output bytes.Buffer
	must(t, Apply(t.Context(), removal, &output, nil))
}

func mustPlan(t *testing.T, options Options) Plan {
	t.Helper()
	plan, err := PlanClaude(options)
	must(t, err)
	return plan
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	must(t, err)
	return data
}

func absent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("%s remains: %v", path, err)
	}
}

func TestProjectInstallWritesLocalSettings(t *testing.T) {
	t.Parallel()
	options := projectOptions(t)
	plan := mustPlan(t, options)
	local := filepath.Join(options.Root, ".claude", "settings.local.json")
	if plan.Receipt.SettingsPath != local || SharedSettings(plan.Receipt) {
		t.Fatalf("project settings path: %s", plan.Receipt.SettingsPath)
	}
	applyPlan(t, plan)
	absent(t, filepath.Join(options.Root, ".claude", "settings.json"))
	var settings struct {
		Plugins map[string]bool `json:"enabledPlugins"`
		Markets map[string]struct {
			Source struct{ Path string } `json:"source"`
		} `json:"extraKnownMarketplaces"`
	}
	data := mustRead(t, local)
	must(t, json.Unmarshal(data, &settings))
	if !settings.Plugins["golaw@golaw"] || settings.Markets["golaw"].Source.Path != plan.Receipt.Bundle {
		t.Fatalf("local settings miss the owned entries: %s", data)
	}
	exclude := filepath.Join(options.Root, ".git", "info", "exclude")
	if got := mustRead(t, exclude); string(got) != excludeLine+"\n" {
		t.Fatalf("exclude file: %q", got)
	}
	if plan.Receipt.Exclude == nil || !plan.Receipt.Exclude.Created {
		t.Fatalf("exclude record: %+v", plan.Receipt.Exclude)
	}
	if again := mustPlan(t, options); len(again.Changes) != 0 {
		t.Fatalf("reinstall changed %d files", len(again.Changes))
	}
	applyRemoval(t, plan.ReceiptPath)
	absent(t, local)
	absent(t, exclude)
}

func TestProjectInstallKeepsThePersonsLines(t *testing.T) {
	t.Parallel()
	options := projectOptions(t)
	exclude := filepath.Join(options.Root, ".git", "info", "exclude")
	mine := []byte("# mine\r\n" + excludeLine + "\r\n")
	must(t, os.MkdirAll(filepath.Dir(exclude), 0o700))
	must(t, os.WriteFile(exclude, mine, 0o600))
	plan := mustPlan(t, options)
	if plan.Receipt.Exclude != nil {
		t.Fatal("a line the person wrote became owned")
	}
	applyPlan(t, plan)
	applyRemoval(t, plan.ReceiptPath)
	if got := mustRead(t, exclude); !bytes.Equal(got, mine) {
		t.Fatalf("the person's exclude file changed: %q", got)
	}
}

func TestProjectUninstallKeepsLineWhileSettingsStay(t *testing.T) {
	t.Parallel()
	options := projectOptions(t)
	exclude := filepath.Join(options.Root, ".git", "info", "exclude")
	must(t, os.MkdirAll(filepath.Dir(exclude), 0o700))
	must(t, os.WriteFile(exclude, []byte("# git"), 0o600))
	local := filepath.Join(options.Root, ".claude", "settings.local.json")
	must(t, os.MkdirAll(filepath.Dir(local), 0o700))
	must(t, os.WriteFile(local, []byte(`{"permissions":{"allow":["Bash(make test)"]}}`), 0o600))
	plan := mustPlan(t, options)
	if plan.Receipt.Exclude == nil || plan.Receipt.Exclude.Created {
		t.Fatalf("exclude record: %+v", plan.Receipt.Exclude)
	}
	applyPlan(t, plan)
	applyRemoval(t, plan.ReceiptPath)
	if got := mustRead(t, exclude); string(got) != "# git\n"+excludeLine+"\n" {
		t.Fatalf("the line left while the person's settings stay: %q", got)
	}
}

func TestProjectUninstallKeepsTheRestOfTheExcludeFile(t *testing.T) {
	t.Parallel()
	options := projectOptions(t)
	exclude := filepath.Join(options.Root, ".git", "info", "exclude")
	must(t, os.MkdirAll(filepath.Dir(exclude), 0o700))
	must(t, os.WriteFile(exclude, []byte("# git"), 0o600))
	plan := mustPlan(t, options)
	applyPlan(t, plan)
	if got := mustRead(t, exclude); string(got) != "# git\n"+excludeLine+"\n" {
		t.Fatalf("the owned line was not appended: %q", got)
	}
	applyRemoval(t, plan.ReceiptPath)
	if got := mustRead(t, exclude); string(got) != "# git\n" {
		t.Fatalf("the owned line stayed: %q", got)
	}
}

// sharedLayout rewrites an applied project installation into the layout a
// release before settings.local.json left: the owned entries merged into
// the person's settings.json, a receipt that names it and no exclude line.
func sharedLayout(t *testing.T, options Options, plan Plan, person []byte) string {
	t.Helper()
	shared := filepath.Join(filepath.Dir(plan.Receipt.SettingsPath), "settings.json")
	merged, owned, err := MergeSettings(person, plan.Receipt.Bundle, options.ReadRules, Ownership{})
	must(t, err)
	must(t, os.WriteFile(shared, merged, 0o600))
	must(t, os.Remove(plan.Receipt.SettingsPath))
	must(t, os.Remove(plan.Receipt.Exclude.Path))
	receipt := plan.Receipt
	receipt.SettingsPath, receipt.Ownership, receipt.Exclude = shared, owned, nil
	for index := range receipt.Artifacts {
		if receipt.Artifacts[index].Path == plan.Receipt.SettingsPath {
			receipt.Artifacts[index] = Artifact{Path: shared, SHA256: Digest(merged)}
		}
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	must(t, err)
	must(t, os.WriteFile(plan.ReceiptPath, encoded, 0o600))
	return shared
}

func TestProjectInstallMovesSharedSettings(t *testing.T) {
	t.Parallel()
	options := projectOptions(t)
	person := mustRead(t, "testdata/foreign.json")
	plan := mustPlan(t, options)
	applyPlan(t, plan)
	shared := sharedLayout(t, options, plan, person)
	old, err := ReadReceipt(plan.ReceiptPath)
	must(t, err)
	if !SharedSettings(old) {
		t.Fatal("the earlier receipt is not read as shared")
	}
	upgrade := mustPlan(t, options)
	if len(upgrade.Notes) != 2 || !strings.Contains(upgrade.Notes[0], shared) {
		t.Fatalf("the move is not named: %q", upgrade.Notes)
	}
	applyPlan(t, upgrade)
	assertJSONEqual(t, person, mustRead(t, shared))
	receipt, err := ReadReceipt(plan.ReceiptPath)
	must(t, err)
	if receipt.SettingsPath != plan.Receipt.SettingsPath || receipt.Exclude == nil {
		t.Fatalf("upgraded receipt: %s %+v", receipt.SettingsPath, receipt.Exclude)
	}
	if err := VerifySettings(mustRead(t, receipt.SettingsPath), receipt.Ownership); err != nil {
		t.Fatalf("local settings after the move: %v", err)
	}
	if again := mustPlan(t, options); len(again.Changes) != 0 {
		t.Fatalf("a second install after the move changed %d files", len(again.Changes))
	}
}

func TestUninstallOfSharedReceiptRestoresSettings(t *testing.T) {
	t.Parallel()
	options := projectOptions(t)
	person := mustRead(t, "testdata/foreign.json")
	plan := mustPlan(t, options)
	applyPlan(t, plan)
	shared := sharedLayout(t, options, plan, person)
	applyRemoval(t, plan.ReceiptPath)
	assertJSONEqual(t, person, mustRead(t, shared))
	absent(t, plan.Receipt.SettingsPath)
}

func TestGitExcludeFileFollowsLinkedWorktrees(t *testing.T) {
	t.Parallel()
	base, err := filepath.EvalSymlinks(t.TempDir())
	must(t, err)
	if _, found, findErr := gitExcludeFile(base); found || findErr != nil {
		t.Fatalf("a root without .git: %v %v", found, findErr)
	}
	common := filepath.Join(base, "main", ".git")
	linked := filepath.Join(common, "worktrees", "feature")
	must(t, os.MkdirAll(linked, 0o700))
	must(t, os.WriteFile(filepath.Join(linked, "commondir"), []byte("../..\n"), 0o600))
	worktree := filepath.Join(base, "feature")
	must(t, os.MkdirAll(worktree, 0o700))
	must(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+linked+"\n"), 0o600))
	path, found, err := gitExcludeFile(worktree)
	if err != nil || !found || path != filepath.Join(common, "info", "exclude") {
		t.Fatalf("linked worktree exclude: %s %v %v", path, found, err)
	}
	must(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: ../main/.git\n"), 0o600))
	if path, _, err = gitExcludeFile(worktree); err != nil || path != filepath.Join(common, "info", "exclude") {
		t.Fatalf("relative gitdir: %s %v", path, err)
	}
	must(t, os.WriteFile(filepath.Join(worktree, ".git"), []byte("not a pointer\n"), 0o600))
	if _, _, err = gitExcludeFile(worktree); !errors.Is(err, ErrPath) {
		t.Fatalf("malformed pointer: %v", err)
	}
}

func TestRemoveExcludeRefusesForeignRecord(t *testing.T) {
	t.Parallel()
	plan := Plan{Receipt: Receipt{Exclude: &Exclude{Path: filepath.Join(t.TempDir(), "notes"), Line: excludeLine}}}
	if err := removeExclude(&plan, true); !errors.Is(err, ErrPath) {
		t.Fatalf("a record outside info/exclude: %v", err)
	}
	if err := removeExclude(&plan, false); err != nil || len(plan.Changes) != 0 {
		t.Fatalf("a kept settings file: %v", err)
	}
}
