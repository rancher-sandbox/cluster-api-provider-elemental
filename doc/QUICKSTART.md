# Quickstart

## Prerequisites

1. This setup uses two machines. One to deploy the CAPI management cluster and one to deploy a single node k3s cluster.  
   The machines must be able to reach each other on the network.  
   The setup assumes `192.168.122.10` will be used for the CAPI management cluster (and to expose the Elemental API).  
   `192.168.122.100` will be used by the host and used as `CONTROL_PLANE_ENDPOINT_HOST` of the downstream k3s cluster.  

   Note that CAPI requires bi-directional communication between the **management** cluster and any provisioned downstream control plane endpoint.  
   For simplicity and demo purposes, it is recommended to run the **management** and all Elemental hosts within the same network.  

1. On the **management** machine, [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) is used to bootstrap a CAPI management cluster.  
   The ElementalAPI will be exposed using a `NodePort` on `30009`. This port needs to be free and not blocked by any firewall.  

## Preparation

1. On the **management** machine, install the required dependencies: `docker`, `kind`, `helm`, `kubectl`, and `clusterctl`.
   For example on a fresh OpenSUSE Tumbleweed installation, run:

    ```bash
    # Install dependencies
    zypper install -y docker helm kubernetes-client make yq

    # Install kind
    curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.24.0/kind-linux-amd64
    install -o root -g root -m 0755 kind /usr/local/bin/kind

    # Install clusterctl
    curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.8.1/clusterctl-linux-amd64 -o clusterctl
    install -o root -g root -m 0755 clusterctl /usr/local/bin/clusterctl

    systemctl enable docker
    systemctl disable firewalld

    # Reboot to please docker
    reboot
    ```

## Management Cluster configuration

1. Initialize a cluster:

    ```bash
    cat << EOF | kind create cluster --name elemental-capi-management --config -
    kind: Cluster
    apiVersion: kind.x-k8s.io/v1alpha4
    nodes:
    - role: control-plane
      image: kindest/node:v1.31.0
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
      - containerPort: 30000
        hostPort: 30000
        protocol: TCP
    EOF
    ```

1. Configure `clusterctl` to use the custom provider:

    ```bash
    mkdir -p $HOME/.cluster-api 

    cat << EOF > $HOME/.cluster-api/clusterctl.yaml
    providers:
    - name: "elemental"
      url: "https://github.com/rancher-sandbox/cluster-api-provider-elemental/releases/latest/infrastructure-components.yaml"
      type: "InfrastructureProvider"
    - name: "k3s"
      url: "https://github.com/k3s-io/cluster-api-k3s/releases/latest/bootstrap-components.yaml"
      type: "BootstrapProvider"
    - name: "k3s"
      url: "https://github.com/k3s-io/cluster-api-k3s/releases/latest/control-plane-components.yaml"
      type: "ControlPlaneProvider"
    EOF
    ```

1. Install CAPI Core provider, the k3s Control Plane and Bootstrap providers, and the Elemental Infrastructure provider:  

    ```bash
    ELEMENTAL_ENABLE_DEBUG="\"true\"" \
    ELEMENTAL_API_ENDPOINT="192.168.122.10.sslip.io" \
    ELEMENTAL_API_ENABLE_TLS="\"true\"" \
    ELEMENTAL_ENABLE_DEFAULT_CA="\"true\"" \
    clusterctl init --bootstrap k3s --control-plane k3s --infrastructure elemental
    ```

1. Expose the Elemental API server:  

    ```bash
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
    ```

1. Generate `cluster.yaml` config:

    ```bash
    CONTROL_PLANE_ENDPOINT_HOST=192.168.122.100 clusterctl generate cluster \
    --infrastructure elemental \
    --flavor k3s-single-node \
    elemental-cluster-k3s > $HOME/elemental-cluster-k3s.yaml
    ```

1. Apply `elemental-cluster-k3s.yaml` config:

    ```bash
    kubectl apply -f $HOME/elemental-cluster-k3s.yaml
    ```

1. Create a new `ElementalRegistration`:

    Note that since we are using a non-standard port for this quickstart, we are manually setting the registration `uri` field.  
    Normally this would be automatically populated by the controller from the `ELEMENTAL_API_PROTOCOL` and `ELEMENTAL_API_ENDPOINT` environment variables.  
    For more details on how to configure and expose the Elemental API, please read the related [document](./ELEMENTAL_API_SETUP.md).  

    ```bash
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
            snapshotter:
              type: btrfs
          reset:
            resetOem: true
            resetPersistent: true
    EOF
    ```

1. Wait for the `ElementalRegistration` to be ready:

   This will ensure the provider created a new private key to sign Registration tokens and the trust CA Cert is also loaded.  

   ```bash
   kubectl wait --for=condition=ready elementalregistration my-registration
   ```

## (Elemental Toolkit) Host configuration

A bootable ISO image can be build using the [elemental-toolkit](https://github.com/rancher/elemental-toolkit).
The image contains the `elemental-agent` and an initial configuration to connect. Upon boot, it will auto-install an Elemental system on a machine on the configured device: `/dev/vda`.

You can configure a different device, editing the `ElementalRegistration` created above.  

- Clone this repository:

    ```bash
    git clone https://github.com/rancher-sandbox/cluster-api-provider-elemental.git
    cd cluster-api-provider-elemental
    ```

- Generate a valid agent config (depends on `kubectl` and `yq`):  

    ```bash
    ./test/scripts/print_agent_config.sh -n default -r my-registration > iso/config/my-config.yaml
    ```

  Note that the agent config should contain a valid registration `token`.  
  By default this is a JWT formatted token with no expiration.  

- Build the ISO image:

    This depends on `make` and `docker`:

    ```bash
    AGENT_CONFIG_FILE=iso/config/my-config.yaml make build-iso
    ```

- A new bootable iso should be available: `iso/elemental-dev.iso`.

### kubeadm variant

A `kubeadm` ready image can be built with:

```bash
AGENT_CONFIG_FILE=iso/config/my-config.yaml KUBEADM_READY_OS=true make build-iso
```

Note that the Kubeadm cluster needs to be initialized with a CNI.
For example, using [calico](https://docs.tigera.io/calico/latest/getting-started/kubernetes/quickstart) you can run on any bootstrapped node:  

```bash
export KUBECONFIG=/etc/kubernetes/super-admin.conf

kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.28.1/manifests/tigera-operator.yaml

cat << EOF | kubectl apply -f -
apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  calicoNetwork:
    ipPools:
    - blockSize: 26
      cidr: 10.244.0.0/16
      encapsulation: VXLANCrossSubnet
      natOutgoing: Enabled
      nodeSelector: all()
---
apiVersion: operator.tigera.io/v1
kind: APIServer
metadata:
  name: default
spec: {}
EOF
```

A Cluster manifest can be generated with:

```bash
CONTROL_PLANE_ENDPOINT_HOST=192.168.122.50 \
clusterctl generate cluster \
--control-plane-machine-count=1 \
--worker-machine-count=1 \
--infrastructure elemental \
--flavor kubeadm \
kubeadm > ~/kubeadm-cluster-manifest.yaml
```

## Host upgrade

For more information about OS Version Reconcile, please consult the related [documentation](./OS_VERSION_RECONCILE.md).  
This part of the quickstart only covers upgrading the single k3s node that was provisioned previously.  

### Setup a test registry

In order to upgrade the node, we are going to need a registry where to push the new OS image to be applied.  
The registry needs to be installed on the **management** cluster and can be exposed through a NodePort.  
For this example we are going to use `192.168.122.10:30000` as our test registry.  

```bash
cat << EOF | kubectl apply -f -
apiVersion: v1
kind: Namespace
metadata:
  name: test-registry
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-registry
  namespace: test-registry
  labels:
    app: test-registry
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-registry
  template:
    metadata:
      labels:
        app: test-registry
    spec:
      containers:
      - name: registry
        image: registry:2
        ports:
        - containerPort: 5000
---
apiVersion: v1
kind: Service
metadata:
  name: registry-nodeport
  namespace: test-registry
spec:
  type: NodePort
  selector:
    app: test-registry
  ports:
  - nodePort: 30000
    port: 5000
    protocol: TCP
    targetPort: 5000  
EOF
```

### Build an updated OS Image

A fresh OS image can be rebuilt with `make build-os`.  
For conveniency, the `ELEMENTAL_OS_IMAGE` can be used to specify the tag.  

```bash
AGENT_CONFIG_FILE=iso/config/my-config.yaml ELEMENTAL_OS_IMAGE="192.168.122.10:30000/elemental-capi-os:v1.2.3" make build-os
```

The image can be pushed to the registry:

```bash
docker push 192.168.122.10:30000/elemental-capi-os:v1.2.3
```

### Apply the new OS Version to the ElementalHost

The `ElementalMachine` associated with the `ElementalHost` can now be upgraded simply patching the `.spec.osVersionManagement` to it:

```bash
kubectl patch elementalmachine elemental-cluster-k3s-control-plane-p8jzq -p '{"spec":{"osVersionManagement":{"osVersion":{"imageUri":"oci://192.168.122.10:30000/elemental-capi-os:v1.2.3","debug":true}}}}' --type=merge
```

Since this `ElementalHost` is already bootstrapped, we are going to apply the `pending` `in-place-update` label.

```bash
kubectl label elementalhost m-aa90d0b4-f81a-4eb7-aa9f-f835f06cb527 elementalhost.infrastructure.cluster.x-k8s.io/in-place-update=pending
```

The host should then upgrade to the new image and reboot.
Upon update completion, the `in-place-update` label will mutate to `done`, the `ElementalHost` will also be ready:

```yaml
kubectl wait --for=condition=ready elementalhost m-aa90d0b4-f81a-4eb7-aa9f-f835f06cb527
```

## Trigger a Host reset

A Host can receive a trigger reset instruction on the following scenarios:

- The ElementalHost resource on the k8s management cluster is deleted:  

    ```bash
    kubectl delete elementalhost my-host
    ```

- The ElementalMachine associated to this host is deleted:  

    ```bash
    kubectl delete elementalmachine my-control-plane-machine
    ```

- The CAPI Cluster resource that this Host belonged to is deleted:  

    ```bash
    kubectl delete cluster my-cluster
    ```
