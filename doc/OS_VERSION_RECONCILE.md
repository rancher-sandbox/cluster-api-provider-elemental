# OS Version Reconcile

The `ElementalHost` API supports user defined "OS Version" schemas.  
The `ElementalHost.spec.osVersionManagement` can be populated with arbitrary information that will be passed to the `elemental-agent` running on the host system, in order to reconcile a desired OS Version state.  

This information can be anything, for example a list of packages to refresh, a set of commands to run, an OCI image to upgrade to. It depends on the [OS Plugin](./ELEMENTAL_AGENT.md#plugins) in use.  

## Upgrading a single host

The `ElementalHost.spec.osVersionManagement` can be configured directly, for example to apply a certain version to a single host.  

Hosts that are not yet bootstrapped will try to reconcile the version on the next `elemental-agent` reconcile loop.  

For example, using the [Elemental plugin](./PLUGIN_ELEMENTAL.md):  

```bash
kubectl patch elementalhost my-elemental-host -p '{"spec":{"osVersionManagement":{"osVersion":{"imageUri":"oci://my-registry/my-image:v1.2.3"}}}}' --type=merge
```

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ElementalHost
metadata:
  labels:
    elementalhost.infrastructure.cluster.x-k8s.io/installed: "true"
  name: my-elemental-host
  namespace: default
spec:
  osVersionManagement:
    osVersion:
      imageUri: oci://my-registry/my-image:v1.2.3
      upgradeRecovery: false
      debug: false
```

If the host will need a reboot due to upgrades (signaled by the OS Plugin in use), then the `ElementalHost` will enter the `Reconciling OS Version` phase. Otherwise the reconcile will be considered successful.  

The `Ready` condition can be can be used to determine whether the Host has been upgraded successfully:

```bash
kubectl wait --for=condition=ready elementalhost my-elemental-host
```

### In-place updates

While the [In-place updates proposal](https://github.com/kubernetes-sigs/cluster-api/pull/11029) is not yet finalized, the Elemental provider offers a rudimentary way of updating `ElementalHosts` that are already bootstrapped and part of a cluster.  

Be aware that this requires you to [safely drain the node](https://kubernetes.io/docs/tasks/administer-cluster/safely-drain-node/) before proceeding.  

The `elementalhost.infrastructure.cluster.x-k8s.io/in-place-update` label with a value of `pending` can be used to tell the `elemental-agent` that the OS Version has to be reconciled, even if the host is already bootstrapped.

```bash
kubectl label elementalhost my-to-be-updated-host elementalhost.infrastructure.cluster.x-k8s.io/in-place-update=pending
```

Since the `ElementalHost` is already associated to an `ElementalMachine`, the OS Version is reconciled from the latter, therefore the `ElementalMachine` needs to be patched instead with the desired version:

```bash
kubectl patch elementalmachine my-elemental-machine -p '{"spec":{"osVersionManagement":{"osVersion":{"imageUri":"oci://my-registry/my-image:v1.2.3"}}}}' --type=merge
```

Once the update is successful, the label will automatically mutate to `done`.

## Upgrade bootstrapped hosts with machine rollouts

Elemental supports upgrading hosts during [machine rollouts](https://cluster-api.sigs.k8s.io/tasks/upgrading-clusters).  

The desired OS Version can be defined on the `ElementalMachineTemplate` prior triggering a rollout to replace nodes.  
For example:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ElementalMachineTemplate
metadata:
  name: kubeadm-md-0
  namespace: default
spec:
  template:
    spec: 
      osVersionManagement:
        osVersion:
          imageUri: oci://my-registry/my-image:v1.2.3
          upgradeRecovery: false
          debug: false        
```

Upon triggering a rollout, new `Machines` will be created to replace the old ones, and during `ElementalMachine` to `ElementalHost` association, the `osVersionManagement` field will be forwarded to the `ElementalHost` to be applied **before** bootstrapping.  

Also note that Elemental hosts will undergo reset when a `Machine` is deleted. Normally this would require you to have at least a spare +1 `ElementalHost` to begin the rollout with, or downscale your node pool by 1 to reset one `ElementalHost` first.

```bash
kubectl patch machinedeployment kubeadm-md-0 -p '{"spec":{"replicas":1}}' --type=merge
```

One way to start a rollout is to trigger it directly:

```bash
clusterctl alpha rollout restart machinedeployment/kubeadm-md-0
```

The nodes can be upscaled again to the desired amount after rollout is finished:

```bash
kubectl patch machinedeployment kubeadm-md-0 -p '{"spec":{"replicas":2}}' --type=merge
```
