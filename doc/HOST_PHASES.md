# Host Phases

The `ElementalHost.status.phase` describes the current phase the host is going through.  
Host phases can be used to quickly determine what any `ElementalHost` is doing, for example if it is still installing, resetting, or just running normally.  

All `ElementalHosts` are following the same phases, however the implementation of each phase heavily depends on the [OSPlugin](./ELEMENTAL_AGENT.md#plugins) in use.  
Please refer to the plugin documentation to learn about specific details. For example the [Elemental plugin](./PLUGIN_ELEMENTAL.md).  

## Phases

Phases are normally executed in the documented order, but this might not always be the case.  
For example a host can be reset just after installation, or some hosts may never use the `Bootstrapping` phase, if not associated to any CAPI Machine.  

Most importantly, during normal operation the `Triggering Reset` phase is prioritized, so that in case of any errors, the user has the possibility to execute a host reset, to fix issues, for example to recover from a failed bootstrap.  

### Registering

The `Registering` phase is the first ever phase for any host.  
During this phase the `elemental-agent` picks a new hostname (according to the `ElementalRegistration` configuration), creates a new key pair for identification, and attempts to create a new `ElementalHost` on the management cluster using the [Elemental API](./ELEMENTAL_API_SETUP.md).  
If an `ElementalHost` with the same name and public key already exists, the `elemental-agent` will consider the registration already done. This allows to attempt registration multiple times, **when not using random hostnames**.  

Normally the `Registering` phase is very short living and transitory.  
Upon successful registration, the `elemental-agent` will automatically execute the `Finalizing Registration` phase.  

This phase is ran using the `elemental-agent register` command.  

### Finalizing Registration

The `Finalizing Registration` immediately follows the `Registration` phase when `elemental-agent register` is used.  

During this phase the `elemental-agent` will take the following steps:  

1. Install the registered hostname. (OSPlugin dependent)

1. Fetch the remote `ElementalRegistration` and use it to install a new agent config file. (OSPlugin dependent)  
**Note:** During this phase the current agent config path is used to determine the install location. This is important if you are using a custom path (ex. `elemental-agent run --config /my/custom/config.yaml`) and you will need to change it later. Migration of this file is going to be needed and depending on how the `OSPlugin` installs files (for ex. in an immutable system), the migration strategy may differ.  

1. Install the generated private key used for the host registration. (OSPlugin dependent)
The private key is used by the `elemental-agent` for authentication and is going to be installed in the agent `workDir` (from the `ElementalRegistration` derived config) under the `private.key` filename.  

### Installing

The `Installing` phase first installs the provided cloud-init config from the `ElementalRegistration.spec.cloudConfig`.  
Finally it installs the system.  

Both steps are heavily `OSPlugin` dependent.  
The `ElementalRegistration.spec.elemental.install` is a schemaless field that can be used to pass arbitrary data to the `OSPlugin`, when executing this phase.  

This phase is ran using the `elemental-agent install` command.  
Note that if `elemental-agent register --install` is used instead, this phase will happen automatically after the registration has been finalized.  

### Bootstrapping

The `Bootstrapping` phase happens whenever an `ElementalHost` is associated to an `ElementalMachine`.  
This is part of the CAPI bootstrap process.  
The `elemental-agent` evaluates the correct bootstrapping of an `ElementalHost` confirming the presence of the `/run/cluster-api/bootstrap-success.complete` [sentinel file](https://cluster-api.sigs.k8s.io/developer/providers/bootstrap#sentinel-file).  

If the sentinel file is not found, the `elemental-agent` will invoke the `OSPlugin` to apply the bootstrap config on the system.  
If the application is successful, the `elemental-agent` will reboot the system to execute the bootstrap config at boot stage.  

Note that the `OSPlugin` can also return an error during this step if it determines that bootstrap was already applied, for example after the reboot.  
This will lead to the `ElementalHost` being stuck in the `Bootstrapping` phase with an error status condition, requiring human intervention to determine and solve the cause of bootstrap failure.  

The bootstrap configuration is dependent on the [CAPI Bootstrap Provider](https://cluster-api.sigs.k8s.io/reference/providers#bootstrap) in use.  

### Running

The `Running` phase defines the normal operation of the `ElementalHost`.  
An `ElementalHost` not associated to any `ElementalMachine` and not bootstrapped yet, may simply be `Running` idle, waiting for association.  

If association already happened, then the `ElementalHost` is `Running` without issues and performing its normal operations.  

### Trigger Reset

The `Trigger Reset` phase happens in the following cases:

- The `ElementalHost` is associated to an `ElementalMachine` and the `ElementalMachine` is deleted (for ex. during [machine rollout](https://cluster-api.sigs.k8s.io/tasks/upgrading-clusters#how-to-schedule-a-machine-rollout))  
- The `ElementalHost` is associated to an `ElementalMachine` belonging to a CAPI `Cluster` and the entire `Cluster` is deleted.  This will lead to the `ElementalMachine` deletion.  
- The `ElementalHost` is directly deleted. If the `ElementalHost` was associated to an `ElementalMachine`, a new available `ElementalHost` will be picked as replacement.  

During this phase, the `elemental-agent` will inform the `OSPlugin` that reset has been triggered.  
Implementation details are plugin dependent, this is the occasion for the plugin to stop services and do anything needed to prepare the system for a reset.  

The `elemental-agent` will exit once reset has been triggered.  
It is expected to run `elemental-agent reset` after, to actually perform the reset of the host.  

### Resetting

The `Resetting` phase happens when `elemental-agent reset` is ran.  

The `elemental-agent` will first delete the remote `ElementalHost`.  
This will add a `deletionTimestamp` to the remote resource, however a finalizer will prevent deletion until reset is deemed successful.  

After deleting the remote `ElementalHost`, the `elemental-agent` will fetch the remote `ElementalRegistration` and pass the schemaless `ElementalRegistration.spec.elemental.reset` field to the `OSPlugin` to perform reset.  

If the `OSPlugin` resets the host successfully, the remote `ElementalHost` is updated one last time to highlight reset has been completed. This will allow the deletion of the `ElementalHost`.  

It is expected to re-start the lifecycle of the host at this point if desired.  
This means running `elemental-agent register --install` to perform a new registration and a fresh installation of the system.  

### Reconciling OS Version

The `Reconciling OS Version` happens during the [Running](#running) phase, if a new OS Version has to be reconciled **and** the host needs to reboot to apply it.  

Note that if the `ElementalHost` does not need to reboot to reconcile an OS Version, then this phase will not be shown and the `ElementalHost` last applied OS Version will be considered reconciled already.  

The `OSPlugin` in use determines whether the host needs a reboot or not, for example to run a new kernel, or to boot from an updated partition.  

For more information, you can read the related [documentation](./OS_VERSION_RECONCILE.md).  
