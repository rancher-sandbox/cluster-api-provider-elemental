# Just for demo

1. Initialize a cluster:

```bash
kind create cluster
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

1. Create ElementalMachineRegistration:

```bash
kubectl apply -f demo/registration.yaml
```

1. Port-forward the Elemental API:

```bash
kubectl -n elemental-system port-forward deployments/elemental-controller-manager 9090
```

1. Create 2 dummy ElementalHosts:

```bash
curl -v -X POST localhost:9090/elemental/v1/namespaces/default/registrations/my-registration/hosts -d '{"name":"host-1"}'
curl -v -X POST localhost:9090/elemental/v1/namespaces/default/registrations/my-registration/hosts -d '{"name":"host-2"}'
```

1. Fake installation complete successfully

```bash
curl -v -X PATCH localhost:9090/elemental/v1/namespaces/default/registrations/my-registration/hosts/host-1 -d '{"installed":true}'
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
```
