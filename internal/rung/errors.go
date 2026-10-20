package rung

import "errors"

// ErrScan marks a govulncheck run that answered an exit status without a
// verdict, as a scanner that fails or a database it cannot reach answers;
// the pre-push rung names it and lets the push go (A6).
var ErrScan = errors.New("govulncheck gave no verdict")
