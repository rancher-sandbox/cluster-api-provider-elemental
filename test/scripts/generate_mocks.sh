#!/bin/sh

go install go.uber.org/mock/mockgen@v0.3.0

# Always create mock files into a "_mocks.go" file to be ignored in test coverage.
# See codecov.yml for more info 

mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/client/client_mocks.go -package=client github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/client Client
mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/host/installer_mocks.go -package=host github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/host InstallerSelector,Installer
mockgen -copyright_file=hack/boilerplate.go.txt -destination=internal/agent/hostname/hostname_mocks.go -package=hostname github.com/rancher-sandbox/cluster-api-provider-elemental/internal/agent/hostname Manager
