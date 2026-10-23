package policy_test

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// upgradeCommit stages the golaw.yml init --update writes for 0.2.0 over a
// repository enrolled under the example profile, the record already
// holding it, and answers the LoadNext input of that commit with the
// loader given.
func upgradeCommit(t *testing.T, bundle policy.BundleLoader) policy.Input {
	t.Helper()
	root, d := enrolled(t, fullPolicy)
	next := strings.Replace(fullPolicy, "kit: {version: 0.1.0}", "kit: {version: 0.2.0}", 1)
	writeRecord(t, d.State, policy.Record{
		Root: root, PolicyDigest: policy.Digest(decode(t, next)), ProfileDigest: "sha256:" + acmeDigest,
		KitVersion: "0.2.0", Bootstrap: next, WrittenBy: policy.WrittenByInit, WrittenAt: now(),
	})
	write(t, root, "golaw.yml", next)
	gitRun(t, root, "add", "golaw.yml")
	in := input(root, d)
	in.Version, in.Bundle = "0.2.0", bundle
	return in
}

func TestLoadNextReadsTheBundleOfAnUpgradeCommit(t *testing.T) {
	t.Parallel()
	l, err := policy.LoadNext(t.Context(), upgradeCommit(t, func(string, policy.ProfileRef) (*model.ProfileData, error) {
		return acmeData(), nil
	}))
	if err != nil || l.Origin != policy.OriginStaged || l.Data == nil || l.Policy.ProfileData == nil || !slices.Contains(l.Policy.Terms.Tools, "zeta") {
		t.Fatalf("the upgrade commit without the bundle data: %+v, %v", l, err)
	}
	refusal := "profile acme: digest sha256:0 of the installed bundle differs from its stamp: run golaw profile install https://example.com/acme.git@0.3.0"
	_, err = policy.LoadNext(t.Context(), upgradeCommit(t, func(string, policy.ProfileRef) (*model.ProfileData, error) {
		return nil, fmt.Errorf("%w: %s", model.ErrPolicy, refusal)
	}))
	if !errors.Is(err, model.ErrPolicy) || !strings.HasSuffix(err.Error(), refusal) {
		t.Errorf("the loader error of an upgrade commit: %v", err)
	}
	plain, err := policy.LoadNext(t.Context(), upgradeCommit(t, nil))
	if err != nil || plain.Data != nil || plain.Stamp == nil {
		t.Errorf("a nil loader verifies the stamp alone: %+v, %v", plain, err)
	}
}
