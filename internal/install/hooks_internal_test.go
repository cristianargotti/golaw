package install

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"github.com/cristianargotti/golaw/internal/assets"
)

func TestPluginSchema(t *testing.T) {
	t.Parallel()
	data, err := assets.Read("agents/claude/hooks/hooks.json")
	if err != nil {
		t.Fatal(err)
	}
	if validateErr := ValidateHooks(data); validateErr != nil {
		t.Fatal(validateErr)
	}
	rejectHandlerMutations(t, data)
	hooks, err := ParseHooks(data)
	if err != nil {
		t.Fatal(err)
	}
	delete(hooks.Hooks, "PostToolUseFailure")
	changed, err := json.Marshal(hooks)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateHooks(changed); err == nil {
		t.Fatal("missing failure event accepted")
	}
	if _, err := ParseHooks(append(data, []byte("{}")...)); err == nil {
		t.Fatal("trailing document accepted")
	}
}

// rejectHandlerMutations checks that every change of the exec form is
// refused.
func rejectHandlerMutations(t *testing.T, data []byte) {
	t.Helper()
	for _, replacement := range []struct {
		old string
		new string
	}{
		{`"timeout":10`, `"timeout":9`},
		{`"onFailure":"block"`, `"onFailure":"allow"`},
		{`"command":"golaw"`, `"command":"golaw hook claude"`},
		{`"args":["hook","claude"]`, `"args":[]`},
		{`"type":"command"`, `"type":"command","unknown":true`},
	} {
		changed := bytes.Replace(data, []byte(replacement.old), []byte(replacement.new), 1)
		if bytes.Equal(changed, data) {
			t.Fatalf("fixture did not change %s", replacement.old)
		}
		if validateErr := ValidateHooks(changed); validateErr == nil {
			t.Fatalf("invalid handler accepted: %s", replacement.new)
		}
	}
}

func TestPinnedDistributionCatalog(t *testing.T) {
	t.Parallel()
	data, err := os.ReadFile("../../.claude-plugin/marketplace.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateCatalog(data); err != nil {
		t.Fatal(err)
	}
	for _, ref := range []string{"main", "", "v0.1"} {
		changed := bytes.ReplaceAll(data, []byte("v0.1.0"), []byte(ref))
		if err := ValidateCatalog(changed); err == nil {
			t.Fatalf("unpinned catalog accepted: %q", ref)
		}
	}
}
