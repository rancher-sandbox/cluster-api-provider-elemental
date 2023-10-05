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
	Prefix   = "/elemental"
	PrefixV1 = "/v1"
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

func (s *Server) NewRouter() *mux.Router {
	router := mux.NewRouter()
	elementalV1 := router.PathPrefix(fmt.Sprintf("%s%s", Prefix, PrefixV1)).Subrouter()

	elementalV1.Handle("/namespaces/{namespace}/registrations/{registrationName}",
		NewGetElementalRegistrationHandler(s.logger, s.k8sClient)).
		Methods(http.MethodGet)

	elementalV1.Handle("/namespaces/{namespace}/registrations/{registrationName}/hosts",
		NewPostElementalHostHandler(s.logger, s.k8sClient)).
		Methods(http.MethodPost)

	elementalV1.Handle("/namespaces/{namespace}/registrations/{registrationName}/hosts/{hostName}",
		NewPatchElementalHostHandler(s.logger, s.k8sClient)).
		Methods(http.MethodPatch)

	elementalV1.Handle("/namespaces/{namespace}/registrations/{registrationName}/hosts/{hostName}/bootstrap",
		NewGetElementalHostBootstrapHandler(s.logger, s.k8sClient)).
		Methods(http.MethodGet)

	return router
}

func (s *Server) Start() error {
	s.logger.Info("Starting Elemental API V1 Server")

	s.httpServer = &http.Server{
		Handler:      s.NewRouter(),
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
