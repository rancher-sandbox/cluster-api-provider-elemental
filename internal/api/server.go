package api

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-logr/logr"
	"github.com/gorilla/mux"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	PrefixAPI = "/elemental"
	PrefixV1  = "/v1"
)

type Server struct {
	context    context.Context
	k8sClient  client.Client
	httpServer *http.Server
	logger     logr.Logger
}

func NewServer(ctx context.Context, k8sClient client.Client) *Server {
	return &Server{
		context:   ctx,
		k8sClient: k8sClient,
	}
}

func (s *Server) Start() error {
	logger := log.FromContext(s.context)
	logger.Info("Starting Elemental API V1 Server")

	router := mux.NewRouter()
	elementalV1 := router.PathPrefix(fmt.Sprintf("%s%s", PrefixAPI, PrefixV1)).Subrouter()

	elementalV1.Path("/namespaces/{namespace}/registrations/{registrationName}").
		Methods("GET").
		HandlerFunc(s.GetMachineRegistration) // TODO: Wrap me with RegistrationToken auth handler

	elementalV1.Path("/namespaces/{namespace}/registrations/{registrationName}/hosts").
		Methods("POST").
		HandlerFunc(s.PostMachineHost) // TODO: Wrap me with RegistrationToken + Host auth handler

	elementalV1.Path("/namespaces/{namespace}/registrations/{registrationName}/hosts/{hostName}").
		Methods("PATCH").
		HandlerFunc(s.PatchMachineHost) // TODO: Wrap me with RegistrationToken + Host auth handler

	elementalV1.Path("/namespaces/{namespace}/registrations/{registrationName}/hosts/{hostName}/bootstrap").
		Methods("GET").
		HandlerFunc(s.GetMachineHostBootstrap) // TODO: Wrap me with RegistrationToken + Host auth handler

	s.httpServer = &http.Server{
		Handler: router,
		Addr:    ":9090",
		// Good practice: enforce timeouts for servers you create!
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop() error {
	return s.httpServer.Shutdown(s.context)
}
