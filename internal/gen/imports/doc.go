// Package imports is the certain import fix the post hook applies in
// process (M7.9.3 item 2): it removes an import no selector names and adds
// one whose package name maps to exactly one known path. It never guesses,
// loads no package and runs no tool, so it fits the hook budget; whatever
// it cannot decide with certainty it reports and leaves as it is.
package imports
