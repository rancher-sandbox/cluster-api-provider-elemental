package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
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
	context     context.Context
	port        uint
	k8sClient   client.Client
	httpServer  *http.Server
	logger      logr.Logger
	useTLS      bool
	privKey     string
	certificate string
}

func NewServer(ctx context.Context, k8sClient client.Client, port uint, useTLS bool, privKey string, certificate string) *Server {
	return &Server{
		context:     ctx,
		port:        port,
		k8sClient:   k8sClient,
		logger:      log.FromContext(ctx),
		useTLS:      useTLS,
		privKey:     privKey,
		certificate: certificate,
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
		NewDeleteElementalHostHandler(s.logger, s.k8sClient)).
		Methods(http.MethodDelete)

	elementalV1.Handle("/namespaces/{namespace}/registrations/{registrationName}/hosts/{hostName}",
		NewPatchElementalHostHandler(s.logger, s.k8sClient)).
		Methods(http.MethodPatch)

	elementalV1.Handle("/namespaces/{namespace}/registrations/{registrationName}/hosts/{hostName}/bootstrap",
		NewGetElementalHostBootstrapHandler(s.logger, s.k8sClient)).
		Methods(http.MethodGet)

	return router
}

func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting Elemental API V1 Server")

	s.httpServer = &http.Server{
		Handler:      s.NewRouter(),
		Addr:         fmt.Sprintf(":%d", s.port),
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  30 * time.Second,
	}

	go func() {
		var err error
		if s.useTLS {
			err = s.httpServer.ListenAndServeTLS(s.certificate, s.privKey)
		} else {
			err = s.httpServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			s.logger.Error(err, "FATAL: listening for TCP incoming connections")
			os.Exit(1)
		}
	}()
	<-ctx.Done()

	s.logger.Info("Shutting down Elemental API V1 Server")
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Error(err, "shutting down http server")
	}
	return nil
}
