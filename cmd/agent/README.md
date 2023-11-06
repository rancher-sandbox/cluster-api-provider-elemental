# Elemental CAPI Agent

## Usage

1. On a clean host, register and install Elemental:

    ```bash
    elemental-agent --register --install
    ```

    The `--register` argument will pick a new hostname and register a new `ElementalHost` using the Elemental API.  
    Upon successful registration, the remote `ElementalRegistration` is used to update and override the agent config.  

    If `--install` argument is also included (can be used standalone), the agent will then install the machine and flag the `ElementalHost` as **installed**.  
    Any **installed** host is considered ready to be bootstrapped by the Elemental CAPI provider.  

1. Operating normally:  

    ```bash
    elemental-agent
    ```

    During normal operation, the agent will periodically patch the remote `ElementalHost` with current host information.  
    Upon successful patching, the agent may receive instructions from the Elemental API to bootstrap the machine.  
    Eventually, the agent will receive instructions to trigger a reset of this machine.  

1. Resetting the host:  

    ```bash
    elemental-agent --reset
    ```

    When `--reset` is invoked, the agent will trigger the remote `ElementalHost` deletion using the Elemental API.  
    After that the agent will reset the system, and upon successful reset, the remote `ElementalHost` will be patched as **reset**.  
    The Elemental CAPI Provider will delete any `ElementalHost` that was up for deletion, only when also marked as **reset**.  
    This gives a way to track hosts that are supposed to reset, but fail to do it successfully.  

## Config

By default the agent will look for a configuration in: `/etc/elemental/agent/config.yaml`

```yaml
registration:
  # This is the ElementalRegistration URI.
  uri: https://my.elemental.api.endpoint/elemental/v1/namespaces/default/registrations/my-registration
  # The CA certificate to trust, if any
  caCert: |
    -----BEGIN CERTIFICATE-----
    MIIBvjCCAWOgAwIBAgIBADAKBggqhkjOPQQDAjBGMRwwGgYDVQQKExNkeW5hbWlj
    bGlzdGVuZXItb3JnMSYwJAYDVQQDDB1keW5hbWljbGlzdGVuZXItY2FAMTY5NTMw
    MjQ0MjAeFw0yMzA5MjExMzIwNDJaFw0zMzA5MTgxMzIwNDJaMEYxHDAaBgNVBAoT
    E2R5bmFtaWNsaXN0ZW5lci1vcmcxJjAkBgNVBAMMHWR5bmFtaWNsaXN0ZW5lci1j
    YUAxNjk1MzAyNDQyMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE7BzWmM5CljI0
    T5qH13xC4ukIkuaU6sn35B39AWIryvNpzB3Dx1Y0QUnFnktEMwln084OvZ1anO7Z
    zNi7DO4M7KNCMEAwDgYDVR0PAQH/BAQDAgKkMA8GA1UdEwEB/wQFMAMBAf8wHQYD
    VR0OBBYEFISgAh7vrCcMxKZKEikNpWbj20mKMAoGCCqGSM49BAMCA0kAMEYCIQD1
    WhfJrSPzvfWPO73w0MFMBRXZ74Tc24SN6QPBin5LaAIhAM9hidFQ71SZQnPY3Y1I
    JZPkAoVeIOoFDgXvl9MkHBuk
    -----END CERTIFICATE-----
agent:
  # Work directory
  workDir: /var/lib/elemental/agent
  # Hostname settings
  hostname:
    useExisting: false
    prefix: ""
  # Post Install behavior (when running --install)
  postInstall:
    powerOff: false
    reboot: false
  # Post Reset behavior (when running --reset)
  postReset:
    powerOff: false
    reboot: false
  # Add SMBIOS labels (not implemented yet)
  noSmbios: false
  # Enable agent debug logs
  debug: false
  # Which OS plugin to use
  osPlugin: "/usr/lib/elemental/plugins/elemental.so"
  # The period used by the agent to sync with the Elemental API
  reconciliation: 1m
  # Allow 'http' scheme
  insecureAllowHttp: false
  # Skip TLS verification when communicating with the Elemental API
  insecureSkipTLSVerify: false
  # Use the system's cert pool for TLS verification
  useSystemCertPool: false
```

## Plugins

A [Plugin](../../pkg/agent/osplugin/plugin.go) interface is defined to enable OS management customization.  
The `elemental-agent` is expected to always be packaged with the [elemental.so](../../internal/agent/plugin/elemental/elemental.go) and [dummy.so](../../internal/agent/plugin/dummy/dummy.go) plugins in the `/usr/lib/elemental/plugins` directory.  

To build the plugins:  

```bash
CGO_ENABLED=1 go build -buildmode=plugin -o elemental.so internal/agent/plugin/elemental/elemental.go
CGO_ENABLED=1 go build -buildmode=plugin -o dummy.so internal/agent/plugin/dummy/dummy.go
```

### Elemental Plugin

The Elemental plugin leverages the [elemental-toolkit](https://rancher.github.io/elemental-toolkit/) to offer a fully managed OS experience.  
This plugin supports automated workflows to install, operate, and reset any underlying host.  
If you want to try it out, just follow the [quickstart](../../doc/QUICKSTART.md) and build your own iso.

### Dummy Plugin

The Dummy plugin is a very simple plugin, as the name suggests, that can be exploited to automate OS management by external means.  
For example, instead of installing a system when the agent is called with the `--install` argument, this plugin will output the install information from [ElementalRegistration's](../../api/v1beta1/elementalregistration_types.go) `spec.config.elemental.install` into an `install.yaml` file in the agent work directory.  
No further action is taken by the plugin, once the file is created the system will be considered **installed** and ready to be bootstrapped.  
An administrator can implement logic around this expected file, for example leveraging [Systemd's Path Units](https://www.freedesktop.org/software/systemd/man/latest/systemd.path.html).  

When a reset is triggered, the plugin will create a `needs.reset` file in the agent work directory.  
When this file is created, some logic can take place to prepare the machine for reset, delete the `needs.reset` file and start the agent with the `--reset` argument to mark the host as reset.  
In this stage some host services may also be stopped or uninstalled, for example `k3s`.  

Similarly to the installation, a `reset.yaml` in the agent work directory will be created when the agent is called with the `--reset` argument.  
A host is considered successfully **reset** after the file is created.  
The reset will fail if the `needs.reset` file exists. This highlight that the host was not prepared for reset first.  
