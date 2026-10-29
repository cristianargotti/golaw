package surface_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/cristianargotti/golaw/internal/surface"
)

// pipelineProperties is the shape of a Go pipeline's scanner properties
// (survey 1.1): placeholders in keys the scope never reads, the exclusion
// list, the coverage report.
const pipelineProperties = `sonar.projectKey=${GIT_REPOSITORY}
sonar.projectVersion=${GIT_REPOSITORY_TAG}
sonar.go.version=${GO_VERSION}
sonar.go.coverage.reportPaths=profile.cov
sonar.exclusions=**/*_test.go,integration/**,**/gen/**,**/testdata/**,examples/*
sonar.qualitygate.wait=true
`

// bs is one backslash: the escapes the subset refuses are composed at run
// time, so this file holds no unicode escape sequence a tool could decode.
const bs = `\`

func TestParsePropertiesReadsTheDeclaredSubset(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		data       string
		want       map[string][]string
		unmeasured []string
	}{
		"the pipeline shape": {
			data: pipelineProperties,
			want: map[string][]string{
				"sonar.go.coverage.reportPaths": {"profile.cov"},
				"sonar.exclusions": {
					"**/*_test.go", "integration/**", "**/gen/**", "**/testdata/**", "examples/*",
				},
			},
		},
		"blanks around an equals sign": {data: "sonar.sources = cmd, internal\n", want: map[string][]string{"sonar.sources": {"cmd", "internal"}}},
		"a colon separator":            {data: "sonar.tests: test\n", want: map[string][]string{"sonar.tests": {"test"}}},
		"a blank separator":            {data: "sonar.sources src\n", want: map[string][]string{"sonar.sources": {"src"}}},
		"a blank then an equals sign":  {data: "sonar.sources   =  src\n", want: map[string][]string{"sonar.sources": {"src"}}},
		"a continuation": {
			data: "sonar.exclusions=**/gen/**,\\\n    **/testdata/**\n",
			want: map[string][]string{"sonar.exclusions": {"**/gen/**", "**/testdata/**"}},
		},
		"a continuation line that starts with a hash": {
			data: "sonar.exclusions=a,\\\n#b\n",
			want: map[string][]string{"sonar.exclusions": {"a", "#b"}},
		},
		"a continuation at the end of the file": {data: "sonar.exclusions=a\\", want: map[string][]string{"sonar.exclusions": {"a"}}},
		"an escaped backslash ends no line":     {data: "sonar.sources=a\\\\\nsonar.tests=t\n", want: map[string][]string{"sonar.sources": {`a\`}, "sonar.tests": {"t"}}},
		"empty entries are dropped":             {data: "sonar.cpd.exclusions = a , , b \n", want: map[string][]string{"sonar.cpd.exclusions": {"a", "b"}}},
		"an empty value is an empty list":       {data: "sonar.sources=\n", want: map[string][]string{"sonar.sources": {}}},
		"CRLF line breaks":                      {data: "sonar.tests=test\r\nsonar.sources=src\r\n", want: map[string][]string{"sonar.tests": {"test"}, "sonar.sources": {"src"}}},
		"the last definition wins":              {data: "sonar.sources=a\nsonar.sources=b\n", want: map[string][]string{"sonar.sources": {"b"}}},
		"a later definition clears a placeholder": {
			data: "sonar.sources=${env.SRC}\nsonar.sources=b\n", want: map[string][]string{"sonar.sources": {"b"}},
		},
		"every read key": {
			data: "sonar.inclusions=**/*.ts\nsonar.test.inclusions=**/*.test.ts\nsonar.test.exclusions=e2e/**\nsonar.coverage.exclusions=cmd/**\n",
			want: map[string][]string{
				"sonar.inclusions": {"**/*.ts"}, "sonar.test.inclusions": {"**/*.test.ts"},
				"sonar.test.exclusions": {"e2e/**"}, "sonar.coverage.exclusions": {"cmd/**"},
			},
		},
		"an escaped comma still separates": {data: "sonar.sources=a\\,b\n", want: map[string][]string{"sonar.sources": {"a", "b"}}},
		"the control escapes": {
			data: "sonar.sources=a\\tb,c\\nd,e\\rf,g\\fh\n",
			want: map[string][]string{"sonar.sources": {"a\tb", "c\nd", "e\rf", "g\fh"}},
		},
		"comments (near)":                {data: "# sonar.sources=x\n! sonar.tests=y\n   # sonar.exclusions=z\n", want: map[string][]string{}},
		"a key the scope ignores (near)": {data: "sonar.projectName=Hub" + bs + "u00e9\nsonar.host.url=${HOST}\n", want: map[string][]string{}},
		"a placeholder in a read key": {
			data: "sonar.exclusions=${env.EXCLUDED}\n", want: map[string][]string{},
			unmeasured: []string{"sonar.exclusions: a ${...} placeholder golaw does not resolve"},
		},
		"a unicode escape in a read value": {
			data: "sonar.inclusions=" + bs + "u0073rc/**\n", want: map[string][]string{},
			unmeasured: []string{"sonar.inclusions: a " + bs + "u escape golaw does not decode"},
		},
		"a unicode escape in a read key": {
			data: "sonar." + bs + "u0073ources=src\n", want: map[string][]string{},
			unmeasured: []string{"sonar." + bs + "u0073ources: a " + bs + "u escape golaw does not decode"},
		},
		"a placeholder after a value clears it": {
			data: "sonar.tests=test\nsonar.tests=${T}\n", want: map[string][]string{},
			unmeasured: []string{"sonar.tests: a ${...} placeholder golaw does not resolve"},
		},
		"an escaped separator stays in the key (near)": {data: "sonar\\=sources=src\n", want: map[string][]string{}},
		"blank lines (near)":                           {data: "\n   \n\t\n", want: map[string][]string{}},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got := surface.ParseProperties([]byte(tc.data))
			if !maps.EqualFunc(got.Values, tc.want, slices.Equal) {
				t.Errorf("Values = %q, want %q", got.Values, tc.want)
			}
			if !slices.Equal(got.Unmeasured, tc.unmeasured) {
				t.Errorf("Unmeasured = %q, want %q", got.Unmeasured, tc.unmeasured)
			}
		})
	}
}
