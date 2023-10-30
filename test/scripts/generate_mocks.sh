#!/bin/sh

go install go.uber.org/mock/mockgen@v0.3.0

# Always create mock files into a "_mocks.go" file to be ignored in test coverage.
# See codecov.yml for more info 

mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/client/client_mocks.go -package=client github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client Client
mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/installer/installer_mocks.go -package=installer github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/installer InstallerSelector,Installer
mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/host/host_mocks.go -package=host github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host Manager
mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/elementalcli/runner_mocks.go -package=elementalcli github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/elementalcli Runner
mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/utils/runner_mocks.go -package=utils github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/utils CommandRunner
