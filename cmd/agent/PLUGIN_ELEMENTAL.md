# Elemental Plugin

The Elemental plugin leverages the [elemental-toolkit](https://rancher.github.io/elemental-toolkit/) to offer a fully managed OS experience.  

## Phases

The `elemental-agent` goes through 5 different phases, normally in the documented order.  
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

### 1. (Post) Registration

When running `elemental-agent --register`, upon successful registration, the plugin will be invoked by the agent to perform the following actions:

- Set the chosen hostname on the host.  
- Install the agent config in the work directory: `/oem/elemental/agent/config.yaml`
- Install the agent private key in the work directory: `/oem/elemental/agent/private.key`

For each action, the elemental plugin will create an `elemental-toolkit` cloud-init file in the `/oem` directory.  

### 2. Installation

When running `elemental-agent --install`, assuming registration was performed successfully, the plugin will be invoked to:

- Apply the `cloudConfig` from the remote `ElementalRegistration`.
- Invoke `elemental install` using the remote `ElementalRegistration` `spec.config.elemental.install` config.

In a default elemental installation, the `--register` and `--install` phases are executed together by the `elemental-agent-install` service from the `Elemental recovery` partition.  
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

### 4. Reset trigger

When the `elemental-agent` receives a reset trigger, for example because the CAPI Cluster was deleted, the plugin will take the following actions:

- Write the following reset plan in `/oem/reset-cloud-config.yaml`:

    ```yaml
    name: Elemental Reset
    stages:
        network:
            - if: '[ -f /run/cos/recovery_mode ]'
            name: Runs elemental reset and re-register the system
            commands:
                - elemental-agent --debug --reset --config /oem/elemental/agent/config.yaml
                - elemental-agent --debug --register --install --config /oem/elemental/agent/config.yaml
                - reboot -f
    ```

- Configure `grub` to use the Recovery partition at the next boot: `grub2-editenv /oem/grubenv set next_entry=recovery`
- Schedule system reboot (in 1 minute): `shutdown -r +1`  

### 5. Reset

In a typical Elemental installation, the reset phase is executed from the above mentioned `Elemental Reset` plan.  
When running `elemental-agent --reset`, the plugin will make a copy of the agent config in `/tmp/elemental-agent-config.yaml` and then invoke `elemental reset` using the remote `ElementalRegistration` `spec.config.elemental.reset` config.  
If no errors occur, the previously copied agent config is moved back to `/oem/elemental/agent/config.yaml`.  

Similarly to the bootstrap phase, if any issues arise, you can manually execute and debug the reset plan by running: `elemental --debug run-stage network`  

Upon successful reset, the plan should run `elemental-agent --register --install` to register a new `ElementalHost` and mark it as installed.  
Finally, the host will reboot to the active partition and be ready for CAPI provisioning.  
