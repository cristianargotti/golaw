package policy_test

import "time"

// now is the clock every test runs under.
func now() time.Time { return time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC) }
