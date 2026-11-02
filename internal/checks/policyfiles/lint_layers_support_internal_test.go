package policyfiles

import (
	"reflect"
	"testing"
)

// TestInnerLayersKeepTheDefaultSupportOfAnEmptyKey mirrors the or of the
// lint asset: a generate.support written empty renders the default package,
// so the check must allow the same one instead of reporting it.
func TestInnerLayersKeepTheDefaultSupportOfAnEmptyKey(t *testing.T) {
	t.Parallel()
	if got, want := innerLayers(""), innerLayers("internal/gensupport"); !reflect.DeepEqual(got, want) {
		t.Fatalf("innerLayers(\"\") = %+v, want %+v", got, want)
	}
}
