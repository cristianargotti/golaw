package source

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestBatchBlobsReadsTheAnswersOfCatFile reads the answers git cat-file
// --batch gives: a blob as its bytes, a missing or ambiguous name, an
// object of another type and a blob over the read bound as nil, in the
// order asked; an answer cut short or a header it cannot read is an error.
func TestBatchBlobsReadsTheAnswersOfCatFile(t *testing.T) {
	t.Parallel()
	big := strconv.Itoa(model.MaxReadBytes + 1)
	out := "aaaa blob 5\nhello\n" + "tree:gone.md missing\n" + "abc ambiguous\n" +
		"bbbb tree 3\nxyz\n" + "cccc blob " + big + "\n" + strings.Repeat("x", model.MaxReadBytes+1) + "\n"
	blobs, err := batchBlobs([]byte(out), 5)
	if err != nil || len(blobs) != 5 || string(blobs[0]) != "hello" || blobs[1] != nil || blobs[2] != nil || blobs[3] != nil || blobs[4] != nil {
		t.Fatalf("blobs %q, %v", blobs, err)
	}
	for _, broken := range []string{"aaaa blob 5\nhel", "aaaa blob five\nhello\n", "aaaa blob\nhello\n", "no line feed"} {
		if _, err := batchBlobs([]byte(broken), 1); !errors.Is(err, ErrGit) {
			t.Errorf("%q answered %v", broken, err)
		}
	}
}
