ARG ELEMENTAL_TOOLKIT
ARG ELEMENTAL_AGENT

FROM ${ELEMENTAL_AGENT} as AGENT
FROM  ${ELEMENTAL_TOOLKIT} as TOOLKIT

# OS base image of our choice
FROM registry.opensuse.org/opensuse/tumbleweed:latest as OS

ARG AGENT_CONFIG_FILE=iso/config/example-config.yaml
ARG KUBEADM_READY

# install kernel, systemd, dracut, grub2 and other required tools
RUN ARCH=$(uname -m); \
    if [[ $ARCH == "aarch64" ]]; then ARCH="arm64"; fi; \
    zypper --non-interactive install --no-recommends -- \
      kernel-default \
      device-mapper \
      dracut \
      grub2 \
      grub2-${ARCH}-efi \
      shim \
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
      iptables \
      iproute2 \
      btrfsprogs \
      btrfsmaintenance \
      snapper

# Add the elemental cli
COPY --from=TOOLKIT /usr/bin/elemental /usr/bin/elemental
# Add the elemental-agent and plugins
COPY --from=AGENT /elemental-agent /usr/sbin/elemental-agent
COPY --from=AGENT /elemental.so /usr/lib/elemental/plugins/elemental.so
COPY --from=AGENT /dummy.so /usr/lib/elemental/plugins/dummy.so

# Install kubeadm stack dependencies
RUN if [ -n "${KUBEADM_READY}" ]; then \
    zypper --non-interactive install -- \
    conntrackd \
    conntrack-tools \
    iptables \
    ebtables \
    buildah \
    ethtool \
    socat; \
    fi;

# Install kubeadm stack
COPY test/scripts/install_kubeadm_stack.sh /tmp/install_kubeadm_stack.sh
RUN if [ -n "${KUBEADM_READY}" ]; then /tmp/install_kubeadm_stack.sh; fi;
RUN rm -f /tmp/install_kubeadm_stack.sh

# Add framework files
COPY framework/files/ /

# Add agent config
COPY $AGENT_CONFIG_FILE /oem/elemental/agent/config.yaml

# Enable essential services
RUN systemctl enable NetworkManager.service sshd

# Enable kubeadm needed services
RUN if [ -n "${KUBEADM_READY}" ]; then \
    systemctl enable conntrackd containerd kubelet; \
    fi;

# !!! This is for testing purposes, do not do this in production. !!!
RUN echo "PermitRootLogin yes" > /etc/ssh/sshd_config.d/rootlogin.conf

# Make sure trusted certificates are properly generated
RUN /usr/sbin/update-ca-certificates

# Ensure /tmp is mounted as tmpfs by default
RUN if [ -e /usr/share/systemd/tmp.mount ]; then \
      cp /usr/share/systemd/tmp.mount /etc/systemd/system; \
    fi

# Save some space
RUN zypper clean --all && \
    rm -rf /var/log/update* && \
    >/var/log/lastlog && \
    rm -rf /boot/vmlinux*

# Ensure /tmp is mounted as tmpfs by default
RUN if [ -e /usr/share/systemd/tmp.mount ]; then \
      cp /usr/share/systemd/tmp.mount /etc/systemd/system; \
    fi

# Required by k3s/rke2
RUN mkdir -p /usr/libexec && touch /usr/libexec/.keep

# Generate initrd with required elemental services
# Features currently excluded: [cloud-config-defaults]
RUN elemental init --debug --force boot-assessment,cloud-config-essentials,dracut-config,elemental-rootfs,elemental-setup,elemental-sysroot,grub-config,grub-default-bootargs

# Update os-release file with some metadata
RUN echo TIMESTAMP="`date +'%Y%m%d%H%M%S'`" >> /etc/os-release && \
    echo GRUB_ENTRY_NAME=\"Elemental\" >> /etc/os-release

# Good for validation after the build
CMD /bin/bash
