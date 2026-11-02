package adapterkit_test

import (
	"bytes"
	"testing"
)

const (
	httpx      = "internal/gensupport/zz_generated_httpx.go"
	httpxTest  = "internal/gensupport/zz_generated_httpx_internal_test.go"
	httpxErrs  = "internal/gensupport/zz_generated_errors_httpx.go"
	server     = "internal/gensupport/tlstest/zz_generated_server.go"
	serverTest = "internal/gensupport/tlstest/zz_generated_server_internal_test.go"
)

func TestSupportRendersTheHTTPHelpers(t *testing.T) {
	t.Parallel()
	files := supportFiles(t)
	for _, p := range []string{httpx, httpxTest, httpxErrs, server, serverTest} {
		content, ok := files[p]
		if !ok {
			t.Errorf("support lacks %s", p)
			continue
		}
		requireLaws(t, p, content)
	}
	tests := []struct {
		path string
		want []string
	}{
		{httpx, []string{
			"const DefaultTimeout = 10 * time.Second", "const MaxBody = 1 << 20",
			"func Client(timeout time.Duration) *http.Client",
			"type HTTPConfig struct {\n\tBaseURL string\n\tTimeout time.Duration\n\tClient  *http.Client\n}",
			"func (cfg HTTPConfig) Endpoint() (string, *http.Client)", `strings.TrimSuffix(cfg.BaseURL, "/")`,
			"func Get(ctx context.Context, client *http.Client, target string) ([]byte, int, error)",
			"func Send(ctx context.Context, client *http.Client, method, target string, body []byte) ([]byte, int, error)",
			"func Status(code int, want ...int) error", "func Decode(body []byte, v any) error",
			"http.NoBody", "io.LimitReader(response.Body, MaxBody)",
		}},
		{httpxErrs, []string{"var ErrStatus = errors.New(", "var ErrDecode = errors.New("}},
		{server, []string{
			"func Server(tb testing.TB, routes ...Route) *httptest.Server",
			"func Serve(tb testing.TB, script *Routes) *httptest.Server",
			"func (r *Routes) Add(route Route)", "httptest.NewTLSServer",
		}},
	}
	for _, tc := range tests {
		for _, want := range tc.want {
			if !bytes.Contains(files[tc.path], []byte(want)) {
				t.Errorf("%s lacks %q", tc.path, want)
			}
		}
	}
}

func TestSupportHTTPHelpersPassTheScan(t *testing.T) {
	t.Parallel()
	requireScanClean(t, supportFiles(t), httpx, httpxTest, httpxErrs, server, serverTest)
}

func TestSupportHTTPHelpersCoverEveryLine(t *testing.T) {
	t.Parallel()
	requireCovered(t, supportFiles(t), "./internal/gensupport/...", "zz_generated_httpx.go", "tlstest/zz_generated_server.go")
}
