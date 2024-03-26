# Just for demo

## 1. Register 3 Elemental Hosts

1. Setup test environment

```bash
./test/scripts/setup_kind_cluster.sh
```

1. Fetch the Registration config

```bash
./test/scripts/print_agent_config.sh -n default -r my-registration
```

1. Build a kubeadm-ready iso

```bash
GIT_COMMIT="initial-version" AGENT_CONFIG_FILE=iso/config/my-config.yaml make build-iso-kubeadm
```

1. Run the iso on 3 VMs and wait for the ElementalHosts to be ready

```bash
kubectl get elementalhosts -w
```

## 2. Upgrade 1 Worker Host

1. Build an upgrade OS image

```bash
GIT_COMMIT="upgraded-version" make build-os-kubeadm
```

1. Push the image to the test registry

```bash
docker image tag docker.io/library/elemental-os:dev-kubeadm 192.168.122.10:30000/elemental-os:dev-next
docker push 192.168.122.10:30000/elemental-os:dev-next
```

1. Update the Host OSVersion

```bash
kubectl patch elementalhost my-host -p '{"spec":{"osVersionManagement":{"osVersion":{"imageUri":"oci://192.168.122.10:30000/elemental-os:dev-next"}}}}' --type=merge
```

## 3. Bootstrap a Kubeadm CAPI cluster (1 Control Plane - 2 Workers)

```bash
clusterctl generate cluster \
--control-plane-machine-count=1 \
--worker-machine-count=2 \
--infrastructure elemental:v0.0.0 \
--flavor kubeadm \
kubeadm > ~/kubeadm-cluster-manifest.yaml

kubectl apply -f ~/kubeadm-cluster-manifest.yaml
```

## 4. Trigger a CAPI Rollout upgrade

## 5. Downscale MachineDeployment

## 6. Wait for Rollout to Finish

## 7. Upscale MachineDeployment

## 8. Mock in-place upgrade on one Host
