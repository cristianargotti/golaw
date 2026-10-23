package main

import (
	"context"
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
// policy in force beside the bundle the load found. When the policy does
// not load, the pin comes from golaw.yml at HEAD and the bundle is loaded
// alone, so the line is red for a bundle error only, in the loader's own
// words, which name the install command (A19).
func (b binding) profileDoctor(ctx context.Context, root string, now time.Time) doctor.Line {
	return doctor.AssessProfile(b.profileFacts(ctx, root, now))
}

func (b binding) profileFacts(ctx context.Context, root string, now time.Time) doctor.ProfileFacts {
	l, err := b.loaded(ctx, root, now)
	if err != nil {
		return b.headProfileFacts(ctx, root, now)
	}
	if l.File.Profile == nil {
		return doctor.ProfileFacts{}
	}
	return stampFacts(*l.File.Profile, l.Stamp, now)
}

// headProfileFacts are the facts of the pin at HEAD when the policy does
// not load: the bundle loaded alone, its own error kept.
func (b binding) headProfileFacts(ctx context.Context, root string, now time.Time) doctor.ProfileFacts {
	ref, ok := headProfile(ctx, root)
	if !ok {
		return doctor.ProfileFacts{}
	}
	dirs, err := b.dirs()
	if err == nil {
		_, err = profile.Load(dirs.Config, ref)
	}
	if err != nil {
		facts := stampFacts(ref, nil, now)
		facts.Error = err.Error()
		return facts
	}
	stamp, err := policy.LoadStamp(policy.StampPath(dirs.Config, ref.Name))
	if err != nil {
		return stampFacts(ref, nil, now)
	}
	return stampFacts(ref, &stamp, now)
}

// stampFacts are the facts of a pin and of the stamp found for it, the
// age counted in whole days and never below zero.
func stampFacts(ref policy.ProfileRef, stamp *policy.Stamp, now time.Time) doctor.ProfileFacts {
	facts := doctor.ProfileFacts{Name: ref.Name, PinnedVersion: ref.Version, PinnedDigest: ref.Digest, Source: ref.Source}
	if stamp == nil {
		return facts
	}
	facts.InstalledVersion, facts.InstalledDigest, facts.Date = stamp.Version, stamp.Digest, stamp.Date
	if day, err := time.Parse(stampDate, stamp.Date); err == nil {
		facts.Age = max(0, int(now.Sub(day).Hours()/hoursPerDay))
	}
	return facts
}
