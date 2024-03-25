# Just for demo

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

1. Run the iso on a VM and wait for the ElementalHost to be ready

```bash
kubectl get elementalhosts -w
```

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
kubectl patch elementalhost my-host -p '{"spec":{"osVersionManagement":{"osVersion":{"imageURI":"oci://192.168.122.10:30000/elemental-os:dev-next"}}}}' --type=merge
```
