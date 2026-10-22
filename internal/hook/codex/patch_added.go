package codex

// patchAdded is the added lines of each hunk of a patch, in hunk order:
// the plus lines of an Add File and of every chunk of an Update File, none
// for a Delete File. It is read after ParsePatch accepted the text, so a
// text the grammar refuses answers nothing; the lines keep their plus
// provenance, which the chunk shape loses, because a chunk that does not
// apply is judged on those lines alone (D3).
func patchAdded(text string) [][]string {
	_, added, err := parseDoc(text)
	if err != nil {
		return nil
	}
	return added
}
