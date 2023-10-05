# Quickstart

## Prerequisite

## Management Cluster initialization

1. Initialize a cluster:

    ```bash
    kind create cluster --config test/kind-config.yaml --name elemental-capi-management
    ```

1. Configure `clusterctl` to use the custom providers:

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
    ```

1. Install CAPI controllers, Kubeadm providers, and the Elemental provider:

    ```bash
    clusterctl init --bootstrap k3s --control-plane k3s --infrastructure elemental
    ```

1. Generate `cluster.yaml` config:

    ```bash
    CONTROL_PLANE_ENDPOINT_IP=172.18.0.3 clusterctl generate cluster \
    --infrastructure elemental \
    --flavor k3s-single-node \
    --kubernetes-version v1.27.4 \
    --control-plane-machine-count 1 \
    --worker-machine-count 1 \
    elemental-cluster-k3s > $HOME/elemental-cluster-k3s.yaml
    ```

1. Apply `elemental-cluster-k3s.yaml` config:

    ```bash
    kubectl apply -f $HOME/elemental-cluster-k3s.yaml
    ```

1. Apply the test manifest:

    ```bash
    kubectl apply -f test/test-manifest.yaml
    ```

1. Build the agent container:

    ```bash
    make docker-build-agent
    ```

1. Start one container:

    ```bash
    docker run -d -h elemental-host --name elemental-host --ip 172.18.0.3 -ti --tmpfs /run --tmpfs /var/lib/containerd -v /sys/fs/cgroup:/sys/fs/cgroup:rw --cgroupns=host --network=kind docker.io/library/agent:latest
    ```

1. Install Elemental:

    ```bash
    docker exec -it elemental-host -c 'elemental-agent --install'
    ```

## Cleanup

```bash
kind delete cluster --name elemental-capi-management
docker stop elemental-host && docker rm elemental-host
```
