package planapi_test

import (
	"testing"

	"example.com/slice/internal/adapters/out/planapi"
	"example.com/slice/internal/gensupport/tlstest"
)

func TestClientDefaults(t *testing.T) {
	t.Parallel()
	server := tlstest.Server(t)
	if planapi.NewClient(planapi.ClientConfig{BaseURL: server.URL, Client: server.Client()}) == nil {
		t.Fatal("NewClient answered nil")
	}
}
