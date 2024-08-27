#!/bin/bash
set -e

cat << EOF | kind create cluster --name elemental-capi-management --config -
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: kindest/node:v1.26.6
  kubeadmConfigPatches:
  - |
    kind: InitConfiguration
    nodeRegistration:
      kubeletExtraArgs:
        node-labels: "ingress-ready=true"
  extraPortMappings:
  - containerPort: 80
    hostPort: 80
    protocol: TCP
  - containerPort: 443
    hostPort: 443
    protocol: TCP
  - containerPort: 30009
    hostPort: 30009
    protocol: TCP
EOF

# Build the Elemental provider docker image and load it to the kind cluster
make kind-load

# Generate infrastructure manifest
make generate-infra-yaml

# Create a dummy clusterctl config in a tmp folder
CONFIG_DIR="/tmp/cluster-api"
CONFIG_FILE="$CONFIG_DIR/clusterctl.yaml"
cd "$(dirname "$0")/../../"
REPO_DIR="$(pwd)"
mkdir -p $CONFIG_DIR
cat << EOF > $CONFIG_FILE
providers:
- name: "elemental"
  url: "file:///$REPO_DIR/infrastructure-elemental/v0.0.0/infrastructure-components.yaml"
  type: "InfrastructureProvider"
- name: "k3s"
  url: "https://github.com/k3s-io/cluster-api-k3s/releases/latest/bootstrap-components.yaml"
  type: "BootstrapProvider"
- name: "k3s"
  url: "https://github.com/k3s-io/cluster-api-k3s/releases/latest/control-plane-components.yaml"
  type: "ControlPlaneProvider"
- name: "rke2"
  url: "https://github.com/rancher-sandbox/cluster-api-provider-rke2/releases/latest/bootstrap-components.yaml"
  type: "BootstrapProvider"
- name: "rke2"
  url: "https://github.com/rancher-sandbox/cluster-api-provider-rke2/releases/latest/control-plane-components.yaml"
  type: "ControlPlaneProvider"
EOF

# Determine the public IP address of this host
# This is used to expose the Elemental API
DEFAULT_HOST=$(ip addr show $(ip route | awk '/default/ { print $5 }') | grep "inet\b" | awk '{print $2}' | cut -d/ -f1)

# Enable Experimental cluster topology support (Cluster classes)
export CLUSTER_TOPOLOGY=true
# Level 5 is highest for debugging
export CLUSTERCTL_LOG_LEVEL=4

# Elemental provider variables
export ELEMENTAL_ENABLE_DEBUG="\"true\""
export ELEMENTAL_API_ENDPOINT="$DEFAULT_HOST.sslip.io"
export ELEMENTAL_API_ENABLE_TLS="\"true\""
export ELEMENTAL_ENABLE_DEFAULT_CA="\"true\""

# Install kubeadm, k3s, and rke2 providers for testing
clusterctl init --config $CONFIG_FILE \
                --bootstrap kubeadm --control-plane kubeadm \
                --bootstrap k3s --control-plane k3s \
                --bootstrap rke2 --control-plane rke2 \
                --infrastructure elemental:v0.0.0

# Expose the Elemental API through a nodeport
cat << EOF | kubectl apply -f -
apiVersion: v1
kind: Service
metadata:
  name: elemental-debug
  namespace: elemental-system
spec:
  type: NodePort
  selector:
    control-plane: controller-manager
  ports:
  - nodePort: 30009
    port: 9090
    protocol: TCP
    targetPort: 9090    
EOF

# Create a test registration
cat << EOF | kubectl apply -f -
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ElementalRegistration
metadata:
  name: my-registration
  namespace: default
spec:
  config:
    cloudConfig:
      users:
        - name: root
          passwd: root
    elemental:
      registration:
        uri: https://$DEFAULT_HOST.sslip.io:30009/elemental/v1/namespaces/default/registrations/my-registration
      agent:
        hostname:
          useExisting: false
          prefix: "m-"
        debug: true
        osPlugin: "/usr/lib/elemental/plugins/elemental.so"
        workDir: "/oem/elemental/agent"
        postInstall:
          reboot: true
      install:
        debug: true
        device: "/dev/vda"
        snapshotter:
          type: btrfs
      reset:
        resetOem: true
        resetPersistent: true
EOF

# Wait for registration to be initialized
kubectl wait --for=condition=ready elementalregistration my-registration --timeout=120s

# Print the agent config
cd "$(dirname "$0")"
echo -e "Test Agent config:\n"
./print_agent_config.sh -n default -r my-registration
echo -e "\nBuild a new image with:"\
  "\n test/scripts/print_agent_config.sh -n default -r my-registration > iso/config/my-config.yaml"\
  "\n AGENT_CONFIG_FILE=iso/config/my-config.yaml make build-iso"
