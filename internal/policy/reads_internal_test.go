package policy

import (
	"errors"
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

// TestBatchBlobReadsTheAnswerOfOneName reads what git cat-file --batch
// answers for golaw.yml at HEAD: the blob with its id, a name that resolves
// to nothing as absent, and every other answer as a policy error, so a tree
// or an object git cannot show never reads as a missing file.
func TestBatchBlobReadsTheAnswerOfOneName(t *testing.T) {
	t.Parallel()
	data, oid, found, err := batchBlob([]byte("abc blob 3\nxyz\n"))
	if err != nil || !found || oid != "abc" || string(data) != "xyz" {
		t.Fatalf("a blob = %q, %q, %v, %v", data, oid, found, err)
	}
	if _, _, found, err := batchBlob([]byte(headName + " missing\n")); err != nil || found {
		t.Fatalf("a missing name = %v, %v", found, err)
	}
	for name, answer := range map[string]string{
		"a tree":                 "abc tree 30\n",
		"a short blob":           "abc blob 9\nxyz\n",
		"a size that is no size": "abc blob x\nxyz\n",
		"an object git lacks":    "abc missing\n",
		"an unknown answer":      "garbage\n",
	} {
		if _, _, found, err := batchBlob([]byte(answer)); !errors.Is(err, model.ErrPolicy) || found {
			t.Errorf("%s = %v, %v, want ErrPolicy", name, found, err)
		}
	}
}
