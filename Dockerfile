# Build the manager binary
FROM golang:1.22 as builder
ARG TARGETOS
ARG TARGETARCH

WORKDIR /workspace

# Copy the go source
COPY go.mod go.mod
COPY go.sum go.sum
COPY cmd/manager/main.go cmd/manager/main.go
COPY api/ api/
COPY internal/ internal/

ARG TAG=v0.0.0
ARG COMMIT=""
ARG COMMITDATE=""

# Build
# the GOARCH has not a default value to allow the binary be built according to the host where the command
# was called. For example, if we call make docker-build in a local env which has the Apple Silicon M1 SO
# the docker BUILDPLATFORM arg will be linux/arm64 when for Apple x86 it will be linux/amd64. Therefore,
# by leaving it empty we can ensure that the container and binary shipped on it will have the same platform.
RUN CGO_ENABLED=0 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build \
    -ldflags "-w -s  \
    -X github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version.Version=$TAG  \
    -X github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version.Commit=$COMMIT  \
    -X github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version.CommitDate=$COMMITDATE"  \
    -a -o manager cmd/manager/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532

ENTRYPOINT ["/manager"]
