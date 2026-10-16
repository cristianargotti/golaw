package policy_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
	"github.com/cristianargotti/golaw/internal/policy"
)

// acmeStamp is profile.yml of the example bundle.
const acmeStamp = "name: acme\nversion: 0.3.0\ndate: 2026-10-14\nsource: https://example.com/acme.git\ndigest: sha256:" + acmeDigest + "\n"

// acmeRef is the profile reference the example policy carries.
func acmeRef() policy.ProfileRef {
	return policy.ProfileRef{Name: "acme", Source: "https://example.com/acme.git", Version: "0.3.0", Digest: "sha256:" + acmeDigest}
}

func TestLoadStamp(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	write(t, config, "profiles/acme/profile.yml", acmeStamp)
	s, err := policy.LoadStamp(policy.StampPath(config, "acme"))
	if err != nil || s.Name != "acme" || s.Date != "2026-10-14" || s.Digest != "sha256:"+acmeDigest {
		t.Errorf("LoadStamp = %+v, %v", s, err)
	}
	bad := map[string]string{
		"unknown key": acmeStamp + "extra: 1\n",
		"bad name":    strings.Replace(acmeStamp, "name: acme", "name: Acme", 1),
		"bad date":    strings.Replace(acmeStamp, "2026-10-14", "yesterday", 1),
		"bad digest":  strings.Replace(acmeStamp, acmeDigest, "xyz", 1),
		"not yaml":    "name: [\n",
	}
	for name, text := range bad {
		write(t, config, "profiles/"+name+"/profile.yml", text)
		if _, err := policy.LoadStamp(policy.StampPath(config, name)); !errors.Is(err, model.ErrPolicy) {
			t.Errorf("%s: %v, want ErrPolicy", name, err)
		}
	}
	if _, err := policy.LoadStamp(policy.StampPath(config, "absent")); !errors.Is(err, model.ErrPolicy) {
		t.Errorf("absent: %v", err)
	}
}

func TestVerifyProfile(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	_, err := policy.VerifyProfile(config, acmeRef())
	if !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "golaw profile install https://example.com/acme.git@0.3.0") {
		t.Errorf("a missing bundle names the install command: %v", err)
	}
	write(t, config, "profiles/acme/profile.yml", acmeStamp)
	if s, err := policy.VerifyProfile(config, acmeRef()); err != nil || s.Version != "0.3.0" {
		t.Errorf("VerifyProfile = %+v, %v", s, err)
	}
	for field, change := range map[string]func(*policy.ProfileRef){
		"source":  func(r *policy.ProfileRef) { r.Source = "https://example.com/evil.git" },
		"version": func(r *policy.ProfileRef) { r.Version = "0.4.0" },
		"digest":  func(r *policy.ProfileRef) { r.Digest = "sha256:" + strings.Repeat("f", 64) },
	} {
		ref := acmeRef()
		change(&ref)
		if _, err := policy.VerifyProfile(config, ref); !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), field+" is") {
			t.Errorf("%s mismatch: %v", field, err)
		}
	}
	other := strings.Replace(strings.Replace(acmeStamp, "name: acme", "name: other", 1), "acme.git", "other.git", 1)
	write(t, config, "profiles/other/profile.yml", other)
	ref := acmeRef()
	ref.Name = "other"
	if _, err := policy.VerifyProfile(config, ref); !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "source is") {
		t.Errorf("a bundle whose source differs: %v", err)
	}
	write(t, config, "profiles/broken/profile.yml", "name: [\n")
	ref.Name = "broken"
	if _, err := policy.VerifyProfile(config, ref); !errors.Is(err, model.ErrPolicy) || strings.Contains(err.Error(), "install") {
		t.Errorf("a broken stamp is not a missing bundle: %v", err)
	}
}

func TestVerifyProfileRefusesAStampOfAnotherName(t *testing.T) {
	t.Parallel()
	config := t.TempDir()
	write(t, config, "profiles/acme/profile.yml", strings.Replace(acmeStamp, "name: acme", "name: other", 1))
	_, err := policy.VerifyProfile(config, acmeRef())
	if !errors.Is(err, model.ErrPolicy) || !strings.Contains(err.Error(), "name is acme in golaw.yml and other in the installed bundle") {
		t.Errorf("a bundle under acme whose stamp names other: %v", err)
	}
}
