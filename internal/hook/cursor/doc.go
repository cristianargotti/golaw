// Package cursor holds the decoder and the encoder of the Cursor hook
// events, a compatibility preview: the envelope on stdin becomes a
// normalized model event and the result becomes the JSON answer the host
// documents, printed for every event because no output counts as a failure
// there (specification section 8.2).
package cursor
