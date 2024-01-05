#!/bin/bash
set -e

# This script generates cluster manifests using all available providers in separate namespaces.
# The purpose is to have a quick automated way to test that all templates and cluster classes are working correctly.

# Setup
PROVIDER_VERSION="${PROVIDER_VERSION:-"v0.0.0"}"
LOG_LEVEL=${LOG_LEVEL:-4}
MANIFESTS_DIR="/tmp/cluster-api-provider-elemental/test/manifests"
mkdir -p $MANIFESTS_DIR

# This is applied to all clusters, so it's quite incorrect on a normal scenario, but good enough for testing.
export CONTROL_PLANE_ENDPOINT_HOST="192.168.100.100"


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
  url: "https://github.com/cluster-api-provider-k3s/cluster-api-k3s/releases/latest/bootstrap-components.yaml"
  type: "BootstrapProvider"
- name: "k3s"
  url: "https://github.com/cluster-api-provider-k3s/cluster-api-k3s/releases/latest/control-plane-components.yaml"
  type: "ControlPlaneProvider"
- name: "rke2"
  url: "https://github.com/rancher-sandbox/cluster-api-provider-rke2/releases/latest/bootstrap-components.yaml"
  type: "BootstrapProvider"
- name: "rke2"
  url: "https://github.com/rancher-sandbox/cluster-api-provider-rke2/releases/latest/control-plane-components.yaml"
  type: "ControlPlaneProvider"
EOF

# k3s
printf "\n##### k3s-single-node #####\n"
MANIFEST_K3S_SINGLE_NODE="$MANIFESTS_DIR/k3s-single-node.yaml"
MANIFEST_K3S_SINGLE_NODE_NAMESPACE="k3s-single-node"
kubectl delete namespace $MANIFEST_K3S_SINGLE_NODE_NAMESPACE --ignore-not-found
clusterctl generate cluster --config $CONFIG_FILE \
--infrastructure elemental:$PROVIDER_VERSION \
--target-namespace $MANIFEST_K3S_SINGLE_NODE_NAMESPACE \
--flavor k3s-single-node \
--v $LOG_LEVEL \
k3s-single-node > $MANIFEST_K3S_SINGLE_NODE
kubectl create namespace $MANIFEST_K3S_SINGLE_NODE_NAMESPACE
kubectl apply -f $MANIFEST_K3S_SINGLE_NODE

printf "\n##### k3s #####\n"
MANIFEST_K3S="$MANIFESTS_DIR/k3s.yaml"
MANIFEST_K3S_NAMESPACE="k3s"
kubectl delete namespace $MANIFEST_K3S_NAMESPACE --ignore-not-found
clusterctl generate cluster --config $CONFIG_FILE \
--control-plane-machine-count=1 \
--worker-machine-count=1 \
--infrastructure elemental:$PROVIDER_VERSION \
--target-namespace $MANIFEST_K3S_NAMESPACE \
--flavor k3s \
--v $LOG_LEVEL \
k3s > $MANIFEST_K3S
kubectl create namespace $MANIFEST_K3S_NAMESPACE
kubectl apply -f $MANIFEST_K3S

## k3s clusterclass not supported upstream yet
# printf "\n##### k3s-clusterclass #####\n"
# MANIFEST_K3S_CLUSTERCLASS="$MANIFESTS_DIR/k3s-clusterclass.yaml"
# MANIFEST_K3S_CLUSTERCLASS_NAMESPACE="k3s-clusterclass"
# kubectl delete namespace $MANIFEST_K3S_CLUSTERCLASS_NAMESPACE --ignore-not-found
# clusterctl generate cluster --config $CONFIG_FILE \
# --control-plane-machine-count=1 \
# --worker-machine-count=1 \
# --infrastructure elemental:$PROVIDER_VERSION \
# --target-namespace $MANIFEST_K3S_CLUSTERCLASS_NAMESPACE \
# --flavor k3s-clusterclass \
# --v $LOG_LEVEL \
# k3s-clusterclass > $MANIFEST_K3S_CLUSTERCLASS
# kubectl create namespace $MANIFEST_K3S_CLUSTERCLASS_NAMESPACE
# kubectl apply -f $MANIFEST_K3S_CLUSTERCLASS

# rke2
printf "\n##### rke2 #####\n"
MANIFEST_RKE2="$MANIFESTS_DIR/rke2.yaml"
MANIFEST_RKE2_NAMESPACE="rke2"
kubectl delete namespace $MANIFEST_RKE2_NAMESPACE --ignore-not-found
clusterctl generate cluster --config $CONFIG_FILE \
--control-plane-machine-count=1 \
--worker-machine-count=1 \
--infrastructure elemental:$PROVIDER_VERSION \
--target-namespace $MANIFEST_RKE2_NAMESPACE \
--flavor rke2 \
--v $LOG_LEVEL \
rke2 > $MANIFEST_RKE2
kubectl create namespace $MANIFEST_RKE2_NAMESPACE
kubectl apply -f $MANIFEST_RKE2

## rke2 clusterclass not supported upstream yet
# printf "\n##### rke2-clusterclass #####\n"
# MANIFEST_RKE2_CLUSTERCLASS="$MANIFESTS_DIR/rke2-clusterclass.yaml"
# MANIFEST_RKE2_CLUSTERCLASS_NAMESPACE="rke2-clusterclass"
# kubectl delete namespace $MANIFEST_RKE2_CLUSTERCLASS_NAMESPACE --ignore-not-found
# clusterctl generate cluster --config $CONFIG_FILE \
# --control-plane-machine-count=1 \
# --worker-machine-count=1 \
# --infrastructure elemental:$PROVIDER_VERSION \
# --target-namespace $MANIFEST_RKE2_CLUSTERCLASS_NAMESPACE \
# --flavor rke2-clusterclass \
# --v $LOG_LEVEL \
# rke2-clusterclass > $MANIFEST_RKE2_CLUSTERCLASS
# kubectl create namespace $MANIFEST_RKE2_CLUSTERCLASS_NAMESPACE
# kubectl apply -f $MANIFEST_RKE2_CLUSTERCLASS

# kubeadm
printf "\n##### kubeadm #####\n"
MANIFEST_KUBEADM="$MANIFESTS_DIR/kubeadm.yaml"
MANIFEST_KUBEADM_NAMESPACE="kubeadm"
kubectl delete namespace $MANIFEST_KUBEADM_NAMESPACE --ignore-not-found
clusterctl generate cluster --config $CONFIG_FILE \
--control-plane-machine-count=1 \
--worker-machine-count=1 \
--infrastructure elemental:$PROVIDER_VERSION \
--target-namespace $MANIFEST_KUBEADM_NAMESPACE \
--flavor kubeadm \
--v $LOG_LEVEL \
kubeadm > $MANIFEST_KUBEADM
kubectl create namespace $MANIFEST_KUBEADM_NAMESPACE
kubectl apply -f $MANIFEST_KUBEADM

printf "\n##### kubeadm-clusterclass #####\n"
MANIFEST_KUBEADM_CLUSTERCLASS="$MANIFESTS_DIR/kubeadm-clusterclass.yaml"
MANIFEST_KUBEADM_CLUSTERCLASS_NAMESPACE="kubeadm-clusterclass"
kubectl delete namespace $MANIFEST_KUBEADM_CLUSTERCLASS_NAMESPACE --ignore-not-found
clusterctl generate cluster --config $CONFIG_FILE \
--control-plane-machine-count=1 \
--worker-machine-count=1 \
--infrastructure elemental:$PROVIDER_VERSION \
--target-namespace $MANIFEST_KUBEADM_CLUSTERCLASS_NAMESPACE \
--flavor kubeadm-clusterclass \
--v $LOG_LEVEL \
kubeadm-clusterclass > $MANIFEST_KUBEADM_CLUSTERCLASS
kubectl create namespace $MANIFEST_KUBEADM_CLUSTERCLASS_NAMESPACE
kubectl apply -f $MANIFEST_KUBEADM_CLUSTERCLASS
