FROM opensuse/tumbleweed:latest

RUN zypper install -y systemd
RUN zypper install -y fuse-overlayfs
RUN zypper install -y patterns-kubernetes-kubeadm containerd

RUN systemctl enable containerd
RUN systemctl enable kubelet

COPY bin/agent /agent
COPY agent-config.yaml /oem/elemental/agent/config.yaml

ENTRYPOINT ["/sbin/init"]
