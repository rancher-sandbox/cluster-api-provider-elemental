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

// Prefixes.
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
		logger:    log.FromContext(ctx),
	}
}

func (s *Server) Start() error {
	s.logger.Info("Starting Elemental API V1 Server")

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
		Handler:      router,
		Addr:         ":9090",
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	if err := s.httpServer.ListenAndServe(); err != nil {
		return fmt.Errorf("listening for TCP incoming connections: %w", err)
	}
	return nil
}

func (s *Server) Stop() error {
	if err := s.httpServer.Shutdown(s.context); err != nil {
		return fmt.Errorf("shutting down server: %w", err)
	}
	return nil
}
