package vuln

import (
	"time"

	"github.com/cristianargotti/golaw/internal/model"
)

// Verdict splits the reachable advisories by the waivers in force on a day.
type Verdict struct {
	Waived  []Finding // covered by an unexpired waiver
	Refused []Finding // no waiver, or one whose last day has passed
}

// Judge applies the waivers (model.Waiver, expiry as policy.Expired reads
// it: in force through the whole of its last day, in UTC).
func Judge(found []Finding, waivers []model.Waiver, now time.Time) Verdict {
	var v Verdict
	for _, f := range found {
		if waived(waivers, f.ID, now) {
			v.Waived = append(v.Waived, f)
		} else {
			v.Refused = append(v.Refused, f)
		}
	}
	return v
}

// Expired answers the waivers whose last day has passed, in their order,
// which the scheduled lane refuses so the list only shrinks.
func Expired(waivers []model.Waiver, now time.Time) []model.Waiver {
	var out []model.Waiver
	for _, w := range waivers {
		if !inForce(w, now) {
			out = append(out, w)
		}
	}
	return out
}

// waived holds an advisory while any entry of its id is in force, so an
// entry that extends a waiver keeps it waived beside the one that lapsed.
func waived(waivers []model.Waiver, id string, now time.Time) bool {
	for _, w := range waivers {
		if w.ID == id && inForce(w, now) {
			return true
		}
	}
	return false
}

// inForce reads the last day of a waiver in UTC; a day that does not parse
// holds nothing, so a broken waiver never waives.
func inForce(w model.Waiver, now time.Time) bool {
	day, err := time.Parse(time.DateOnly, w.Until)
	return err == nil && now.UTC().Before(day.AddDate(0, 0, 1))
}
