# OS Upgrades Demo (2024-04-15)

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
CONTROL_PLANE_ENDPOINT_HOST=192.168.122.50 \
VIP_INTERFACE=enp1s0 \
clusterctl generate cluster \
--control-plane-machine-count=1 \
--worker-machine-count=2 \
--infrastructure elemental \
--flavor kubeadm \
kubeadm > ~/kubeadm-cluster-manifest.yaml

kubectl apply -f ~/kubeadm-cluster-manifest.yaml
```

## 4. Initialize the Kubeadm cluster with a CNI

On the control plane node:

```bash
export KUBECONFIG=/etc/kubernetes/super-admin.conf

kubectl create -f https://raw.githubusercontent.com/projectcalico/calico/v3.27.2/manifests/tigera-operator.yaml

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

## 4. Downscale MachineDeployment

Since we don't have a +1 idle host to start the rollout with, we have to downscale by 1 first so that one ElementalHost will reset and become available

```bash
kubectl patch machinedeployment kubeadm-md-0 -p '{"spec":{"replicas":1}}' --type=merge
```

## 5. Trigger a CAPI MachineDeployment rollout

First update the ElementalMachineTemplate with the desired OS version:

```bash
kubectl patch elementalmachinetemplate kubeadm-md-0 -p '{"spec":{"template":{"spec":{"osVersionManagement":{"osVersion":{"imageUri":"oci://192.168.122.10:30000/elemental-os:dev-next"}}}}}}' --type=merge
```

Trigger the rollout:

```bash
clusterctl alpha rollout restart machinedeployment/kubeadm-md-0
```

Wait for it to finish:

```bash
kubectl get machines -w
```

## 6. Upscale MachineDeployment

```bash
kubectl patch machinedeployment kubeadm-md-0 -p '{"spec":{"replicas":2}}' --type=merge
```

## 7. Mock in-place upgrade on one Host

Build an in-place upgrade OS image

```bash
GIT_COMMIT="in-place-upgraded-version" make build-os-kubeadm
```

Push the image to the test registry

```bash
docker image tag docker.io/library/elemental-os:dev-kubeadm 192.168.122.10:30000/elemental-os:dev-next-in-place
docker push 192.168.122.10:30000/elemental-os:dev-next-in-place
```

Update the ElementalMachine OSVersion

```bash
kubectl patch elementalmachine my-associated-elemental-machine -p '{"spec":{"osVersionManagement":{"osVersion":{"imageUri":"oci://192.168.122.10:30000/elemental-os:dev-next-in-place"}}}}' --type=merge
```

Confirm the OSVersion was correctly propagated to the associated host

```bash
kubectl describe elementalhost my-to-be-upgraded-host
```

Drain the selected node (on the control plane node)

```bash
kubectl drain --ignore-daemonsets my-to-be-upgraded-host
```

Mark it as in-place-upgradable (on the management cluster):

```bash
kubectl label elementalhost my-to-be-upgraded-host elementalhost.infrastructure.cluster.x-k8s.io/in-place-upgrade=pending
```

After successful reboot, uncordon the node (on the control plane node)

```bash
kubectl uncordon my-to-be-upgraded-host
```
