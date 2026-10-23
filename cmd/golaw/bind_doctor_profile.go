package main

import (
	"context"
	"path/filepath"
	"time"

	"github.com/cristianargotti/golaw/internal/doctor"
	"github.com/cristianargotti/golaw/internal/policy"
	"github.com/cristianargotti/golaw/internal/profile"
)

// stampDate is the layout of the date stamp of profile.yml, and a day is
// the unit of the age doctor reports.
const (
	stampDate   = "2006-01-02"
	hoursPerDay = 24
)

// profileDoctor is the profile line of a repository (D10): the pin of the
// policy in force beside the bundle found for it under the config
// directory. When the policy does not load, the pin comes from golaw.yml
// at HEAD and the bundle is judged alone, so the line is red for a bundle
// fault only, in the loader's own words, which name the install command
// (A19).
func (b binding) profileDoctor(ctx context.Context, root string, now time.Time) doctor.Line {
	return doctor.AssessProfile(b.profileFacts(ctx, root, now))
}

// profileFacts are the facts of the pin and its bundle; core only has none.
func (b binding) profileFacts(ctx context.Context, root string, now time.Time) doctor.ProfileFacts {
	ref, pinned := b.pinInForce(ctx, root, now)
	if !pinned {
		return doctor.ProfileFacts{}
	}
	dirs, err := b.dirs()
	if err != nil {
		facts := pinFacts(ref)
		facts.Error = err.Error()
		return facts
	}
	return bundleFacts(dirs.Config, ref, now)
}

// pinInForce is the profile the policy in force pins, read from golaw.yml
// at HEAD when the policy does not load.
func (b binding) pinInForce(ctx context.Context, root string, now time.Time) (policy.ProfileRef, bool) {
	l, err := b.loaded(ctx, root, now)
	if err != nil {
		return headProfile(ctx, root)
	}
	if l.File.Profile == nil {
		return policy.ProfileRef{}, false
	}
	return *l.File.Profile, true
}

// bundleFacts are the facts of the bundle found for a pin: the loader's
// error, then the stamp of the directory the loader reads (the one
// LocateProfile found, else the current bundle of the name) when it is of
// the pinned source, and the digest recomputed over that directory. A
// stamp of another source is another bundle under the name, so the pinned
// one counts as not installed.
func bundleFacts(configDir string, ref policy.ProfileRef, now time.Time) doctor.ProfileFacts {
	facts := pinFacts(ref)
	if _, err := profile.Load(configDir, ref); err != nil {
		facts.Error = err.Error()
	}
	dir := bundleDir(configDir, ref)
	stamp, err := policy.LoadStamp(filepath.Join(dir, policy.StampFile))
	if err != nil || stamp.Source != ref.Source {
		return facts
	}
	facts.InstalledVersion, facts.InstalledDigest, facts.Date = stamp.Version, stamp.Digest, stamp.Date
	facts.Age = stampAge(stamp.Date, now)
	computed, err := profile.Digest(dir)
	if err != nil {
		return facts
	}
	facts.ComputedDigest = computed
	return facts
}

// bundleDir is the directory the loader reads for a pin.
func bundleDir(configDir string, ref policy.ProfileRef) string {
	located, err := policy.LocateProfile(configDir, ref)
	if err != nil {
		return profile.Dir(configDir, ref.Name)
	}
	return located.Dir
}

// stampAge is the whole days from a date stamp to the clock, never below
// zero; LoadStamp already refused a stamp whose date does not parse.
func stampAge(date string, now time.Time) int {
	day, err := time.Parse(stampDate, date)
	if err != nil {
		return 0
	}
	return max(0, int(now.Sub(day).Hours()/hoursPerDay))
}

// pinFacts are the facts golaw.yml gives.
func pinFacts(ref policy.ProfileRef) doctor.ProfileFacts {
	return doctor.ProfileFacts{Name: ref.Name, PinnedVersion: ref.Version, PinnedDigest: ref.Digest, Source: ref.Source}
}
