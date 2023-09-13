# Just for demo

1. Initialize a cluster:

    ```bash
    kind create cluster --network=capi-demo --config=demo/kind.yaml
    ```

1. Install CAPI controllers and Kubeadm providers:

    ```bash
    clusterctl init
    ```

1. Deploy CAPI Elemental provider:

    ```bash
    make kind-deploy
    ```

1. Apply pre-generated `cluster.yaml` config:

    ```bash
    kubectl apply -f demo/cluster.yaml
    ```

1. Apply Demo manifest:

    ```bash
    kubectl apply -f demo/demo-manifest.yaml
    ```

1. Build the agent container:

    ```bash
    make docker-build-agent
    ```

1. Start the first container and wait for `kubeadm init` to end successfully:

    ```bash
    docker run -d --privileged -h host-1 --name host-1 -ti --tmpfs /run -v /sys/fs/cgroup:/sys/fs/cgroup:rw --cgroupns=host --network=kind docker.io/library/agent:latest
    docker exec -it host-1 /agent
    ```

1. Start the second container and wait for `kubeadm join` to end successfully:

    ```bash
    docker run -d --privileged -h host-2 --name host-2 -ti --tmpfs /run -v /sys/fs/cgroup:/sys/fs/cgroup:rw --cgroupns=host --network=kind docker.io/library/agent:latest
    docker exec -it host-2 /agent
    ```

1. Verify that both CAPI Machine resources are provisioned:

    ```bash
    kubectl get machines -o wide
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

<!---
## FIXME:

1. Generate local release files:

```bash
make generate-local-infra-yaml
```

1. Configure `clusterctl` to use local release files:

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
--->
