package holes_test

import (
	"reflect"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/holes"
)

func TestKebabAndIDSpellTheHoleIDs(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"ReadPlan": "read-plan", "GetHTTPStatus": "get-http-status", "ArchivePlan2": "archive-plan2",
		"ID": "id", "readPlan": "read-plan", "Now": "now", "OAuthToken": "o-auth-token",
	} {
		if got := holes.Kebab(in); got != want {
			t.Errorf("Kebab(%q) = %q, want %q", in, got, want)
		}
	}
	if got := holes.ID("Plans", "ReadPlan"); got != "plans.read-plan" {
		t.Errorf("ID = %q", got)
	}
	if got := holes.ID("wiring", "Infra"); got != "wiring.infra" {
		t.Errorf("ID = %q", got)
	}
}

func TestDiffOpensAndClosesByID(t *testing.T) {
	t.Parallel()
	before := []holes.Hole{{ID: "plans.read-plan", Path: "a.go", Line: 3}, {ID: "plans.archive-plan", Path: "a.go", Line: 9}}
	after := []holes.Hole{{ID: "plans.purge-plans", Path: "a.go", Line: 20}, {ID: "plans.archive-plan", Path: "a.go", Line: 12}}
	opened, closed := holes.Diff(before, after)
	if !reflect.DeepEqual(opened, []holes.Hole{after[0]}) || !reflect.DeepEqual(closed, []holes.Hole{before[0]}) {
		t.Errorf("Diff = %+v, %+v", opened, closed)
	}
	if opened, closed := holes.Diff(nil, nil); opened != nil || closed != nil {
		t.Errorf("Diff of nothing = %+v, %+v", opened, closed)
	}
}
