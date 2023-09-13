# Just for demo

1. Initialize a cluster:

    ```bash
    kind create cluster --config=demo/kind.yaml
    ```

1. Build the operator docker and load it to kind:

    ```bash
    make kind-load
    ```

1. Generate local release files:

    ```bash
    make generate-infra-yaml
    ```

1. Configure `clusterctl` to use local release files:

    **Note:** This step assumes your local repository is located in `$HOME/repos/cluster-api-provider-elemental` .  
    If you have it in a different location, you can change the **url** in the snippet below.

    ```bash
    mkdir -p $HOME/.cluster-api 

    cat << EOF > $HOME/.cluster-api/clusterctl.yaml
    providers:
      # add a custom provider
      - name: "elemental"
        url: "file:///${HOME}/repos/cluster-api-provider-elemental/infrastructure-elemental/v0.0.1/infrastructure-components.yaml"
        type: "InfrastructureProvider"
    EOF
    ```

1. Install CAPI controllers, Kubeadm providers, and the Elemental provider:

    ```bash
    clusterctl init --infrastructure elemental:v0.0.1
    ```

1. Generate `cluster.yaml` config:

    ```bash
    CONTROL_PLANE_ENDPOINT_IP=172.18.0.10 clusterctl generate cluster \
    --infrastructure elemental:v0.0.1 \
    --flavor docker \
    --kubernetes-version v1.27.4 \
    --control-plane-machine-count 1 \
    --worker-machine-count 1 \
    elemental-cluster > $HOME/cluster.yaml
    ```

1. Apply `cluster.yaml` config:

    ```bash
    kubectl apply -f $HOME/cluster.yaml
    ```

1. Apply Demo manifest:

    ```bash
    kubectl apply -f demo/demo-manifest.yaml
    ```

1. Build the agent container:

    ```bash
    make docker-build-agent
    ```

1. Start a couple of containers and wait for `kubeadm` to initialize successfully:

    ```bash
    docker run -d --privileged -h host-1 --name host-1 -ti --tmpfs /run -v /sys/fs/cgroup:/sys/fs/cgroup:rw --cgroupns=host --network=kind docker.io/library/agent:latest
    docker exec -it host-1 /agent

    docker run -d --privileged -h host-2 --name host-2 -ti --tmpfs /run -v /sys/fs/cgroup:/sys/fs/cgroup:rw --cgroupns=host --network=kind docker.io/library/agent:latest
    docker exec -it host-2 /agent
    ```

1. Verify that both CAPI Machine resources are provisioned:

    ```bash
    kubectl get machines -o wide -w
    ```

    ```text
    NAME                                    CLUSTER             NODENAME   PROVIDERID                        PHASE         AGE     VERSION
    elemental-cluster-control-plane-9582g   elemental-cluster              elemental://default/demo-host-1   Provisioned   4m36s   v1.27.4
    elemental-cluster-md-0-xpq25-lvcs8      elemental-cluster              elemental://default/demo-host-2   Provisioned   4m37s   v1.27.4
    ```

## Cleanup

```bash
kind delete cluster
docker stop host-1 && docker rm host-1
docker stop host-2 && docker rm host-2
```
