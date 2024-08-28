package main

import (
	"os"
	"testing"

	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/swaggest/openapi-go/openapi3"
	"github.com/swaggest/rest/gorillamux"
)

const outputFile = "elemental-openapi.yaml"

func TestGenerateOpenAPI(t *testing.T) {
	server := api.Server{}
	router := server.NewRouter()

	// Setup OpenAPI schema.
	refl := openapi3.NewReflector()
	refl.SpecSchema().SetTitle("Elemental API")
	refl.SpecSchema().SetVersion("v0.0.1")
	refl.SpecSchema().SetDescription(`This API can be used to interact with the Cluster API Elemental operator.<br />
	This API is for <b>Internal</b> use by the <a href="https://github.com/rancher-sandbox/cluster-api-provider-elemental/tree/main/cmd/agent">Elemental CAPI agent</a> and it's not supported for public use.<br />
	Use it at your own risk.<br />
	<br />
	The schemas are mapping the related <a href="https://github.com/rancher-sandbox/cluster-api-provider-elemental/tree/main/api/v1beta1">Elemental CAPI resources</a>.<br />`)

	// Walk the router with OpenAPI collector.
	c := gorillamux.NewOpenAPICollector(refl)

	if err := router.Walk(c.Walker); err != nil {
		t.Fatalf("Could not walk API routes: %s", err.Error())
	}

	// Get the resulting schema.
	if yaml, err := refl.Spec.MarshalYAML(); err != nil {
		t.Fatalf("Could not marshal OpenAPI YAML: %s", err.Error())
	} else {
		writeOpenAPISpecFile(t, yaml)
	}
}

func writeOpenAPISpecFile(t *testing.T, spec []byte) {
	t.Helper()

	f, err := os.Create(outputFile)
	if err != nil {
		t.Fatalf("Could not create file '%s': %s", outputFile, err.Error())
	}

	defer f.Close()

	_, err = f.Write(spec)
	if err != nil {
		t.Fatalf("Could not write file '%s': %s", outputFile, err.Error())
	}
}
