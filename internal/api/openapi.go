package api

import (
	"net/http"

	"github.com/swaggest/openapi-go"
)

type OpenAPIDecoratedHandler interface {
	SetupOpenAPIOperation(oc openapi.OperationContext) error
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

func WithDecoration(description string, contentType string, httpStatus int) func(cu *openapi.ContentUnit) {
	return func(cu *openapi.ContentUnit) {
		cu.Description = description
		cu.ContentType = contentType
		cu.HTTPStatus = httpStatus
	}
}
