package model_test

import (
	"testing"

	"github.com/cristianargotti/golaw/internal/model"
)

func TestDefaultGenerateIsTheAddendumLayout(t *testing.T) {
	t.Parallel()
	want := model.GeneratePolicy{
		Layout: model.GenerateLayout{
			Port: "internal/application/{area}", Usecase: "internal/application/{area}",
			AdapterSQL: "internal/adapters/out/{package}", AdapterHTTP: "internal/adapters/out/{package}",
			Handler: "internal/adapters/in/{package}", Wiring: "cmd/{main}",
		},
		Support: "internal/gensupport",
		ThirdParty: model.GeneratePaths{
			SQLCOut: "internal/adapters/out/postgres/sqlcgen", OpenAPI: "api/openapi.yaml", OpenAPIOut: "api/gen",
		},
	}
	if got := model.DefaultGenerate(); got != want {
		t.Errorf("DefaultGenerate() = %+v, want %+v", got, want)
	}
	if got := model.CoreDefaults().Generate; got != want || got.Declared {
		t.Errorf("CoreDefaults().Generate = %+v, want the defaults undeclared", got)
	}
}
