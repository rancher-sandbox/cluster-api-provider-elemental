# Rancher Turtles Integration Demo

## Preparation

1. Deploy 1 machine for Rancher deployment.  
   This depends on `git`, `make`, `go`, `docker`, `kind`, `helm`, `kubectl`, and `clusterctl`.

    ```bash
    # Install dependencies
    zypper install -y git make go docker helm kubernetes1.27-client

    # Install kind
    [ $(uname -m) = x86_64 ] && curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
    chmod +x ./kind
    mv ./kind /usr/local/bin/kind

    # Install clusterctl
    curl -L https://github.com/kubernetes-sigs/cluster-api/releases/download/v1.5.2/clusterctl-linux-amd64 -o clusterctl
    install -o root -g root -m 0755 clusterctl /usr/local/bin/clusterctl

    systemctl enable docker
    systemctl disable firewalld

    # Reboot to please docker
    reboot
    ```

1. Deploy 2 machine to form the downstream cluster.  
   These depend on `git`, `go`, `make`, `containerd`, `kubeadm`, `kubelet`, and `kubectl`.

   ```bash
   # Install dependencies
   zypper install -y git make go iproute2 iptables conntrack-tools containerd patterns-kubernetes-kubeadm

   # Enable br_netfilter module
   modprobe br_netfilter
   echo "br_netfilter" > /etc/modules-load.d/99_kubernetes.conf

   # Install CNI plugins
   mkdir -p /opt/cni/bin
   wget https://github.com/containernetworking/plugins/releases/download/v1.3.0/cni-plugins-linux-amd64-v1.3.0.tgz
   tar -xf cni-plugins-linux-amd64-v1.3.0.tgz -C /opt/cni/bin

   systemctl enable containerd
   systemctl start containerd
   systemctl enable kubelet
   systemctl start kubelet
   systemctl disable firewalld
   systemctl stop firewalld
   ```

The network layout assumes `192.168.122.10` is assigned for management/Rancher deployment.  
`192.168.122.100` and `192.168.122.101` assigned to the other 2 host machines.  
`kube-vip` will be configured to provide a load balancer for the downstream cluster's control-plane at `192.168.122.50`.  

If using `libvirt` you may follow this Network example.  
Consider updating the `host mac` addresses to the ones of your Virtual Machines.  

```XML
<network connections="1">
  <name>default</name>
  <uuid>02ec693c-b06f-4926-936f-9b6cbaed0606</uuid>
  <forward mode="nat">
    <nat>
      <port start="1024" end="65535"/>
    </nat>
  </forward>
  <bridge name="virbr0" stp="on" delay="0"/>
  <mac address="52:54:00:12:82:84"/>
  <ip address="192.168.122.1" netmask="255.255.255.0">
    <dhcp>
      <range start="192.168.122.2" end="192.168.122.254"/>
      <host mac="52:54:00:9c:e7:99" name="management" ip="192.168.122.10"/>
      <host mac="52:54:00:e6:7b:55" name="host-1" ip="192.168.122.100"/>
      <host mac="52:54:00:3c:ad:3c" name="host-2" ip="192.168.122.101"/>
    </dhcp>
  </ip>
</network>
```

## Rancher Server setup

The following steps must be executed on the machine reserved to run the Rancher (and CAPI) infrastructure.  

1. On the Rancher machine, initialize a cluster:

    ```bash
    cat << EOF > kind-rancher.yaml
    kind: Cluster
    apiVersion: kind.x-k8s.io/v1alpha4
    nodes:
    - role: control-plane
      image: kindest/node:v1.26.4
      kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
      extraPortMappings:
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      - containerPort: 443
        hostPort: 443
        protocol: TCP
      - containerPort: 30009
        hostPort: 30009
        protocol: TCP
    EOF
    ```

    ```bash
    kind create cluster --config=kind-rancher.yaml
    ```

1. Install the following Helm repositories:

    ```bash
    helm repo add rancher-latest https://releases.rancher.com/server-charts/latest
    helm repo add jetstack https://charts.jetstack.io
    helm repo update
    ```

1. Install nginx-ingress controller:

    ```bash
    kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
    ```

1. Install cert-manager:

    ```bash
    helm upgrade --install \
    cert-manager jetstack/cert-manager \
    --namespace cert-manager \
    --create-namespace \
    --version v1.13.0 \
    --set installCRDs=true
    ```

1. Clone the `cluster-api-provider-elemental` repository and cd into it:

    ```bash
    git clone --branch main https://github.com/rancher-sandbox/cluster-api-provider-elemental.git
    cd cluster-api-provider-elemental
    ```

1. Configure `clusterctl` to use local release files:

    **Note:** This step assumes your local repository is located in `$HOME/cluster-api-provider-elemental` .  
    If you have it in a different location, you can change the **url** in the snippet below.

    ```bash
    mkdir -p $HOME/.cluster-api 

    cat << EOF > $HOME/.cluster-api/clusterctl.yaml
    providers:
      # add a custom provider
      - name: "elemental"
        url: "file:///${HOME}/cluster-api-provider-elemental/infrastructure-elemental/v0.0.1/infrastructure-components.yaml"
        type: "InfrastructureProvider"
    EOF
    ```

1. Build and load the Elemental CAPI operator into the kind local registry:

    ```bash
    make kind-load
    ```

1. Install CAPI controllers, Kubeadm providers, and the Elemental provider:

    ```bash
    clusterctl init --infrastructure elemental:v0.0.1
    ```

1. Install Rancher

    ```bash
    helm upgrade --install rancher rancher-latest/rancher \
    -n cattle-system \
    --set features=embedded-cluster-api=false \
    --set hostname=192.168.122.10.sslip.io \
    --set version=2.7.6 \
    --set namespace=cattle-system \
    --set bootstrapPassword=admin \
    --set replicas=1 \
    --create-namespace \
    --wait
    ```

1. Clone the Rancher Turtles repository and cd into it:

    ```bash
    git clone --branch deploy-shenanigans https://github.com/anmazzotti/rancher-turtles.git
    cd rancher-turtles
    ```

1. Install Rancher Turtles

    ```bash
    make deploy
    ```

1. Mark the namespace for auto-import:

    ```bash
    kubectl label namespace default cluster-api.cattle.io/rancher-auto-import=true
    ```

1. Generate `cluster.yaml` config:

    ```bash
    CONTROL_PLANE_ENDPOINT_IP=192.168.122.50 \
    CONTROL_PLANE_INTERFACE=enp1s0 \
    clusterctl generate cluster \
    --infrastructure elemental:v0.0.1 \
    --kubernetes-version v1.27.4 \
    --control-plane-machine-count 1 \
    --worker-machine-count 1 \
    elemental-cluster > $HOME/cluster.yaml
    ```

1. Apply `cluster.yaml` config:

    ```bash
    kubectl apply -f $HOME/cluster.yaml
    ```

1. Apply Demo manifest:

    ```bash
    cat << EOF > demo-manifest.yaml
    apiVersion: v1
    kind: Service
    metadata:
      name: elemental-manager-service
      namespace: elemental-system
    spec:
      type: NodePort
      selector:
        control-plane: controller-manager
      ports:
      - nodePort: 30009
        port: 9090
        protocol: TCP
        targetPort: 9090
    ---
    apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
    kind: ElementalMachineRegistration
    metadata:
      name: my-registration
      namespace: default
    spec:
      config:
        elemental:
          registration:
            hostname:
              useExisting: true
              prefix: "demo-"
    EOF
    ```

    ```bash
    kubectl apply -f demo-manifest.yaml
    ```

1. Ensure that all pods are up and running:

    ```bash
    kubectl get pods --all-namespaces
    NAMESPACE                           NAME                                                             READY   STATUS      RESTARTS   AGE
    capi-kubeadm-bootstrap-system       capi-kubeadm-bootstrap-controller-manager-5f5969fbc5-5755g       1/1     Running     0          32m
    capi-kubeadm-control-plane-system   capi-kubeadm-control-plane-controller-manager-69d9b87d88-9g2f5   1/1     Running     0          32m
    capi-system                         capi-controller-manager-c54448c74-rfrz8                          1/1     Running     0          32m
    cattle-fleet-local-system           fleet-agent-6bbf74f978-2tldt                                     1/1     Running     0          28m
    cattle-fleet-system                 fleet-controller-79bdbcd9b4-9rxnb                                1/1     Running     0          31m
    cattle-fleet-system                 gitjob-85b85d5df8-wwz6k                                          1/1     Running     0          31m
    cattle-system                       helm-operation-8q4vx                                             0/2     Completed   0          31m
    cattle-system                       helm-operation-dcxpb                                             0/2     Completed   0          30m
    cattle-system                       helm-operation-fjgcx                                             0/2     Completed   0          29m
    cattle-system                       helm-operation-m7rlt                                             0/2     Completed   0          30m
    cattle-system                       helm-operation-s88jj                                             0/2     Completed   0          30m
    cattle-system                       rancher-65b464bcf9-cg8r2                                         1/1     Running     0          32m
    cattle-system                       rancher-webhook-dbfbc89d6-flbq2                                  1/1     Running     0          30m
    cert-manager                        cert-manager-64d969474b-t6hnp                                    1/1     Running     0          33m
    cert-manager                        cert-manager-cainjector-646d9649d9-gnls2                         1/1     Running     0          33m
    cert-manager                        cert-manager-webhook-5995b68bf7-9kpcn                            1/1     Running     0          33m
    elemental-system                    elemental-controller-manager-5c4d5f499c-ln9zb                    2/2     Running     0          32m
    ingress-nginx                       ingress-nginx-admission-create-jrrsd                             0/1     Completed   0          34m
    ingress-nginx                       ingress-nginx-admission-patch-5twcn                              0/1     Completed   1          34m
    ingress-nginx                       ingress-nginx-controller-68d6bfc9b-zjzdt                         1/1     Running     0          34m
    kube-system                         coredns-787d4945fb-b977m                                         1/1     Running     0          34m
    kube-system                         coredns-787d4945fb-rt76x                                         1/1     Running     0          34m
    kube-system                         etcd-kind-control-plane                                          1/1     Running     0          34m
    kube-system                         kindnet-mljpb                                                    1/1     Running     0          34m
    kube-system                         kube-apiserver-kind-control-plane                                1/1     Running     0          34m
    kube-system                         kube-controller-manager-kind-control-plane                       1/1     Running     0          34m
    kube-system                         kube-proxy-rp6h7                                                 1/1     Running     0          34m
    kube-system                         kube-scheduler-kind-control-plane                                1/1     Running     0          34m
    local-path-storage                  local-path-provisioner-6bd6454576-gbhgm                          1/1     Running     0          34m
    rancher-turtles-system              rancher-turtles-controller-manager-7fcd547647-mtg84              1/1     Running     0          14m
    ```

1. Ensure that Rancher is up and running at <https://192.168.122.10.sslip.io/>

1. Ensure the CAPI Machines have been correctly created:

    ```bash
    kubectl get machines -o wide
    NAME                                    CLUSTER             NODENAME   PROVIDERID   PHASE          AGE   VERSION
    elemental-cluster-control-plane-nkm6q   elemental-cluster                           Provisioning   9s    v1.27.4
    elemental-cluster-md-0-drwfz-d8d5n      elemental-cluster                           Pending        39s   v1.27.4
    ```

## Elemental Hosts preparation

Apply the following steps to each machine.

1. Clone the `cluster-api-provider-elemental` repository and cd into it:

    ```bash
    git clone --branch main https://github.com/rancher-sandbox/cluster-api-provider-elemental.git
    cd cluster-api-provider-elemental
    ```

1. Build the agent binary:

    ```bash
    make build-agent
    ```

1. Create the agent config file:

    ```bash
    mkdir -p /oem/elemental/agent

    cat << EOF > /oem/elemental/agent/config.yaml
    registration:
      uri: http://192.168.122.10:30009/elemental/v1/namespaces/default/registrations/my-registration
    agent:
      debug: true
      reconciliation: 10s
    EOF
    ```

1. Start the agent:

    ```bash
    ./bin/agent
    ```

1. On the `control-plane` machine only, install `flannel` once `kubeadm init` finished successfully:

    ```bash
    KUBECONFIG=/etc/kubernetes/admin.conf kubectl apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml
    ```

1. On the `control-plane` node, trust the Rancher self-signed CA.  
   Note that the command below will be different, to check which URL to use visit the Rancher's dashboard:  
   <https://192.168.122.10.sslip.io/dashboard/c/_/manager/provisioning.cattle.io.cluster>

    ```bash
    curl --insecure -sfL https://192.168.122.10.sslip.io/v3/import/vhfqz7phtgd92wv6hhtlg5fcc6jchg4png8bnjlpz87hbvrlg6rx66_c-m-4sp59cf5.yaml | KUBECONFIG=/etc/kubernetes/admin.conf kubectl apply -f -
    ```

## Check Results

If everything worked correctly, you should be able to see the imported cluster in the Rancher dashboard:  

![imported cluster](images/imported-capi-cluster-png)
