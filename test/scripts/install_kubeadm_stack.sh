#!/bin/sh

# See: https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/#installing-kubeadm-kubelet-and-kubectl
# See: https://github.com/go4clouds/cloud-infra/blob/main/libvirt/provision-k8s-node.sh

set -e

ARCH=$(uname -m)
if [[ $ARCH == "aarch64" ]]; then ARCH="arm64"; fi;
if [[ $ARCH == "x86_64" ]]; then ARCH="amd64"; fi;

DOWNLOAD_DIR="/usr/local/bin"
mkdir -p "$DOWNLOAD_DIR"

## CNI Plugins
CNI_PLUGINS_VERSION="v1.4.1"
DEST="/opt/cni/bin"
mkdir -p "$DEST"
curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/cni-plugins-linux-${ARCH}-${CNI_PLUGINS_VERSION}.tgz" | tar -C "$DEST" -xz

## crictl
CRICTL_VERSION="v1.29.0"
curl -L "https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-${ARCH}.tar.gz" | tar -C $DOWNLOAD_DIR -xz

## kubeadm/kubelet
RELEASE="v1.29.3"
ARCH="amd64"
cd $DOWNLOAD_DIR
curl -L --remote-name-all https://dl.k8s.io/release/${RELEASE}/bin/linux/${ARCH}/{kubeadm,kubelet}
chmod +x {kubeadm,kubelet}

RELEASE_VERSION="v0.16.5"
curl -sSL "https://raw.githubusercontent.com/kubernetes/release/${RELEASE_VERSION}/cmd/krel/templates/latest/kubelet/kubelet.service" | sed "s:/usr/bin:${DOWNLOAD_DIR}:g" | tee /usr/lib/systemd/system/kubelet.service
mkdir -p /usr/lib/systemd/system/kubelet.service.d
curl -sSL "https://raw.githubusercontent.com/kubernetes/release/${RELEASE_VERSION}/cmd/krel/templates/latest/kubeadm/10-kubeadm.conf" | sed "s:/usr/bin:${DOWNLOAD_DIR}:g" | tee /usr/lib/systemd/system/kubelet.service.d/10-kubeadm.conf

## kubectl
curl -LO https://dl.k8s.io/release/${RELEASE}/bin/linux/amd64/kubectl
chmod +x kubectl

## containerd
CONTAINERD_VERSION="1.7.14"
curl -L "https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz" | tar  --strip-components=1 -C "$DOWNLOAD_DIR" -xz
curl -sSL "https://raw.githubusercontent.com/containerd/containerd/main/containerd.service" | sed "s:/usr/bin:${DOWNLOAD_DIR}:g" | tee /usr/lib/systemd/system/containerd.service

## Preflight checks
# See: https://github.com/go4clouds/cloud-infra/blob/main/libvirt/provision-os-node.sh

# Load br_netfilter
cat >> /etc/modules-load.d/99-k8s.conf << EOF
overlay
br_netfilter
EOF

# Network-related sysctls
cat >> /etc/sysctl.d/99-k8s.conf << EOF
net.bridge.bridge-nf-call-iptables = 1
net.ipv4.ip_forward = 1
net.ipv4.conf.all.forwarding = 1
EOF
