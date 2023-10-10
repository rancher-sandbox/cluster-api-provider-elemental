# Quickstart

## Prerequisites

1. This setup uses two machines. One to deploy the CAPI management cluster and one to deploy a single node k3s cluster.  
   The machines must be able to reach each other on the network.  
   The setup assumes `192.168.122.10` will be used for the CAPI management cluster (and to expose the Elemental API).  
   `192.168.122.100` will be used by the host and used as `CONTROL_PLANE_ENDPOINT_IP` of the downstream k3s cluster.  

1. On the **management** machine, [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation) is used to bootstrap a CAPI management cluster.  
   The ElementalAPI will be exposed using a `NodePort` on `30009`. This port needs to be free and not blocked by any firewall.  

1. On the **host** machine, it is recommended to disable and stop the firewall to not interfere with the k3s deployment.  

## Preparation

1. On the **management** machine, install the required dependencies: `docker`, `kind`, `helm`, `kubectl`, and `clusterctl`.
   For example on a fresh OpenSUSE Tumbleweed installation, run:

    ```bash
    # Install dependencies
    zypper install -y docker helm kubernetes1.27-client

    # Install kind
    [ $(uname -m) = x86_64 ] && curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
    chmod +x ./kind
    mv ./kind /usr/local/bin/kind

    # Install clusterctl
    curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.5.2/clusterctl-linux-amd64 -o clusterctl
    install -o root -g root -m 0755 clusterctl /usr/local/bin/clusterctl

    systemctl enable docker
    systemctl disable firewalld

    # Reboot to please docker
    reboot
    ```

1. On the **host** machine, no dependencies are needed since they will be included by `k3s`.  
   The firewall however should be disabled.  

   ```bash
   systemctl disable firewalld
   systemctl stop firewalld
   ```

## Management Cluster configuration

1. Initialize a cluster:

    ```bash
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
    ```

1. Configure `clusterctl` to use the custom provider:

    ```bash
    mkdir -p $HOME/.cluster-api 

    cat << EOF > $HOME/.cluster-api/clusterctl.yaml
    providers:
    - name: "elemental"
      url: "https://github.com/rancher-sandbox/cluster-api-provider-elemental/releases/v0.0.1/infrastructure-components.yaml"
      type: "InfrastructureProvider"
    EOF
    ```

1. Install CAPI Core provider and the Elemental Infrastructure provider:  

    ```bash
    clusterctl init --bootstrap "-" --control-plane "-" --infrastructure elemental:v0.0.1
    ```

1. Install the **k3s** bootstrap and control plane providers:

   **Note**: This is a workaround for the current [issue](https://github.com/cluster-api-provider-k3s/cluster-api-k3s/issues/55) using `clusterctl init`.  

    ```bash
    kubectl apply -f https://github.com/cluster-api-provider-k3s/cluster-api-k3s/releases/download/v0.1.7/bootstrap-components.yaml
    kubectl apply -f https://github.com/cluster-api-provider-k3s/cluster-api-k3s/releases/download/v0.1.7/control-plane-components.yaml
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

1. Set the `ELEMENTAL_API_URL` on the operator:

    ```bash
    export ELEMENTAL_API_URL="http://192.168.122.10:30009" 
    kubectl -n elemental-system patch deployment elemental-controller-manager -p '{"spec":{"template":{"spec":{"containers":[{"name":"manager","env":[{"name":"ELEMENTAL_API_URL","value":"'${ELEMENTAL_API_URL}'"}]}]}}}}'
    ```

1. Generate `cluster.yaml` config:

    ```bash
    CONTROL_PLANE_ENDPOINT_IP=192.168.122.100 clusterctl generate cluster \
    --infrastructure elemental:v0.0.1 \
    --flavor k3s-single-node \
    --kubernetes-version v1.28.2 \
    elemental-cluster-k3s > $HOME/elemental-cluster-k3s.yaml
    ```

1. Apply `elemental-cluster-k3s.yaml` config:

    ```bash
    kubectl apply -f $HOME/elemental-cluster-k3s.yaml
    ```

1. Create a new `ElementalRegistration`:

    ```bash
    cat << EOF | kubectl apply -f -
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: ElementalRegistration
    metadata:
      name: my-registration
      namespace: default
    spec:
      config:
        elemental:
          agent:
            hostname:
              useExisting: false
              prefix: "m-"
            debug: true
            installer: "unmanaged"
            insecureAllowHttp: true
    EOF
    ```

## Elemental Host configuration

For more information on how to configure and use the agent, please read the [docs](../cmd/agent/README.md).

1. Install the agent:  

    ```bash
    curl -L https://github.com/rancher-sandbox/cluster-api-provider-elemental/releases/downloads/v0.0.1/elemental_agent_linux_amd64 -o elemental-agent
    mv ./elemental-agent /usr/local/sbin/elemental-agent
    ```

1. Generate the initial agent config file:  

    ```bash
    mkdir -p /etc/elemental/agent

    cat << EOF > /etc/elemental/agent/config.yaml
    agent:
      debug: true
      insecureAllowHttp: true
      reconciliation: 10s
    registration:
      uri: http://192.168.122.10:30009/elemental/v1/namespaces/default/registrations/my-registration
    EOF
    ```

1. Install Elemental:

    ```bash
    elemental-agent --install
    ```

1. Run the agent:

    ```bash
    elemental-agent
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

Whenever one of the reset conditions triggers, the `elemental-agent` using an `unmanaged` installer will create a `needs.reset` sentinel file in the configured `workDir`.  

## Resetting a Host

Using the `unmanaged` installer, the host administrator needs to delete the `needs.reset` sentinel file before being able to reset the host.  
The `k3s` components also need to be deleted.  

**Note**: If using the `hostname.useExisting: true` agent option in combination with prefixes, you should reset the machine hostname to its original value **after** calling `elemental-agent --reset`.  

  ```bash
  k3s-uninstall.sh
  
  rm /var/lib/elemental/agent/needs.reset

  elemental-agent --reset

  hostnamectl set-hostname my-bare-metal-host
  ```

<!-- This part is not really working yet: https://github.com/cluster-api-provider-k3s/cluster-api-k3s/issues/55 -->
<!-- 1. Configure `clusterctl` to use the custom providers:

    ```bash
    mkdir -p $HOME/.cluster-api 

    cat << EOF > $HOME/.cluster-api/clusterctl.yaml
    providers:
    - name: "elemental"
      url: "https://github.com/rancher-sandbox/cluster-api-provider-elemental/releases/v0.0.1/infrastructure-components.yaml"
      type: "InfrastructureProvider"
    - name: "k3s"
      url: "https://github.com/cluster-api-provider-k3s/cluster-api-k3s/releases/v0.1.7/bootstrap-components.yaml"
      type: "BootstrapProvider"
    - name: "k3s"
      url: "https://github.com/cluster-api-provider-k3s/cluster-api-k3s/releases/v0.1.7/control-plane-components.yaml"
      type: "ControlPlaneProvider"
    EOF
    ``` -->
