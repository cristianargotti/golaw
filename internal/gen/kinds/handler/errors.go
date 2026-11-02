package handler

import "errors"

// errStub is a rendering of one operation that is not the one declaration
// per file the append needs; only a broken template reaches it.
var errStub = errors.New("handler stub is not one declaration")
