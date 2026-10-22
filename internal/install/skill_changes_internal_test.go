package install

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestSharedSkillCopiesOnlyTheSingleSource(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	options := Options{
		Home: home, Root: filepath.Join(home, "repository"), Scope: "user",
	}
	source, err := assets.Read("agents/SKILL.md")
	must(t, err)
	var plan Plan
	must(t, addSharedSkill(&plan, options))
	if len(plan.Changes) != 1 || len(plan.Receipt.Artifacts) != 1 ||
		!bytes.Equal(plan.Changes[0].After, source) {
		t.Fatalf("skill plan = %+v", plan)
	}
	path := SkillPath(options)
	cinWrite(t, path, source)
	var equal Plan
	must(t, addSharedSkill(&equal, options))
	if len(equal.Changes) != 0 || len(equal.Receipt.Artifacts) != 1 {
		t.Fatalf("equal skill plan = %+v", equal)
	}
	cinWrite(t, path, []byte("Personal skill.\n"))
	var changed Plan
	if err := addSharedSkill(&changed, options); !errors.Is(err, ErrChanged) {
		t.Fatalf("unequal skill = %v", err)
	}
	if string(cinRead(t, path)) != "Personal skill.\n" {
		t.Fatal("personal skill was overwritten")
	}
}

// TestSharedSkillUpgradesWhatAReceiptRecorded installs over the skill an
// earlier kit wrote, which a receipt of another agent records: the skill is
// replaced by the source and that receipt is rewritten with the digest of
// the source, so it keeps recording what is on disk (added by the review
// of the plan).
func TestSharedSkillUpgradesWhatAReceiptRecorded(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	state := filepath.Join(home, "state")
	options := Options{Home: home, Root: filepath.Join(home, "repository"), Scope: "user", StateDir: state}
	path := SkillPath(options)
	source, err := assets.Read("agents/SKILL.md")
	must(t, err)
	cinWrite(t, path, []byte("skill"))
	cinSkillReceipt(t, state, "cursor", "user", "project", path)
	plan := Plan{Receipt: Receipt{Scope: "user", ProjectID: "project"}}
	must(t, addSharedSkill(&plan, options))
	other := filepath.Join(state, "cursor", "user", "receipt.json")
	var skill, refreshed []byte
	for _, change := range plan.Changes {
		switch change.Path {
		case path:
			skill = change.After
		case other:
			refreshed = change.After
		}
	}
	if !bytes.Equal(skill, source) || len(plan.Receipt.Artifacts) != 1 || plan.Receipt.Artifacts[0].SHA256 != Digest(source) {
		t.Fatalf("upgrade plan = %+v", plan)
	}
	var receipt Receipt
	must(t, json.Unmarshal(refreshed, &receipt))
	if len(receipt.Artifacts) != 1 || receipt.Artifacts[0].SHA256 != Digest(source) {
		t.Fatalf("the other owner was not refreshed: %s", refreshed)
	}
}

func TestSharedSkillRemovalUsesOtherReceipts(t *testing.T) {
	t.Parallel()
	for _, shared := range []bool{false, true} {
		t.Run(map[bool]string{false: "sole owner", true: "shared owner"}[shared],
			func(t *testing.T) {
				t.Parallel()
				home := cinHome(t)
				state := filepath.Join(home, "state")
				path := filepath.Join(home, ".agents", "skills", "golaw", "SKILL.md")
				cinWrite(t, path, []byte("skill"))
				if shared {
					cinSkillReceipt(t, state, "cursor", "user", "project", path)
				}
				plan := Plan{Receipt: Receipt{Scope: "user", ProjectID: "project"}}
				artifact := Artifact{Path: path, SHA256: Digest([]byte("skill"))}
				must(t, removeSharedSkill(&plan, state, "codex", artifact))
				want := 1
				if shared {
					want = 0
				}
				if len(plan.Changes) != want {
					t.Fatalf("removal changes = %+v", plan.Changes)
				}
				if want == 1 && !plan.Changes[0].Remove {
					t.Fatal("sole-owner skill was not scheduled for removal")
				}
			})
	}
}

// TestSharedSkillRemovalAcceptsTheCurrentSource removes a skill another
// install upgraded to the current source after this receipt recorded an
// older one: the bytes are golaw's, so the sole owner removes them instead
// of refusing them as changed (added by the review of the plan).
func TestSharedSkillRemovalAcceptsTheCurrentSource(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	path := filepath.Join(home, ".agents", "skills", "golaw", "SKILL.md")
	source, err := assets.Read("agents/SKILL.md")
	must(t, err)
	cinWrite(t, path, source)
	plan := Plan{Receipt: Receipt{Scope: "user", ProjectID: "project"}}
	artifact := Artifact{Path: path, SHA256: Digest([]byte("an older skill"))}
	must(t, removeSharedSkill(&plan, filepath.Join(home, "state"), "codex", artifact))
	if len(plan.Changes) != 1 || !plan.Changes[0].Remove {
		t.Fatalf("removal of an upgraded skill = %+v", plan.Changes)
	}
}

// TestSharedSkillRemovalKeepsWhatIsNotGolaws removes nothing when the
// skill is already gone, refuses bytes no receipt and no source names, and
// reads the digest another owner's receipt records as golaw's, which that
// owner then keeps on disk.
func TestSharedSkillRemovalKeepsWhatIsNotGolaws(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	state := filepath.Join(home, "state")
	path := filepath.Join(home, ".agents", "skills", "golaw", "SKILL.md")
	plan := Plan{Receipt: Receipt{Scope: "user", ProjectID: "project"}}
	older := Artifact{Path: path, SHA256: Digest([]byte("an older skill"))}
	must(t, removeSharedSkill(&plan, state, "codex", older))
	if len(plan.Changes) != 0 {
		t.Fatalf("removal of an absent skill = %+v", plan.Changes)
	}
	cinWrite(t, path, []byte("Personal skill.\n"))
	if err := removeSharedSkill(&plan, state, "codex", older); !errors.Is(err, ErrChanged) {
		t.Fatalf("a personal skill = %v", err)
	}
	cinWrite(t, path, []byte("skill"))
	cinSkillReceipt(t, state, "cursor", "user", "other", path)
	must(t, removeSharedSkill(&plan, state, "codex", older))
	if len(plan.Changes) != 0 {
		t.Fatalf("a skill another owner records was removed: %+v", plan.Changes)
	}
	must(t, removeSharedSkill(&plan, state, "cursor", older))
	if len(plan.Changes) != 1 || !plan.Changes[0].Remove {
		t.Fatalf("the last owner kept the skill: %+v", plan.Changes)
	}
	cinWrite(t, filepath.Join(state, "cursor", "user", "receipt.json"), []byte("{"))
	if err := removeSharedSkill(&plan, state, "codex", older); err == nil {
		t.Fatal("corrupt ownership evidence accepted on removal")
	}
}

// TestSharedSkillRefreshesOnlyStaleOtherOwners leaves the receipt being
// planned to its planner and a receipt that already records the source,
// and refuses a skill path that is no file or a state it cannot read.
func TestSharedSkillRefreshesOnlyStaleOtherOwners(t *testing.T) {
	t.Parallel()
	home := cinHome(t)
	state := filepath.Join(home, "state")
	options := Options{Home: home, Root: filepath.Join(home, "repository"), Scope: "user", StateDir: state}
	path := SkillPath(options)
	source, err := assets.Read("agents/SKILL.md")
	must(t, err)
	cinWrite(t, path, []byte("skill"))
	cinSkillReceipt(t, state, "codex", "user", "project", path)
	fresh := Receipt{
		Schema: 1, ProjectID: "project", Scope: "user", Version: "0.4.0", Agent: "fresh",
		Bundle: filepath.Join(state, "fresh", "user"), Artifacts: []Artifact{{Path: path, SHA256: Digest(source)}},
	}
	data, err := json.Marshal(fresh)
	must(t, err)
	cinWrite(t, filepath.Join(state, "fresh", "user", "receipt.json"), data)
	own := filepath.Join(state, "codex", "user", "receipt.json")
	plan := Plan{Receipt: Receipt{Scope: "user", ProjectID: "project"}, ReceiptPath: own}
	must(t, addSharedSkill(&plan, options))
	if len(plan.Changes) != 1 || plan.Changes[0].Path != path {
		t.Fatalf("refreshed receipts = %+v", plan.Changes)
	}
	cinWrite(t, filepath.Join(state, "fresh", "user", "receipt.json"), []byte("{"))
	if err := addSharedSkill(&Plan{}, options); err == nil {
		t.Fatal("corrupt ownership evidence accepted on install")
	}
	must(t, os.Remove(path))
	must(t, os.MkdirAll(path, 0o700))
	if err := addSharedSkill(&Plan{}, options); !errors.Is(err, ErrPath) {
		t.Fatalf("a skill path that is a directory = %v", err)
	}
}
