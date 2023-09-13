FROM opensuse/tumbleweed:latest

RUN zypper install -y systemd
RUN zypper install -y fuse-overlayfs

COPY crio-docker-storage.conf /etc/crio/crio.conf.d/99-storage.conf
RUN zypper install -y patterns-kubernetes-kubeadm

COPY bin/agent /agent
COPY agent-config.yaml /oem/elemental/agent/config.yaml

ENTRYPOINT ["/sbin/init"]
