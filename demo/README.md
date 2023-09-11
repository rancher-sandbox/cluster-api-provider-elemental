# Just for demo

1. Initialize a cluster:

```bash
kind create cluster --config=demo/kind.yaml
```

1. Install CAPI controllers and Kubeadm providers:

```bash
clusterctl init
```

1. Deploy CAPI Elemental provider:

```bash
make kind-deploy
```
<!--
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
-->
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

1. Start a couple of containers:

```bash
docker run --privileged -h host-1 --name host-1 -ti --tmpfs /run -v /sys/fs/cgroup:/sys/fs/cgroup:rw --cgroupns=host docker.io/library/agent:latest
docker exec -it host-1 /agent


docker run --privileged -h host-2 --name host-2 -ti --tmpfs /run -v /sys/fs/cgroup:/sys/fs/cgroup:rw --cgroupns=host docker.io/library/agent:latest
docker exec -it host-2 /agent
```

<!-- Sep 11 05:49:29 host-1 containerd[97]: time="2023-09-11T05:49:29.101465710Z" level=error msg="RunPodSandbox for &PodSandboxMetadata{Name:etcd-host-1,Uid:a64af1489ea2e74ef941d04c31cd9473,Namespace:kube-system,Attempt:0,} failed, error" error="failed to create containerd task: failed to create shim task: failed to mount rootfs component &{overlay overlay [index=off workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/83/work upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/83/fs lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/9/fs]}: invalid argument: unknown"
Sep 11 05:49:29 host-1 kubelet[489]: E0911 05:49:29.101974     489 remote_runtime.go:176] "RunPodSandbox from runtime service failed" err="rpc error: code = Unknown desc = failed to create containerd task: failed to create shim task: failed to mount rootfs component &{overlay overlay [index=off workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/83/work upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/83/fs lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/9/fs]}: invalid argument: unknown"
Sep 11 05:49:29 host-1 kubelet[489]: E0911 05:49:29.102095     489 kuberuntime_sandbox.go:72] "Failed to create sandbox for pod" err="rpc error: code = Unknown desc = failed to create containerd task: failed to create shim task: failed to mount rootfs component &{overlay overlay [index=off workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/83/work upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/83/fs lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/9/fs]}: invalid argument: unknown" pod="kube-system/etcd-host-1"
Sep 11 05:49:29 host-1 kubelet[489]: E0911 05:49:29.102146     489 kuberuntime_manager.go:1122] "CreatePodSandbox for pod failed" err="rpc error: code = Unknown desc = failed to create containerd task: failed to create shim task: failed to mount rootfs component &{overlay overlay [index=off workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/83/work upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/83/fs lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/9/fs]}: invalid argument: unknown" pod="kube-system/etcd-host-1"
Sep 11 05:49:29 host-1 kubelet[489]: E0911 05:49:29.102262     489 pod_workers.go:1294] "Error syncing pod, skipping" err="failed to \"CreatePodSandbox\" for \"etcd-host-1_kube-system(a64af1489ea2e74ef941d04c31cd9473)\" with CreatePodSandboxError: \"Failed to create sandbox for pod \\\"etcd-host-1_kube-system(a64af1489ea2e74ef941d04c31cd9473)\\\": rpc error: code = Unknown desc = failed to create containerd task: failed to create shim task: failed to mount rootfs component &{overlay overlay [index=off workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/83/work upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/83/fs lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/9/fs]}: invalid argument: unknown\"" pod="kube-system/etcd-host-1" podUID=a64af1489ea2e74ef941d04c31cd9473
Sep 11 05:49:29 host-1 kubelet[489]: I0911 05:49:29.908439     489 csi_plugin.go:913] Failed to contact API server when waiting for CSINode publishing: Get "https://172.17.0.10:6443/apis/storage.k8s.io/v1/csinodes/host-1": dial tcp 172.17.0.10:6443: connect: no route to host -->

<!-- Sep 11 05:58:07 host-1 containerd[125]: time="2023-09-11T05:58:07.104491871Z" level=error msg="RunPodSandbox for &PodSandboxMetadata{Name:etcd-host-1,Uid:a64af1489ea2e74ef941d04c31cd9473,Namespace:kube-system,Attempt:0,} failed, error" error="failed to create containerd task: failed to create shim task: failed to mount rootfs component &{overlay overlay [index=off workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/44/work upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/44/fs lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/9/fs]}: invalid argument: unknown"
Sep 11 05:58:07 host-1 kubelet[521]: E0911 05:58:07.104857     521 remote_runtime.go:176] "RunPodSandbox from runtime service failed" err="rpc error: code = Unknown desc = failed to create containerd task: failed to create shim task: failed to mount rootfs component &{overlay overlay [index=off workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/44/work upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/44/fs lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/9/fs]}: invalid argument: unknown"
Sep 11 05:58:07 host-1 kubelet[521]: E0911 05:58:07.104937     521 kuberuntime_sandbox.go:72] "Failed to create sandbox for pod" err="rpc error: code = Unknown desc = failed to create containerd task: failed to create shim task: failed to mount rootfs component &{overlay overlay [index=off workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/44/work upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/44/fs lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/9/fs]}: invalid argument: unknown" pod="kube-system/etcd-host-1"
Sep 11 05:58:07 host-1 kubelet[521]: E0911 05:58:07.104969     521 kuberuntime_manager.go:1122] "CreatePodSandbox for pod failed" err="rpc error: code = Unknown desc = failed to create containerd task: failed to create shim task: failed to mount rootfs component &{overlay overlay [index=off workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/44/work upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/44/fs lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/9/fs]}: invalid argument: unknown" pod="kube-system/etcd-host-1"
Sep 11 05:58:07 host-1 kubelet[521]: E0911 05:58:07.105057     521 pod_workers.go:1294] "Error syncing pod, skipping" err="failed to \"CreatePodSandbox\" for \"etcd-host-1_kube-system(a64af1489ea2e74ef941d04c31cd9473)\" with CreatePodSandboxError: \"Failed to create sandbox for pod \\\"etcd-host-1_kube-system(a64af1489ea2e74ef941d04c31cd9473)\\\": rpc error: code = Unknown desc = failed to create containerd task: failed to create shim task: failed to mount rootfs component &{overlay overlay [index=off workdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/44/work upperdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/44/fs lowerdir=/var/lib/containerd/io.containerd.snapshotter.v1.overlayfs/snapshots/9/fs]}: invalid argument: unknown\"" pod="kube-system/etcd-host-1" podUID=a64af1489ea2e74ef941d04c31cd9473 -->


<!-- 1. Create 2 dummy ElementalHosts:

```bash
curl -v -X POST localhost:9090/elemental/v1/namespaces/default/registrations/my-registration/hosts -d '{"name":"host-1"}'
curl -v -X POST localhost:9090/elemental/v1/namespaces/default/registrations/my-registration/hosts -d '{"name":"host-2"}'
```

1. Fake installation complete successfully

```bash
curl -v -X PATCH localhost:9090/elemental/v1/namespaces/default/registrations/my-registration/hosts/demo-host-1 -d '{"installed":true}'
curl -v -X PATCH localhost:9090/elemental/v1/namespaces/default/registrations/my-registration/hosts/host-2 -d '{"installed":true}'
```

1. Continue PATCHing both hosts until one receive a response that contains `"bootstrapReady":true`

1. Fetch the bootstrap configs

```bash
curl -v -X GET localhost:9090/elemental/v1/namespaces/default/registrations/my-registration/hosts/host-1/bootstrap
curl -v -X GET localhost:9090/elemental/v1/namespaces/default/registrations/my-registration/hosts/host-2/bootstrap
```

1. Fake bootstrap complete successfully

```bash
curl -v -X PATCH localhost:9090/elemental/v1/namespaces/default/registrations/my-registration/hosts/host-1 -d '{"bootstrapped":true}'
curl -v -X PATCH localhost:9090/elemental/v1/namespaces/default/registrations/my-registration/hosts/host-2 -d '{"bootstrapped":true}'
``` -->
