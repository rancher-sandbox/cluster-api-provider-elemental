GIT_COMMIT?=$(shell git rev-parse HEAD)
GIT_COMMIT_SHORT?=$(shell git rev-parse --short HEAD)
GIT_TAG?=$(shell git describe --abbrev=0 --tags 2>/dev/null || echo "v0.0.0" )
GIT_COMMIT_DATE?=$(shell git show -s --format='%cI' HEAD)
# Image URL to use all building/pushing image targets
IMG_NAME ?= ghcr.io/rancher-sandbox/cluster-api-provider-elemental
IMG_TAG ?= latest
IMG = ${IMG_NAME}:${IMG_TAG}
# Image URL to use all building/pushing image targets
IMG_NAME_AGENT ?= ghcr.io/rancher-sandbox/cluster-api-provider-elemental/agent
IMG_TAG_AGENT ?= latest
IMG_AGENT = ${IMG_NAME_AGENT}:${IMG_TAG_AGENT}
# ENVTEST_K8S_VERSION refers to the version of kubebuilder assets to be downloaded by envtest binary.
# See: https://storage.googleapis.com/kubebuilder-tools
ENVTEST_K8S_VERSION = 1.30.0
# Install with: go install github.com/onsi/ginkgo/v2/ginkgo
# See: https://github.com/onsi/ginkgo
GINKGO_VER := v2.20.1
# See: https://github.com/golangci/golangci-lint
GOLANCI_LINT_VER := v1.60.3
# Tool Versions
# See: https://github.com/kubernetes-sigs/kustomize
KUSTOMIZE_VERSION ?= v5.4.3
# See: https://github.com/kubernetes-sigs/controller-tools
CONTROLLER_TOOLS_VERSION ?= v0.16.1
# CAPI version used for test CRDs
CAPI_VERSION?=$(shell grep "sigs.k8s.io/cluster-api" go.mod | awk '{print $$NF}')
# Dev Image building
KUBEADM_READY_OS ?= ""
ELEMENTAL_TOOLKIT_IMAGE ?= ghcr.io/rancher/elemental-toolkit/elemental-cli:nightly
ELEMENTAL_AGENT_IMAGE ?= ghcr.io/rancher-sandbox/cluster-api-provider-elemental/agent:latest
ELEMENTAL_OS_IMAGE?=docker.io/local/elemental-capi-os:dev 
ELEMENTAL_ISO_IMAGE?=docker.io/local/elemental-capi-iso:dev 

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

CROSS_COMPILER ?= aarch64-linux-gnu-gcc 
LDFLAGS := -w -s
LDFLAGS += -X "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version.Version=${GIT_TAG}"
LDFLAGS += -X "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version.Commit=${GIT_COMMIT}"
LDFLAGS += -X "github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version.CommitDate=${GIT_COMMIT_DATE}"

ABS_TOOLS_DIR :=  $(abspath bin/)
GO_INSTALL := ./test/scripts/go_install.sh

GINKGO := $(ABS_TOOLS_DIR)/ginkgo-$(GINKGO_VER)
GINKGO_PKG := github.com/onsi/ginkgo/v2/ginkgo

$(GINKGO):
	GOBIN=$(ABS_TOOLS_DIR) $(GO_INSTALL) $(GINKGO_PKG) ginkgo $(GINKGO_VER)

.PHONY: all
all: build-provider

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk commands is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: manifests
manifests: controller-gen ## Generate WebhookConfiguration, ClusterRole and CustomResourceDefinition objects.
	$(CONTROLLER_GEN) rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases

.PHONY: generate
generate: controller-gen ## Generate code containing DeepCopy, DeepCopyInto, and DeepCopyObject method implementations.
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

.PHONY: generate-mocks
generate-mocks:  
	./test/scripts/generate_mocks.sh

.PHONY: openapi
openapi: ## Generate Elemental OpenAPI specs
	go test -v -run ^TestGenerateOpenAPI$ 

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: manifests generate fmt vet envtest generate-mocks $(GINKGO) ## Run tests.
	KUBEBUILDER_ASSETS="$(shell $(ENVTEST) use $(ENVTEST_K8S_VERSION) --bin-dir $(LOCALBIN) -p path)" $(GINKGO) -v -r --trace --race --covermode=atomic --coverprofile=coverage.out $(GINKGO_EXTRA_ARGS) --coverpkg=github.com/rancher-sandbox/cluster-api-provider-elemental/... ./internal/... ./cmd/... ./pkg/...

##@ Build
.PHONY: build-agent
build-agent: fmt vet ## Build agent binary for local architecture.
	CGO_ENABLED=1 go build -ldflags '$(LDFLAGS)' -o bin/elemental-agent main.go

# This does depend on cross compilation library, for example: cross-aarch64-gcc13
.PHONY: build-agent-all
build-agent-all: fmt vet ## Build agent binary for all architectures.
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags '$(LDFLAGS)' -o bin/elemental_agent_linux_amd64 main.go
	CC=$(CROSS_COMPILER) CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -ldflags '$(LDFLAGS)' -o bin/elemental_agent_linux_arm64 main.go

.PHONY: build-manager
build-manager: manifests generate fmt vet ## Build manager binary.
	go build -ldflags '$(LDFLAGS)' -o bin/manager cmd/manager/main.go

.PHONY: build-plugins
build-plugins: fmt vet
	CGO_ENABLED=1 go build -buildmode=plugin -o bin/elemental.so internal/agent/plugin/elemental/elemental.go
	CGO_ENABLED=1 go build -buildmode=plugin -o bin/dummy.so internal/agent/plugin/dummy/dummy.go

# This does depend on cross compilation library, for example: cross-aarch64-gcc13
.PHONY: build-plugins-all
build-plugins-all: generate fmt vet
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildmode=plugin -o bin/elemental_amd64.so internal/agent/plugin/elemental/elemental.go
	CC=$(CROSS_COMPILER) CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -buildmode=plugin -o bin/elemental_arm64.so internal/agent/plugin/elemental/elemental.go
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -buildmode=plugin -o bin/dummy_amd64.so internal/agent/plugin/dummy/dummy.go
	CC=$(CROSS_COMPILER) CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build -buildmode=plugin -o bin/dummy_arm64.so internal/agent/plugin/dummy/dummy.go

.PHONY: run
run: manifests generate fmt vet ## Run a controller from your host.
	go run ./cmd/manager/main.go

# If you wish built the manager image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64 ). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image with the manager.
	$(CONTAINER_TOOL) build \
		--build-arg "TAG=${GIT_TAG}" \
		--build-arg "COMMIT=${GIT_COMMIT}" \
		--build-arg "COMMITDATE=${GIT_COMMIT_DATE}" \
		-t ${IMG} .

.PHONY: docker-build-agent
docker-build-agent: ## Build docker image with the elemental-agent and plugins.
	$(CONTAINER_TOOL) build \
		--build-arg "TAG=${GIT_TAG}" \
		--build-arg "COMMIT=${GIT_COMMIT}" \
		--build-arg "COMMITDATE=${GIT_COMMIT_DATE}" \
		-t ${IMG_AGENT} -f Dockerfile.agent .

# PLATFORMS defines the target platforms for  the manager image be build to provide support to multiple
# architectures. (i.e. make docker-buildx IMG=myregistry/mypoperator:0.0.1). To use this option you need to:
# - able to use docker buildx . More info: https://docs.docker.com/build/buildx/
# - have enable BuildKit, More info: https://docs.docker.com/develop/develop-images/build_enhancements/
# - be able to push the image for your registry (i.e. if you do not inform a valid value via IMG=<myregistry/image:<tag>> then the export will fail)
# To properly provided solutions that supports more than one platform you should use this option.
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
.PHONY: docker-buildx
docker-buildx: test ## Build and push docker image for the manager for cross-platform support
	# copy existing Dockerfile and insert --platform=${BUILDPLATFORM} into Dockerfile.cross, and preserve the original Dockerfile
	sed -e '1 s/\(^FROM\)/FROM --platform=\$$\{BUILDPLATFORM\}/; t' -e ' 1,// s//FROM --platform=\$$\{BUILDPLATFORM\}/' Dockerfile > Dockerfile.cross
	- $(CONTAINER_TOOL) buildx create --name project-v3-builder
	$(CONTAINER_TOOL) buildx use project-v3-builder
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
	- $(CONTAINER_TOOL) buildx rm project-v3-builder
	rm Dockerfile.cross

##@ Deployment

ifndef ignore-not-found
  ignore-not-found = false
endif

.PHONY: install
install: manifests kustomize ## Install CRDs into the K8s cluster specified in ~/.kube/config.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) apply -f -

.PHONY: uninstall
uninstall: manifests kustomize ## Uninstall CRDs from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/crd | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

.PHONY: deploy
deploy: manifests kustomize ## Deploy controller to the K8s cluster specified in ~/.kube/config.
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | $(KUBECTL) apply -f -

.PHONY: undeploy
undeploy: ## Undeploy controller from the K8s cluster specified in ~/.kube/config. Call with ignore-not-found=true to ignore resource not found errors during deletion.
	$(KUSTOMIZE) build config/default | $(KUBECTL) delete --ignore-not-found=$(ignore-not-found) -f -

##@ Build Dependencies

## Location to install dependencies to
LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	mkdir -p $(LOCALBIN)

## Tool Binaries
KUBECTL ?= kubectl
KUSTOMIZE ?= $(LOCALBIN)/kustomize
CONTROLLER_GEN ?= $(LOCALBIN)/controller-gen
ENVTEST ?= $(LOCALBIN)/setup-envtest

.PHONY: kustomize
kustomize: $(KUSTOMIZE) ## Download kustomize locally if necessary. If wrong version is installed, it will be removed before downloading.
$(KUSTOMIZE): $(LOCALBIN)
	@if test -x $(LOCALBIN)/kustomize && ! $(LOCALBIN)/kustomize version | grep -q $(KUSTOMIZE_VERSION); then \
		echo "$(LOCALBIN)/kustomize version is not expected $(KUSTOMIZE_VERSION). Removing it before installing."; \
		rm -rf $(LOCALBIN)/kustomize; \
	fi
	test -s $(LOCALBIN)/kustomize || GOBIN=$(LOCALBIN) GO111MODULE=on go install sigs.k8s.io/kustomize/kustomize/v5@$(KUSTOMIZE_VERSION)

.PHONY: controller-gen
controller-gen: $(CONTROLLER_GEN) ## Download controller-gen locally if necessary. If wrong version is installed, it will be overwritten.
$(CONTROLLER_GEN): $(LOCALBIN)
	test -s $(LOCALBIN)/controller-gen && $(LOCALBIN)/controller-gen --version | grep -q $(CONTROLLER_TOOLS_VERSION) || \
	GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-tools/cmd/controller-gen@$(CONTROLLER_TOOLS_VERSION)

.PHONY: envtest
envtest: $(ENVTEST) ## Download envtest-setup locally if necessary.
$(ENVTEST): $(LOCALBIN)
	test -s $(LOCALBIN)/setup-envtest || GOBIN=$(LOCALBIN) go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

.PHONY: kind-load
kind-load: docker-build
	kind load docker-image ${IMG} --name elemental-capi-management

.PHONY: generate-infra-yaml
generate-infra-yaml:kustomize manifests # Generate infrastructure-components.yaml for the provider
	$(KUSTOMIZE) build config/default > infrastructure-elemental/v0.0.0/infrastructure-components.yaml
	sed -i "s/IMAGE_TAG/${IMG_TAG}/g" infrastructure-elemental/v0.0.0/infrastructure-components.yaml

.PHONY: lint
lint: ## See: https://golangci-lint.run/usage/linters/
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANCI_LINT_VER)
	golangci-lint run -v --timeout 10m

AGENT_CONFIG_FILE?="iso/config/example-config.yaml"

.PHONY: build-os
build-os: 
ifeq ($(AGENT_CONFIG_FILE),"iso/config/example-config.yaml")
	@echo "No AGENT_CONFIG_FILE set, using the default one at ${AGENT_CONFIG_FILE}"
endif
	$(CONTAINER_TOOL) build \
		--build-arg "TAG=${GIT_TAG}" \
		--build-arg "COMMIT=${GIT_COMMIT}" \
		--build-arg "COMMITDATE=${GIT_COMMIT_DATE}" \
		--build-arg "AGENT_CONFIG_FILE=${AGENT_CONFIG_FILE}" \
		--build-arg "KUBEADM_READY=${KUBEADM_READY_OS}" \
		--build-arg "ELEMENTAL_TOOLKIT=${ELEMENTAL_TOOLKIT_IMAGE}" \
		--build-arg "ELEMENTAL_AGENT=${ELEMENTAL_AGENT_IMAGE}" \
		-t ${ELEMENTAL_OS_IMAGE} -f Dockerfile.os .

.PHONY: build-iso
build-iso: build-os
	$(CONTAINER_TOOL) build \
			--build-arg ELEMENTAL_OS_IMAGE=${ELEMENTAL_OS_IMAGE} \
			-t ${ELEMENTAL_ISO_IMAGE} \
			-f Dockerfile.iso .
	$(CONTAINER_TOOL) run -v ./iso:/iso \
			--entrypoint cp ${ELEMENTAL_ISO_IMAGE} \
			-r /elemental-iso/. /iso

.PHONY: update-test-capi-crds
update-test-capi-crds: 
# These files can not be included when vendoring, but we need them to start the controller test suite
	wget -O test/capi-crds/cluster.x-k8s.io_clusters.yaml https://raw.githubusercontent.com/kubernetes-sigs/cluster-api/${CAPI_VERSION}/config/crd/bases/cluster.x-k8s.io_clusters.yaml
	wget -O test/capi-crds/cluster.x-k8s.io_machines.yaml https://raw.githubusercontent.com/kubernetes-sigs/cluster-api/${CAPI_VERSION}/config/crd/bases/cluster.x-k8s.io_machines.yaml

.PHONY: vendor
vendor:
	go mod tidy
	go mod vendor

ALL_VERIFY_CHECKS = manifests generate openapi vendor
.PHONY: verify
verify: $(addprefix verify-,$(ALL_VERIFY_CHECKS))

.PHONY: verify-manifests
verify-manifests: manifests
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make manifests"; exit 1; \
	fi

.PHONY: verify-openapi
verify-openapi: openapi
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make openapi"; exit 1; \
	fi

.PHONY: verify-generate
verify-generate: generate
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi

.PHONY: verify-generate-infra-yaml
verify-generate-infra-yaml: generate-infra-yaml
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make generate-infra-yaml"; exit 1; \
	fi

.PHONY: verify-vendor
verify-vendor: vendor
	@if !(git diff --quiet HEAD); then \
		git diff; \
		echo "generated files are out of date, run make generate"; exit 1; \
	fi
