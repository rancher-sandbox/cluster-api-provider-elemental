FROM opensuse/tumbleweed:latest as OS

RUN zypper install -y patterns-kubernetes-kubeadm containerd

COPY bin/agent /agent
COPY agent-config.yaml /oem/elemental/agent/config.yaml

ENTRYPOINT ["/agent"]
