package holes

import "errors"

// ErrDeclared marks a stub whose name the owned file already declares: the
// code there is the agent's, so the engine never replaces or duplicates it.
var ErrDeclared = errors.New("declaration exists in the owned file")

// ErrAppend marks an edit the engine cannot make by appending alone, so an
// owned file is never left half edited or rewritten.
var ErrAppend = errors.New("append only edit refused")
