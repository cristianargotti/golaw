package vuln

import "errors"

// ErrStream marks a govulncheck stream that came from no complete scan: one
// without the config message govulncheck writes first, or one that does not
// decode to its end, so a scan that failed never reads as clean.
var ErrStream = errors.New("govulncheck stream is no complete scan")
