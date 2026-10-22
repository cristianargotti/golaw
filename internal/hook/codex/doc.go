// Package codex holds the decoder and the encoder of the Codex CLI hook
// events: the envelope on stdin becomes a normalized model event, an
// apply_patch patch its candidates reconstructed in memory, and the result
// becomes the JSON answer, the stderr line and the exit code the host
// documents (specification section 8.2).
package codex
