/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	"go.uber.org/zap/zapcore"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	infrastructurev1beta1 "github.com/rancher-sandbox/cluster-api-provider-elemental/api/v1beta1"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/api"
	"github.com/rancher-sandbox/cluster-api-provider-elemental/internal/controller"
	//+kubebuilder:scaffold:imports
)

// Defaults
const (
	defaultAPIPort = 9090
)

// Environment variables.
const (
	envEnableDebug       = "ELEMENTAL_ENABLE_DEBUG"
	envEnableDefaultCA   = "ELEMENTAL_ENABLE_DEFAULT_CA"
	envAPIEndpoint       = "ELEMENTAL_API_ENDPOINT"
	envAPIProtocol       = "ELEMENTAL_API_PROTOCOL"
	envAPITLSEnable      = "ELEMENTAL_API_ENABLE_TLS"
	envAPITLSCA          = "ELEMENTAL_API_TLS_CA"
	envAPITLSPrivateKey  = "ELEMENTAL_API_TLS_PRIVATE_KEY"
	envAPITLSCertificate = "ELEMENTAL_API_TLS_CERTIFICATE"
)

// Errors.
var (
	ErrElementalAPIEndpointNotSet      = errors.New("ELEMENTAL_API_ENDPOINT environment variable is not set")
	ErrElementalAPIProtocolNotSet      = errors.New("ELEMENTAL_API_PROTOCOL environment variable is not set")
	ErrElementalAPIProtocolUnsupported = errors.New("ELEMENTAL_API_PROTOCOL environment variable defines an unsupported protocol")
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(infrastructurev1beta1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme

	utilruntime.Must(clusterv1.AddToScheme(scheme))
}

func formatAPIURL() (*url.URL, error) {
	endpoint := os.Getenv(envAPIEndpoint)
	if len(envAPIEndpoint) == 0 {
		return nil, ErrElementalAPIEndpointNotSet
	}

	protocol := os.Getenv(envAPIProtocol)
	if len(protocol) == 0 {
		return nil, ErrElementalAPIProtocolNotSet
	}
	if protocol != "http" && protocol != "https" {
		return nil, fmt.Errorf("parsing protocol '%s': %w", protocol, ErrElementalAPIProtocolUnsupported)
	}

	endpointURL, err := url.Parse(fmt.Sprintf("%s://%s", protocol, endpoint))
	if err != nil {
		return nil, fmt.Errorf("formatting Elemental API URL: %w", err)
	}
	return endpointURL, nil
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")

	// Handle Debug flag
	debug := false
	if os.Getenv(envEnableDebug) == "true" {
		debug = true
	}
	opts := zap.Options{}
	if debug {
		opts = zap.Options{
			Development: true,
			Level:       zapcore.DebugLevel,
		}
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	ctx := ctrl.SetupSignalHandler()

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "a422e8b5.cluster.x-k8s.io",
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controller.ElementalHostReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ElementalHost")
		os.Exit(1)
	}
	if err = (&controller.ElementalMachineReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ElementalMachine")
		os.Exit(1)
	}
	if err = (&controller.ElementalMachineTemplateReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ElementalMachineTemplate")
		os.Exit(1)
	}
	if err = (&controller.ElementalClusterReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(ctx, mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ElementalCluster")
		os.Exit(1)
	}
	if err = (&controller.ElementalClusterTemplateReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ElementalClusterTemplate")
		os.Exit(1)
	}

	// Start RegistrationReconciler
	elementalAPIURL, err := formatAPIURL()
	if err != nil {
		setupLog.Error(err, "formatting Elemental API URL")
		os.Exit(1)
	}
	// Load the default CA if this behavior was enabled
	var defaultCACert string
	if os.Getenv(envEnableDefaultCA) == "true" {
		defaultCACertBytes, err := os.ReadFile(os.Getenv(envAPITLSCA))
		if err != nil {
			setupLog.Error(err, "reading Elemental API TLS CA certificate")
			os.Exit(1)
		}
		defaultCACert = string(defaultCACertBytes)
	}
	if err = (&controller.ElementalRegistrationReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		APIUrl:        elementalAPIURL,
		DefaultCACert: defaultCACert,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ElementalRegistration")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	// Start Elemental API
	privateKey := os.Getenv(envAPITLSPrivateKey)
	certificate := os.Getenv(envAPITLSCertificate)
	useTLS := os.Getenv(envAPITLSEnable) == "true"
	elementalAPIServer := api.NewServer(ctx, mgr.GetClient(), defaultAPIPort, useTLS, privateKey, certificate)
	go func() {
		if err := elementalAPIServer.Start(ctx); err != nil {
			setupLog.Error(err, "running Elemental API server")
			os.Exit(1)
		}
	}()

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
