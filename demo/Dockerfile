FROM opensuse/tumbleweed:latest

RUN zypper install -y systemd
RUN zypper install -y fuse-overlayfs
RUN zypper install -y patterns-kubernetes-kubeadm

## FIXME: Using cri-o until the kubeadm pattern is fixed
RUN systemctl disable containerd
RUN zypper install -y cri-o
COPY crio-docker-storage.conf /etc/crio/crio.conf.d/99-storage.conf
RUN systemctl enable crio

COPY bin/agent /agent
COPY agent-config.yaml /oem/elemental/agent/config.yaml

ENTRYPOINT ["/sbin/init"]
