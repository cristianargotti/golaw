package app

// Missing answers the same text three times.
func Missing(kind string) string {
	switch kind {
	case "order":
		return "order not found"
	case "line":
		return "order not found"
	default:
		return "order not found"
	}
}
