#!/bin/bash

cat << EOF | kind create cluster --name elemental-capi-management --config -
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  image: kindest/node:v1.26.4
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

make kind-load

make generate-infra-yaml

export ELEMENTAL_ENABLE_DEBUG="\"true\""
export ELEMENTAL_API_ENDPOINT="192.168.122.10.sslip.io"
export ELEMENTAL_API_PROTOCOL="https"
export ELEMENTAL_API_ENABLE_TLS="\"true\""
export ELEMENTAL_ENABLE_DEFAULT_CA="\"true\""
clusterctl init --bootstrap k3s:v0.1.8 --control-plane k3s:v0.1.8 --infrastructure elemental:v0.0.0

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
        uri: https://192.168.122.10.sslip.io:30009/elemental/v1/namespaces/default/registrations/my-registration
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
      reset:
        resetOem: true
        resetPersistent: true
EOF
