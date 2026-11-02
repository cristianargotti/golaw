// Package genmap is the repository map an agent reads before it explores
// (M7.8): the layers, the ports with their methods, the adapters of each
// port, the use cases, the handlers, the contracts and the open holes. It
// is built from the snapshot and the manifest alone, with go/parser and no
// type loading, so it answers in the time of a hook.
package genmap
