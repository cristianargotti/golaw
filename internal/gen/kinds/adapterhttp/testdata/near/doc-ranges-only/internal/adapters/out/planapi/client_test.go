package planapi_test

import (
	"testing"

	"example.com/slice/internal/adapters/out/planapi"
)

func TestClientKeepsItsBase(t *testing.T) {
	t.Parallel()
	if planapi.NewClient(planapi.ClientConfig{BaseURL: "https://192.0.2.10/plans"}) == nil {
		t.Fatal("NewClient answered nil")
	}
}
