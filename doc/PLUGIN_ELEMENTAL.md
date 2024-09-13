# Elemental Plugin

The Elemental plugin leverages the [elemental-toolkit](https://rancher.github.io/elemental-toolkit/) to offer a fully managed OS experience.  

## Phases

The `elemental-agent` goes through several [phases](../../doc/HOST_PHASES.md), normally in the documented order.  
Failures during any of the phases should be represented within the Status of the related `ElementalHost` on the CAPI management cluster.
For example:

```bash
kubectl describe elementalhost my-host

Status:
  Conditions:
    Last Transition Time:  2023-12-07T13:25:42Z
    Message:               installing host: invoking elemental install: running elemental install: exit status 1
    Reason:                InstallationFailed
    Severity:              Error
    Status:                False
    Type:                  InstallationReady
```

### 1. Finalizing Registration

When running `elemental-agent register`, upon successful registration, the plugin will be invoked by the agent to perform the following actions:

- Set the chosen hostname on the host.  
- Install the agent config in the work directory: `/oem/elemental/agent/config.yaml`
- Install the agent private key in the work directory: `/oem/elemental/agent/private.key`

For each action, the elemental plugin will create an `elemental-toolkit` cloud-init file in the `/oem` directory.  

### 2. Installing

When running `elemental-agent install`, assuming registration was performed successfully, the plugin will be invoked to:

- Apply the `cloudConfig` from the remote `ElementalRegistration`.
- Invoke `elemental install` using the remote `ElementalRegistration` `spec.config.elemental.install` config.

In a default elemental installation, the `register` and `install` phases are executed together by the `elemental-agent-install` service from the `Elemental recovery` partition.  
To debug installation issues you can run: `journalctl -xeu elemental-agent-install` on the host.  

Note that when installing, the plugin will only invoke `elemental install` from a live system.  
When installing from a recovery partition, the plugin will take no install action as the host is assumed to be already installed.  

### 3. Bootstrapping

When the `elemental-agent` receives a CAPI bootstrap config, the plugin will be invoked to apply it to the host.  
Note that the elemental toolkit only supports `cloud-config` formatted bootstraps.  

The plugin will take the `cloud-config` formatted input and will convert it to an `elemental-toolkit` cloud-init file: `/oem/bootstrap-cloud-config.yaml`.  
The original config will be applied during the [network](https://rancher.github.io/elemental-toolkit/docs/customizing/stages/#network) stage.  
Upon successful bootstrap, the config is expected to create the `/run/cluster-api/bootstrap-success.complete` sentinel file, as described by the [Bootstrap contract](https://cluster-api.sigs.k8s.io/developer/providers/bootstrap#sentinel-file).  
The converted config will also include a self-delete command, executed during the `network.after` stage, to ensure that the bootstrap will not be applied twice on the system:  

```yaml
network.after:
    - if: '[ -f "/run/cluster-api/bootstrap-success.complete" ]'
      commands:
        - rm /oem/bootstrap-cloud-config.yaml
      
```  

Under normal cirsumstances, the `/oem/bootstrap-cloud-config.yaml` should be deleted after the host is rebooted to apply the config.  
However, if the bootstrap application fails for any reason, you will be able to review the file to investigate issues.  
Additionally, you can manually execute and debug the bootstrap configuration by running: `elemental --debug run-stage network`  

### 4. Reconciling OS Version

[OS Version Reconcile](./OS_VERSION_RECONCILE.md) can happen before bootstrap, for example on idling hosts or during CAPI Machine rollouts.  
Most likely however reconciling an OS version will be a part of a bootstrapped node lifecycle. This can be achieved with the [in-place-updates](./OS_VERSION_RECONCILE.md#in-place-updates) functionality.  

In any case, whenever an OS Version needs to be reconciled, the plugin will invoke `elemental updgrade` to apply a newer image to the system, and trigger a reboot.  

The supported `osVersionManagement` schema is as follow:

```yaml
apiVersion: v1
items:
  - apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: ElementalHost
    metadata:
      name: m-9a2c3f22-0bd7-4f44-8aad-24e596a0b1d7
      namespace: default
    spec:
      osVersionManagement:
        osVersion:
          # Enables debug logging when invoking elemental
          debug: false
          # Also upgrades the Recovery partition together with the system
          upgradeRecovery: false
          # Image to apply
          imageUri: oci://192.168.122.10:30000/elemental-capi-os:v1.2.3
```

Also note that the plugin will take a hash of the entire `osVersionManagement` content, to determine whether an OS Version has been applied or not. The hash will be used as a `correlationID` to mark the newly created snapshot.  
This can be inspected on the host by running `elemental state`:

```bash
m-9a2c3f22-0bd7-4f44-8aad-24e596a0b1d7:~ # elemental state
date: "2024-09-13T12:52:22Z"
snapshotter:
    type: btrfs
    max-snaps: 4
    config: {}
efi:
    label: COS_GRUB
oem:
    label: COS_OEM
persistent:
    label: COS_PERSISTENT
recovery:
    label: COS_RECOVERY
    recovery:
        source: dir:///run/rootfsbase
        fs: squashfs
        date: "2024-09-13T12:46:00Z"
        fromAction: install
state:
    label: COS_STATE
    snapshots:
        1:
            source: dir:///run/rootfsbase
            date: "2024-09-13T12:46:00Z"
            fromAction: install
        2:
            source: oci://192.168.122.10:30000/elemental-capi-os:v1.2.3
            digest: sha256:7d0085aa8006b03d5697dd53e2967d9997bb65d76e45ea6ae23391628d168792
            active: true
            labels:
                correlationID: 8d138c258216ce8b6eb749d2d107174dbebd56e0cb273bcad8eea31bf1f6476f
            date: "2024-09-13T12:52:22Z"
            fromAction: upgrade
```

### 5. Trigger Reset

When the `elemental-agent` receives a reset trigger, for example because the CAPI Cluster was deleted, the plugin will take the following actions:

- Write the following reset plan in `/oem/reset-cloud-config.yaml`:

    ```yaml
    name: Elemental Reset
    stages:
        network:
            - if: '[ -f /run/elemental/recovery_mode ]'
            name: Runs elemental reset and re-register the system
            commands:
                - elemental-agent reset  --debug --config /oem/elemental/agent/config.yaml
                - elemental-agent register --debug  --install --config /oem/elemental/agent/config.yaml
                - reboot -f
    ```

- Configure `grub` to use the Recovery partition at the next boot: `grub2-editenv /oem/grubenv set next_entry=recovery`
- Schedule system reboot (in 1 minute): `shutdown -r +1`  

### 6. Resetting

In a typical Elemental installation, the reset phase is executed from the above mentioned `Elemental Reset` plan.  
When running `elemental-agent reset`, the plugin will make a copy of the agent config in `/tmp/elemental-agent-config.yaml` and then invoke `elemental reset` using the remote `ElementalRegistration` `spec.config.elemental.reset` config.  
If no errors occur, the previously copied agent config is moved back to `/oem/elemental/agent/config.yaml`.  

Similarly to the bootstrap phase, if any issues arise, you can manually execute and debug the reset plan by running: `elemental --debug run-stage network`  

Upon successful reset, the plan should run `elemental-agent register --install` to register a new `ElementalHost` and mark it as installed.  
Finally, the host will reboot to the active partition and be ready for CAPI provisioning.  
