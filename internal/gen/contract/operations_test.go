package contract_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/cristianargotti/golaw/internal/gen/contract"
	"github.com/cristianargotti/golaw/internal/gen/genkind"
	"github.com/cristianargotti/golaw/internal/gentest"
)

const (
	serverPath  = "api/gen/server.gen.go"
	openapiPath = "api/openapi.yaml"
)

func TestOperationsReadTheStrictServerAndTheDocument(t *testing.T) {
	t.Parallel()
	got, err := growthReader(t).Operations(serverPath, openapiPath)
	if err != nil {
		t.Fatal(err)
	}
	want := []genkind.Operation{
		{Name: "FindPlan", Tag: "plans", Method: "POST", Route: "/plans", Line: 126},
		{Name: "GetPlan", Tag: "plans", Method: "GET", Route: "/plans/{id}", Line: 129},
		{Name: "ArchivePlan", Tag: "plans", Method: "POST", Route: "/plans/{id}/archive", Line: 132},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Operations = %+v\nwant         %+v", got, want)
	}
}

const bareServer = "package apigen\n\nimport \"context\"\n\n// StrictServerInterface represents all server handlers.\ntype StrictServerInterface interface {\n" +
	"\tListPlans(ctx context.Context, request ListPlansRequestObject) (ListPlansResponseObject, error)\n}\n"

func readerOf(t *testing.T, server, document string) contract.Reader {
	t.Helper()
	return contract.Reader{Snapshot: gentest.Snapshot(t, map[string][]byte{serverPath: []byte(server), openapiPath: []byte(document)})}
}

// Near rows: an operation without its route comment is matched by its
// operation id, and one without a tag answers the default tag.
func TestOperationsMatchTheOperationIDWhenTheCommentIsAbsent(t *testing.T) {
	t.Parallel()
	document := "openapi: 3.0.3\npaths:\n  /plans:\n    parameters: []\n    get:\n      operationId: list_plans\n"
	got, err := readerOf(t, bareServer, document).Operations(serverPath, openapiPath)
	want := []genkind.Operation{{Name: "ListPlans", Tag: "default", Method: "GET", Route: "/plans", Line: 7}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Operations = %+v, %v, want %+v", got, err, want)
	}
}

// Red rows: what the document does not hold, a server without the strict
// interface and a document that does not decode.
func TestOperationsRefuseWhatTheyCannotMatch(t *testing.T) {
	t.Parallel()
	tests := map[string]struct {
		server, document string
		want             error
		text             string
	}{
		"an operation the document lacks": {bareServer, "openapi: 3.0.3\npaths: {}\n", genkind.ErrCannotJudge, "ListPlans"},
		"no strict server interface":      {"package apigen\n", "openapi: 3.0.3\n", contract.ErrNoServer, serverPath},
		"a document that does not decode": {bareServer, "paths: [\n", nil, openapiPath},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := readerOf(t, tc.server, tc.document).Operations(serverPath, openapiPath)
			if err == nil || (tc.want != nil && !errors.Is(err, tc.want)) || !strings.Contains(err.Error(), tc.text) {
				t.Errorf("Operations = %v, want %v naming %q", err, tc.want, tc.text)
			}
		})
	}
}
