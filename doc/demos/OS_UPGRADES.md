# OS Upgrades Demo (2024-04-15)

## Preparation

1. Setup test environment

    ```bash
    ./test/scripts/setup_kind_cluster.sh
    ```

1. Fetch the Registration config

    ```bash
    ./test/scripts/print_agent_config.sh -n default -r my-registration > iso/my-config.yaml
    ```

1. Build the `initial-version` bootable ISO:

    ```bash
    GIT_COMMIT="initial-version" AGENT_CONFIG_FILE=iso/config/my-config.yaml make build-iso-kubeadm
    ```

1. Run the iso on 3 VMs and wait for the ElementalHosts to appear

    Recommended 30GB hard drive for all machines. 4GB memory for the control plane, 3GB for the worker nodes.

    ```bash
    kubectl get elementalhosts -w
    ```

1. Build an `upgraded-version` OS image and push it to the test registry

    ```bash
    GIT_COMMIT="upgraded-version" make build-os-kubeadm
    docker image tag docker.io/library/elemental-os:dev-kubeadm 192.168.122.10:30000/elemental-os:dev-next
    docker push 192.168.122.10:30000/elemental-os:dev-next
    ```

1. Build an `in-place-upgraded-version` upgrade OS image and push it to the test registry

    ```bash
    GIT_COMMIT="in-place-upgraded-version" make build-os-kubeadm
    docker image tag docker.io/library/elemental-os:dev-kubeadm 192.168.122.10:30000/elemental-os:dev-next-in-place
    docker push 192.168.122.10:30000/elemental-os:dev-next-in-place
    ```

1. Generate a kubeadm manifest:

    ```bash
    CONTROL_PLANE_ENDPOINT_HOST=192.168.122.50 \
    VIP_INTERFACE=enp1s0 \
    clusterctl generate cluster \
    --control-plane-machine-count=1 \
    --worker-machine-count=2 \
    --infrastructure elemental \
    --flavor kubeadm \
    kubeadm > ~/kubeadm-cluster-manifest.yaml
    ```

1. (Optional) Edit the `kubeadm-control-plane` MachineTemplate to add a selector and "pin" a desired control-plane host with the matching label.

    ```yaml
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: ElementalMachineTemplate
    metadata:
      name: kubeadm-control-plane
      namespace: default
    spec:
      template:
        spec:
          selector:
            matchLabels:
              my-control-plane: "true"
    ```

    ```bash
    kubectl label elementalhost my-control-plane-host my-control-plane=true
    ```

## Upgrade a single Host

1. Verify the `initial-version` on one Host:

    ```bash
    elemental-agent --version
    Agent version v0.5.0, commit initial-version
    ```

1. Upgrade the to be control plane Elemental host:

    ```bash
    kubectl patch elementalhost my-control-plane-host -p '{"spec":{"osVersionManagement":{"osVersion":{"imageUri":"oci://192.168.122.10:30000/elemental-os:dev-next"}}}}' --type=merge
    ```

    The `elemental-agent` will immediately consume this info and try to reconcile the OS Version, since the host is not bootstrapped yet and considered idle, waiting for association.  
    There is no orchestration at the moment, this could be added later in some `ElementalManagedOSVersion` layer.  

    Also note that `elemental upgrade` is invoked directly from the host. This is a behavior change from the current `elemental-operator`, where the to-be-upgraded Elemental image is used to upgrade the host, running from a container.  
    We can implement a similar behavior by relying on `containerd`. The Elemental OS Plugin could be configured to run the upgrade using a container runtime, or directly like it is now, could be a toggable option.  
    Also note that `elemental upgrade` does not seem to be bothered by insecure registries, this is probably something that needs to be addressed in any case.  

    Finally, we miss a way to run actions before the upgrade takes place and after it completes (this should include boot assessment, so after it). Would be nice to be able to configure could-init configs pre and post upgrade, so that users can for example stop services, drain the Harvester node if this host is part of an Harvester cluster, uncordon it after we boot to the upgraded system, etc.  

1. After reboot, verify the version:

    ```bash
    elemental-agent --version
    Agent version v0.5.0, commit upgraded-version
    ```

## Upgrade all worker Hosts in a Cluster

1. Apply the cluster manifest

    ```bash
    kubectl apply -f ~/kubeadm-cluster-manifest.yaml
    ```

1. Wait for the control-plane machine to be associated:

    ```bash
    kubectl get machines -w
    ```

1. Initialize the Cluster with a CNI. On the control plane host:

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

1. Wait for the all machines to be running:

    ```bash
    kubectl get machines -w
    ```

1. Downscale the machine deployment by 1. We don't have an idle host to start the rollout with, therefore we have to "free" one host and make it reset:

    ```bash
    kubectl patch machinedeployment kubeadm-md-0 -p '{"spec":{"replicas":1}}' --type=merge
    ```

1. Ensure one MachineDeployment machine is deleted:

    ```bash
    kubectl get machines
    ```

1. Patch the `kubeadm-md-0` template with the desired OS image:

    ```bash
    kubectl patch elementalmachinetemplate kubeadm-md-0 -p '{"spec":{"template":{"spec":{"osVersionManagement":{"osVersion":{"imageUri":"oci://192.168.122.10:30000/elemental-os:dev-next"}}}}}}' --type=merge
    ```

    Nothing happens at this point. In-place upgrades are not effective here, so mutating the template will have no effect at all. The underlying machines will not be updated. A rollout will not start.

1. Trigger the MachineDeployment rollout manually:

    ```bash
    clusterctl alpha rollout restart machinedeployment/kubeadm-md-0
    ```

1. Wait for the rollout to finish (1 new MachineDeployment machine should be associated and running, the existing one deleted after):

    ```bash
    kubectl get machines -w
    ```

1. Upscale the MachineDeployment to the original replicas:

    ```bash
    kubectl patch machinedeployment kubeadm-md-0 -p '{"spec":{"replicas":2}}' --type=merge
    ```

1. On both MachineDeployment hosts, verify the agent version is updated:

    ```bash
    elemental-agent --version
    Agent version v0.5.0, commit upgraded-version
    ```

## Mock in-place upgrade on one Host

1. Update one of the MachineDeployment ElementalMachine's OSVersion:

    ```bash
    kubectl patch elementalmachine my-associated-elemental-machine -p '{"spec":{"osVersionManagement":{"osVersion":{"imageUri":"oci://192.168.122.10:30000/elemental-os:dev-next-in-place"}}}}' --type=merge
    ```

    In "in-place" upgrades we expect eventually for one already bootstrapped ElementalMachine to be updated.

1. Confirm the OSVersion was correctly propagated to the associated host

    ```bash
    kubectl describe elementalhost my-to-be-upgraded-host
    ```

    At this point nothing should happen. The OSVersion is propagated to the associated ElementalHost (from the ElementalMachine), but since we are already bootstrapped, the `elemental-agent` is not trying to reconcile the OS Version anymore. We need a special trigger for that.

1. Drain the selected node (on the control plane node)

    ```bash
    kubectl drain --ignore-daemonsets my-to-be-upgraded-host
    ```

    Here we assume that the ControlPlane provider (or the CAPI Core provider?) will drain the node during the in-place rollout execution. Another assumption is that after draining, the external upgrade controller will receive an upgrade request for this machine.

1. Mark it as in-place-upgradable (on the management cluster):

    ```bash
    kubectl label elementalhost my-to-be-upgraded-host elementalhost.infrastructure.cluster.x-k8s.io/in-place-upgrade=pending
    ```

    Here we assume that the Elemental provider (ElementalMachine controller in particular) received the signal to proceed with "in-place" upgrade on this machine. If that is so, we can use a special label that serves as trigger for the `elemental-agent` to proceed anyway (even if bootstrapped) with the upgrade.

    After successful upgrade the `elemental-agent` will set the label to `done`. Highlighting that the process is finished from the host point of view. This should include boot assessment, but it doesn't in this proof of concept.

1. After successful reboot, uncordon the node (on the control plane node)

    ```bash
    kubectl uncordon my-to-be-upgraded-host
    ```

    Here the ElementalMachine controller can verify the `done` in place upgrade, and confirm to the external upgrade controller that the machine was in-place upgraded successfully. We expect at this point that the node will be uncordoned (by the KCP or CAPI Core).

1. Verify the `in-place-upgraded-version` on the upgraded host:

    ```bash
    elemental-agent --version
    Agent version v0.5.0, commit in-place-upgraded-version
    ```
