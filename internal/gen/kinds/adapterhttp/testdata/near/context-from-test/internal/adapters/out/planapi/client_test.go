package planapi_test

import (
	"net/http"
	"testing"

	"example.com/slice/internal/adapters/out/planapi"
	"example.com/slice/internal/application/growth"
	"example.com/slice/internal/gensupport/tlstest"
)

func TestClientReadPlan(t *testing.T) {
	t.Parallel()
	server := tlstest.Server(t, tlstest.Route{Status: http.StatusNotFound})
	adapter := planapi.NewClient(planapi.ClientConfig{BaseURL: server.URL, Client: server.Client()})
	var account growth.AccountID
	if _, err := adapter.ReadPlan(t.Context(), account); err == nil {
		t.Fatal("a missing plan answered no error")
	}
}
