package httpapi

import (
	_ "embed"
	"net/http"

	"github.com/swaggest/swgui/v5emb"
)

//go:embed openapi/openapi.yaml
var openAPISpec []byte

func registerDocs(router *http.ServeMux) {
	router.HandleFunc("GET /openapi.yaml", func(response http.ResponseWriter, _ *http.Request) {
		response.Header().Set("Content-Type", "application/yaml; charset=utf-8")
		_, _ = response.Write(openAPISpec)
	})
	router.Handle("/docs/", v5emb.New("Crypto Scanner API", "/openapi.yaml", "/docs/"))
}
