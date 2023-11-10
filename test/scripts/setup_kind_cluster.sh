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

clusterctl init --infrastructure "-"

make generate-infra-yaml
kubectl apply -f infrastructure-elemental/v0.0.0/infrastructure-components.yaml

make kind-load

export ELEMENTAL_API_URL="http://192.168.122.10:30009" 
kubectl -n elemental-system patch deployment elemental-controller-manager -p '{"spec":{"template":{"spec":{"containers":[{"name":"manager","env":[{"name":"ELEMENTAL_API_URL","value":"'${ELEMENTAL_API_URL}'"}]}]}}}}'

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
      agent:
        hostname:
          useExisting: false
          prefix: "m-"
        debug: true
        osPlugin: "/usr/lib/elemental/plugins/elemental.so"
        insecureAllowHttp: true
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
