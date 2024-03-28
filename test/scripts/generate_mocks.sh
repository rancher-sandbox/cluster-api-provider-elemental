#!/bin/sh
set -e

go install go.uber.org/mock/mockgen@v0.3.0

# Always create mock files into a "_mocks.go" file to be ignored in test coverage.
# See codecov.yml for more info 

mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/client/client_mocks.go -package=client github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client Client
mockgen -copyright_file=hack/boilerplate.go.txt -destination=pkg/agent/osplugin/plugin_mocks.go -package=osplugin github.com/rancher-sandbox/cluster-api-provider-elemental/pkg/agent/osplugin Loader,Plugin
mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/hostname/hostname_mocks.go -package=hostname github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/hostname Formatter
mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/host/host_mocks.go -package=host github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host Manager
mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/elementalcli/runner_mocks.go -package=elementalcli github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/elementalcli Runner
mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/elementalcli/device_selector_mocks.go -package=elementalcli github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/elementalcli DeviceSelectorHandler
mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/utils/runner_mocks.go -package=utils github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils CommandRunner
mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/identity/identity_mocks.go -package=identity github.com/rancher-sandbox/cluster-api-provider-elemental/internal/identity Manager,Identity
