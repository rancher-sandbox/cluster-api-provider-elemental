# Dummy Plugin

The Dummy plugin is a very simple plugin, as the name suggests, that can be exploited to automate OS management by external means.  

## Phases

The `elemental-agent` goes through several [phases](../../doc/HOST_PHASES.md), normally in the documented order.  

### 1. Finalizing Registration

When running `elemental-agent register`, upon successful registration, the plugin will be invoked by the agent to perform the following actions:

- Set the chosen hostname on the host. This step relies on: `hostnamectl set-hostname`
- Install the agent config in the work directory: `/var/lib/elemental/agent/config.yaml`
- Install the agent private key in the work directory: `/var/lib/elemental/agent/private.key`

### 2. Installing

When running `elemental-agent install`, this plugin will dump the remote `ElementalRegistration` `spec.config.elemental.install` config into an `install.yaml` file in the agent work directory.  
No further action is taken by the plugin, once the file is created the system will be considered **installed** and ready to be bootstrapped.  
An administrator can implement logic around this expected file, for example leveraging [Systemd's Path Units](https://www.freedesktop.org/software/systemd/man/latest/systemd.path.html).  

### 3. Bootstrapping

When the `elemental-agent` receives a CAPI bootstrap config, the plugin will simply dump the configuration in the following paths:

- `/etc/cloud/cloud.cfg.d/elemental-capi-bootstrap.cfg` if the boostrap format is `cloud-config`
- `/usr/local/bin/ignition/data/elemental-capi-bootstrap.conf` if the boostrap format is `ignition`

The host will then reboot.  
Upon reboot, the bootstrap config is expected to create the `/run/cluster-api/bootstrap-success.complete` sentinel file, as described by the [Bootstrap contract](https://cluster-api.sigs.k8s.io/developer/providers/bootstrap#sentinel-file).  

### 4. OS Version Reconcile

[OS Version Reconcile](./OS_VERSION_RECONCILE.md) can happen before bootstrap, for example on idling hosts or during CAPI Machine rollouts.  
Most likely however reconciling an OS version will be a part of a bootstrapped node lifecycle. This can be achieved with the [in-place-updates](./OS_VERSION_RECONCILE.md#in-place-updates) functionality.  

In any case, whenever an OS Version needs to be reconciled, the plugin will dump the content of the `ElementalHost.spec.osVersionManagement` in the `os-version.yaml` file within the `elemental-agent` work directory.  

The system will not be rebooted and the OS version will be considered reconciled after the file has been written.

### 5. Trigger Reset

When the `elemental-agent` receives a reset trigger, the plugin will create a `needs.reset` file in the agent work directory.  
No further action is taken by the plugin.

When the `needs.reset` file is created, some logic should take place to prepare the machine for reset, delete the `needs.reset` file and start the agent with the `reset` command to mark the host as reset.  
In this stage some host services may also be stopped or uninstalled, for example `k3s`.  

### 6. Resetting

Similarly to the installation, a `reset.yaml` in the agent work directory will be created when the agent is called with the `reset` command.  
This is a simple dump of the `ElementalRegistration` `spec.config.elemental.reset` configuration.

The reset will fail if the `needs.reset` file exists. This highlight that the host was not prepared for reset first.  
A host is considered successfully **reset** after the file is created.  
