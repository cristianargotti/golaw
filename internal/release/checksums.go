package release

import (
	"strings"
)

// ChecksumOf answers the lower case hex of the asset line of checksums.txt,
// "<hex>  <name>" as goreleaser writes it (a leading asterisk on the name,
// the binary mode of sha256sum, is accepted); false when no line names the
// asset. A line without two fields is skipped, never an error, because the
// signed file is judged by its signature and this reads one line of it.
func ChecksumOf(checksums []byte, asset string) (string, bool) {
	for line := range strings.SplitSeq(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if strings.TrimPrefix(fields[1], "*") == asset {
			return strings.ToLower(fields[0]), true
		}
	}
	return "", false
}
