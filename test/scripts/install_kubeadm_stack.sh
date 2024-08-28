#!/bin/bash

# See: https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/install-kubeadm/#installing-kubeadm-kubelet-and-kubectl
# See: https://github.com/go4clouds/cloud-infra/blob/main/libvirt/provision-k8s-node.sh

set -e

ARCH=$(uname -m)
if [[ $ARCH == "aarch64" ]]; then ARCH="arm64"; fi;
if [[ $ARCH == "x86_64" ]]; then ARCH="amd64"; fi;

KUBEADM_DIR="/usr/bin"
mkdir -p "$KUBEADM_DIR"

## CNI Plugins
## See: https://github.com/containernetworking/plugins
echo "Installing CNI Plugins"
CNI_PLUGINS_VERSION=${CNI_PLUGINS_VERSION:="v1.5.1"}
DEST="/opt/cni/bin"
mkdir -p "$DEST"
curl -L "https://github.com/containernetworking/plugins/releases/download/${CNI_PLUGINS_VERSION}/cni-plugins-linux-${ARCH}-${CNI_PLUGINS_VERSION}.tgz" | tar -C "$DEST" -xz

## crictl
## See: https://github.com/kubernetes-sigs/cri-tools
##      https://github.com/kubernetes-sigs/cri-tools?tab=readme-ov-file#compatibility-matrix-cri-tools--kubernetes
echo "Installing crictl"
CRICTL_VERSION=${CRICTL_VERSION:="v1.31.1"}
curl -L "https://github.com/kubernetes-sigs/cri-tools/releases/download/${CRICTL_VERSION}/crictl-${CRICTL_VERSION}-linux-${ARCH}.tar.gz" | tar -C "$KUBEADM_DIR" -xz

## kubeadm/kubelet
## See: https://www.downloadkubernetes.com/
echo "Installing kubeadm/kubelet"
K8S_RELEASE=${K8S_RELEASE:="v1.31.0"}
ARCH="amd64"
cd $KUBEADM_DIR
curl -L --remote-name-all https://dl.k8s.io/release/${K8S_RELEASE}/bin/linux/${ARCH}/{kubeadm,kubelet}
chmod +x {kubeadm,kubelet}

## kubeadm/kubelet configs
## See: https://github.com/kubernetes/release/tree/master
K8S_RELEASE_VERSION=${K8S_RELEASE_VERSION:="v0.17.2"}
mkdir -p /usr/lib/systemd/system/kubelet.service.d

#curl -sSL "https://raw.githubusercontent.com/kubernetes/release/${K8S_RELEASE_VERSION}/cmd/krel/templates/latest/kubelet/kubelet.service" -o /usr/lib/systemd/system/kubelet.service
cat >> /usr/lib/systemd/system/kubelet.service << EOF
[Unit]
Description=kubelet: The Kubernetes Node Agent
Documentation=https://kubernetes.io/docs/
Wants=network-online.target
After=network-online.target
# Do not start in Elemental Recovery or Live mode
ConditionPathExists=!/run/elemental/live_mode
ConditionPathExists=!/run/elemental/recovery_mode

[Service]
ExecStart=$KUBEADM_DIR/kubelet
Restart=always
StartLimitInterval=0
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

curl -sSL "https://raw.githubusercontent.com/kubernetes/release/${K8S_RELEASE_VERSION}/cmd/krel/templates/latest/kubeadm/10-kubeadm.conf" -o /usr/lib/systemd/system/kubelet.service.d/10-kubeadm.conf

## kubectl
echo "Installing kubectl"
curl -LO https://dl.k8s.io/release/${K8S_RELEASE}/bin/linux/amd64/kubectl
chmod +x kubectl

## containerd
## See: https://github.com/containerd/containerd
echo "Installing containerd"
CONTAINERD_VERSION=${CONTAINERD_VERSION:="1.7.20"}
CONTAINERD_DIR="/usr/local/bin"
mkdir -p "$CONTAINERD_DIR"
cd $CONTAINERD_DIR
curl -L "https://github.com/containerd/containerd/releases/download/v${CONTAINERD_VERSION}/containerd-${CONTAINERD_VERSION}-linux-${ARCH}.tar.gz" | tar  --strip-components=1 -C "$CONTAINERD_DIR" -xz
#curl -sSL "https://raw.githubusercontent.com/containerd/containerd/v${CONTAINERD_VERSION}/containerd.service" -o /usr/lib/systemd/system/containerd.service
cat >> /usr/lib/systemd/system/containerd.service << EOF
# Copyright The containerd Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

[Unit]
Description=containerd container runtime
Documentation=https://containerd.io
After=network.target local-fs.target
# Do not start in Elemental Recovery or Live mode
ConditionPathExists=!/run/elemental/live_mode
ConditionPathExists=!/run/elemental/recovery_mode

[Service]
#uncomment to enable the experimental sbservice (sandboxed) version of containerd/cri integration
#Environment="ENABLE_CRI_SANDBOXES=sandboxed"
ExecStartPre=-/sbin/modprobe overlay
ExecStart=$CONTAINERD_DIR/containerd

Type=notify
Delegate=yes
KillMode=process
Restart=always
RestartSec=5
# Having non-zero Limit*s causes performance problems due to accounting overhead
# in the kernel. We recommend using cgroups to do container-local accounting.
LimitNPROC=infinity
LimitCORE=infinity
LimitNOFILE=infinity
# Comment TasksMax if your systemd version does not supports it.
# Only systemd 226 and above support this version.
TasksMax=infinity
OOMScoreAdjust=-999

[Install]
WantedBy=multi-user.target
EOF

## containerd config
## Generate a default config with 'containerd config default'
mkdir -p /etc/containerd
containerd config default > /etc/containerd/config.toml
sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
sed -i 's/level = ""/level = "debug"/' /etc/containerd/config.toml
sed -i 's/snapshotter = "overlayfs"/snapshotter = "native"/' /etc/containerd/config.toml

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
