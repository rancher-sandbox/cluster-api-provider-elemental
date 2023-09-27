FROM opensuse/tumbleweed:latest

RUN zypper install -y systemd
RUN zypper install -y fuse-overlayfs
RUN zypper install -y patterns-kubernetes-kubeadm
# Add missing kubeadm dependencies
RUN zypper install -y iproute2 iptables conntrack-tools

RUN systemctl enable containerd

COPY bin/agent /agent
COPY agent-config.yaml /oem/elemental/agent/config.yaml

ENTRYPOINT ["/sbin/init"]
