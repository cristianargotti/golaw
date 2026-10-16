package app

const notFound = "order not found"

// Missing names the text once and repeats a bare word.
func Missing(kind string) string {
	switch kind {
	case "order":
		return notFound
	case "line":
		return "missing"
	default:
		return "missing"
	}
}
