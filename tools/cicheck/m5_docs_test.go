package cicheck_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

const (
	m5ProfilesPath = "docs/profiles.md"
	m5FixtureDir   = "internal/profile/testdata/fixture"
	m5RulesExample = "### Example: the fixture rules.yml"
)

// m5GuideClaims are the sentences of the profile guide that state what M5
// built; the M3 sentences stay pinned by TestM3ProfileContract.
func m5GuideClaims() []string {
	return []string{
		"`profile.yml` is its identity and `rules.yml` its data; every other regular file, such as the onboarding page, a README or fixtures, is data the kit never reads.",
		"for every regular file of the bundle except `profile.yml`, sorted by path, with a `.git` directory left out.",
		"`rules.yml` is one YAML document whose first key is `golaw_profile: 1`, decoded strictly: an unknown or duplicate key is a schema error.",
		"the schema has no key that names a core floor, so a profile cannot relax one by construction.",
		"`<config>/profiles/<name>@<version>/` is the store, one immutable directory per installed version.",
		"`<state>/profiles/<name>.json` is the install record",
		"golaw profile install <git url>@<tag>\n",
		"None of them reads or writes a `golaw.yml`.",
		"`Perfil <name> <version> instalado de <url>@<tag>, digest <digest>`",
		"`Perfil <name> <version> já estava instalado`",
		"The same version with another digest is refused, because a published version never changes.",
		"`Perfil <name> voltou para <version>`",
		"`Perfil <name> removido`",
		"Refresh, rollback and install alone never change a pin",
		"`profile <name> is not installed: run golaw profile install <source>@<version>`",
		"`profile <name>: rules.yml: <cause>`, which names no command",
		"`Perfil <name> ausente ou divergente: execute golaw profile install <source>@<version>`",
		"so a fresh clone never bricks.",
		"`profile-installed` with `version <v>, digest <d>, <n> days since <date>`",
		"Without a remote stage, automatic kit checks run only on machines whose developers armed them.",
		"commit, then run `golaw init --apply` once more to enroll the committed pin",
		"A teammate who pulls a moved pin installs that version with the command the pin names and runs `golaw init --apply` to enroll it.",
		"Run it over a clean clone of the commit to tag, because every regular file counts, ignored ones included:",
		"or the `.git` file at the root of a linked worktree, gives a digest no clone of the tag reproduces",
	}
}

// m5GuideStale are the M3 sentences that announced M5 as future work.
func m5GuideStale() []string {
	return []string{
		"M5 will add payload loading",
		"The installer named by that diagnostic is future M5 behavior",
		"The zero digest demonstrates syntax",
		"golaw profile digest` over a clean clone or worktree",
	}
}

func TestM5ProfileGuide(t *testing.T) {
	t.Parallel()
	text := string(kitBytes(t, m5ProfilesPath))
	claims, stale := m5GuideClaims(), m5GuideStale()
	rows := []struct {
		name, text string
		want       bool
	}{
		{"repository", text, true},
		{"red-future-lifecycle", text + "\n" + stale[0] + ".\n", false},
		{"red-missing-store", strings.Replace(text, claims[4], "", 1), false},
		{"red-missing-human-line", strings.Replace(text, claims[16], "", 1), false},
		{"red-missing-pin-enrollment", strings.Replace(text, claims[20], "", 1), false},
		{"red-missing-teammate-step", strings.Replace(text, claims[21], "", 1), false},
		{"red-missing-clean-clone", strings.Replace(text, claims[22], "", 1), false},
		{"red-missing-worktree-file", strings.Replace(text, claims[23], "", 1), false},
		{"red-worktree-advice", text + "\nRun `golaw profile digest` over a clean clone or worktree of the tag.\n", false},
		{"near-extra-example", text + "\nAnother synthetic bundle may serve as an example.\n", true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			if got := m4TextOK(row.text, claims, stale); got != row.want {
				t.Fatalf("profile guide = %v, want %v", got, row.want)
			}
		})
	}
	if got := lineCount([]byte(text)); got > 300 {
		t.Fatalf("%s has %d lines, maximum 300", m5ProfilesPath, got)
	}
}

// m5RulesBlock answers the YAML block under the rules example heading.
func m5RulesBlock(t *testing.T) []byte {
	t.Helper()
	_, rest, found := strings.Cut(string(kitBytes(t, m5ProfilesPath)), m5RulesExample+"\n\n```yaml\n")
	if !found {
		t.Fatalf("the rules example is absent: %s", m5RulesExample)
	}
	body, _, found := strings.Cut(rest, "\n```")
	if !found {
		t.Fatal("the rules example is not closed")
	}
	return []byte(body + "\n")
}

func TestM5ProfileExamplesAreTheFixture(t *testing.T) {
	t.Parallel()
	rules := m5RulesBlock(t)
	if !bytes.Equal(rules, kitBytes(t, m5FixtureDir+"/rules.yml")) {
		t.Fatal("the rules example differs from the fixture rules.yml")
	}
	if !bytes.Equal(m3ProfileYAML(t, "## Bundle identity"), kitBytes(t, m5FixtureDir+"/profile.yml")) {
		t.Fatal("the stamp example differs from the fixture profile.yml")
	}
	digest, err := profile.Digest(filepath.Join("..", "..", filepath.FromSlash(m5FixtureDir)))
	if err != nil {
		t.Fatal(err)
	}
	config, err := policy.Decode(m3ProfileYAML(t, "## Repository reference"))
	if err != nil || config.Profile == nil || config.Profile.Digest != digest {
		t.Fatalf("the repository example does not pin the fixture digest %s: %v", digest, err)
	}
	rows := []struct {
		name string
		data []byte
		bad  bool
	}{
		{"near-example", rules, false},
		{"red-unknown-key", append(bytes.Clone(rules), "unknown_key: true\n"...), true},
		{"red-schema-version", bytes.Replace(rules, []byte("golaw_profile: 1"), []byte("golaw_profile: 2"), 1), true},
		{"red-bad-term", bytes.Replace(rules, []byte("[legacyci]"), []byte("[Legacy CI]"), 1), true},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			data, err := profile.Decode(row.data)
			if err == nil {
				err = profile.Validate(data)
			}
			if errors.Is(err, profile.ErrBundle) != row.bad {
				t.Fatalf("rules error = %v, invalid = %v", err, row.bad)
			}
		})
	}
}
