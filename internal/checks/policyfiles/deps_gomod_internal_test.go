package policyfiles

import (
	"reflect"
	"strings"
	"testing"
)

func TestReadModule(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		data string
		want goModule
		fail bool
	}{
		{
			name: "second require block",
			data: "module example.test/app\n" +
				"require example.test/a v1.0.0\n" +
				"require (\n" +
				" example.test/b v1.2.0\n" +
				" example.test/c v1.0.0 // indirect\n" +
				")\n",
			want: goModule{
				path: "example.test/app",
				direct: []moduleRequirement{
					{path: "example.test/a", version: "v1.0.0", line: 2},
					{path: "example.test/b", version: "v1.2.0", line: 4},
				},
			},
		},
		{
			name: "indirect near miss",
			data: "module example.test/app\r\n" +
				"require example.test/a v1.0.0 // indirect\r\n",
			want: goModule{path: "example.test/app"},
		},
		{
			name: "both replace forms",
			data: "module example.test/app\n" +
				"replace example.test/a => ./local\n" +
				"replace (\n" +
				" example.test/b v1.0.0 => example.test/c v1.1.0\n" +
				")\n",
			want: goModule{
				path: "example.test/app",
				replaces: []moduleReplacement{
					{
						oldPath: "example.test/a",
						newPath: "./local",
						line:    2,
					},
					{
						oldPath: "example.test/b", oldVersion: "v1.0.0",
						newPath: "example.test/c", newVersion: "v1.1.0",
						line: 4,
					},
				},
			},
		},
		{
			name: "tool directive is not a requirement",
			data: "module example.test/app\n" +
				"go 1.27\n" +
				"tool example.test/tool/cmd/tool\n",
			want: goModule{path: "example.test/app"},
		},
		{name: "invalid version", data: "module example.test/app\nrequire example.test/a bad\n", fail: true},
		{name: "invalid syntax", data: "module example.test/app\nrequire (\n", fail: true},
		{name: "missing module", data: "go 1.27\n", fail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := readModule("go.mod", []byte(tc.data))
			if tc.fail {
				if err == nil || !strings.Contains(err.Error(), "go.mod") {
					t.Fatalf("error = %v, want a named module failure", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("module = %+v, want %+v", got, tc.want)
			}
		})
	}
}
