// Package gen plans one generator run: it reads the contracts and the owned
// sources through the snapshot alone, renders each kind and answers the
// writes, the appends, the drift and the holes, so the post hook and the
// commands share one engine and apply.Files stays the only writer (M7.7).
package gen
