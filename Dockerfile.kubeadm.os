ARG ELEMENTAL_TOOLKIT=ghcr.io/rancher/elemental-toolkit/elemental-cli:v2.0.0

FROM opensuse/leap:15.5 as AGENT

# Install Go 1.22
RUN zypper install -y wget tar gzip gcc
RUN wget https://go.dev/dl/go1.22.0.linux-amd64.tar.gz
RUN tar -C /usr/local -xzf go1.22.0.linux-amd64.tar.gz
ENV PATH "$PATH:/usr/local/go/bin"

# Copy the Go Modules manifests
WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY cmd/agent/main.go cmd/agent/main.go
COPY api/ api/
COPY internal/ internal/
COPY pkg/ pkg/

ARG TAG=v0.0.0
ARG COMMIT=""
ARG COMMITDATE=""

# Build agent binary
RUN CGO_ENABLED=1 go build \
    -ldflags "-w -s  \
    -X github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version.Version=$TAG  \
    -X github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version.Commit=$COMMIT  \
    -X github.com/rancher-sandbox/cluster-api-provider-elemental/internal/version.CommitDate=$COMMITDATE"  \
    -a -o elemental-agent cmd/agent/main.go

# Build elemental-toolkit plugin
RUN CGO_ENABLED=1 go build \
    -buildmode=plugin \
    -o elemental.so internal/agent/plugin/elemental/elemental.go

# Build dummy plugin
RUN CGO_ENABLED=1 go build \
    -buildmode=plugin \
    -o dummy.so internal/agent/plugin/dummy/dummy.go

FROM  ${ELEMENTAL_TOOLKIT} as TOOLKIT

# OS base image of our choice
FROM opensuse/tumbleweed:latest as OS

ARG AGENT_CONFIG_FILE=iso/config/example-config.yaml

COPY iso/config/manifest.yaml manifest.yaml

# install kernel, systemd, dracut, grub2 and other required tools
RUN ARCH=$(uname -m); \
    if [[ $ARCH == "aarch64" ]]; then ARCH="arm64"; fi; \
    zypper --non-interactive install --no-recommends -- \
      kernel-default \
      device-mapper \
      dracut \
      grub2 \
      grub2-${ARCH}-efi \
      haveged \
      systemd \
      NetworkManager \
      openssh-server \
      openssh-clients \
      timezone \
      parted \
      e2fsprogs \
      dosfstools \
      mtools \
      xorriso \
      findutils \
      gptfdisk \
      rsync \
      squashfs \
      lvm2 \
      tar \
      gzip \
      vim \
      which \
      less \
      sudo \
      curl \
      sed \
      patch \
      iproute2 \
      shim \
      btrfsprogs \
      btrfsmaintenance \
      snapper

# Install kubeadm stack dependencies
RUN ARCH=$(uname -m); \
    if [[ $ARCH == "aarch64" ]]; then ARCH="arm64"; fi; \
    zypper --non-interactive install -- \
      conntrackd \
      conntrack-tools \
      iptables \
      ebtables \
      buildah \
      ethtool \
      socat

# Install kubeadm stack
COPY test/scripts/install_kubeadm_stack.sh /tmp/install_kubeadm_stack.sh
RUN /tmp/install_kubeadm_stack.sh
RUN rm -f /tmp/install_kubeadm_stack.sh

# Add the elemental cli
COPY --from=TOOLKIT /usr/bin/elemental /usr/bin/elemental
# Add the elemental-agent and plugins
COPY --from=AGENT /workspace/elemental-agent /usr/sbin/elemental-agent
COPY --from=AGENT /workspace/elemental.so /usr/lib/elemental/plugins/elemental.so
COPY --from=AGENT /workspace/dummy.so /usr/lib/elemental/plugins/dummy.so

# Add framework files
COPY framework/files/ /

# Add agent config
COPY $AGENT_CONFIG_FILE /oem/elemental/agent/config.yaml

# Enable essential services
RUN systemctl enable NetworkManager.service sshd conntrackd kubelet containerd

# This is for automatic testing purposes, do not do this in production.
RUN echo "PermitRootLogin yes" > /etc/ssh/sshd_config.d/rootlogin.conf

# Generate initrd with required elemental services
RUN elemental --debug init --force

# Update os-release file with some metadata
RUN echo TIMESTAMP="`date +'%Y%m%d%H%M%S'`" >> /etc/os-release && \
    echo GRUB_ENTRY_NAME=\"Elemental\" >> /etc/os-release

# Good for validation after the build
CMD /bin/bash
