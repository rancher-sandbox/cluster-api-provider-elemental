# Elemental CAPI Agent

## Usage

```text
elemental-agent takes care of the entire lifecycle of an Elemental host, 
first boot registration, installation, CAPI bootstrapping, upgrades, and reset.

Usage:
  elemental-agent [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  install     Installs the OS on this Elemental host
  register    Registers this Elemental host to the remote CAPI management cluster
  reset       Resets this Elemental host
  run         Operates this Elemental host according to the remote CAPI conditions
  version     Returns the version of the elemental-agent

Flags:
      --config string   Config file (default is /etc/elemental/agent/config.yaml) (default "/etc/elemental/agent/config.yaml")
      --debug           Enables debug logging
  -h, --help            help for elemental-agent

Use "elemental-agent [command] --help" for more information about a command.
```

1. On a clean host, register and install Elemental:

    ```bash
    elemental-agent register --install
    ```

    The `register` command will pick a new hostname and register a new `ElementalHost` using the Elemental API.  
    Upon successful registration, the remote `ElementalRegistration` is used to update and override the agent config.  

    If `--install` argument is also included, the agent will then install the machine and flag the `ElementalHost` as **installed**.  
    Any **installed** host is considered ready to be bootstrapped by the Elemental CAPI provider.  

    Alternatively it is possible to run the `install` command standalone once registration is successful.

    ```bash
    elemental-agent install
    ```

1. Operating normally:  

    ```bash
    elemental-agent run
    ```

    During normal operation, the agent will periodically patch the remote `ElementalHost` with current host information.  
    Upon successful patching, the agent may receive instructions from the Elemental API to bootstrap the machine.  
    Eventually, the agent will receive instructions to trigger a reset of this machine.  

1. Resetting the host:  

    ```bash
    elemental-agent reset
    ```

    When `reset` command is invoked, the agent will trigger the remote `ElementalHost` deletion using the Elemental API.  
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
  # A valid JWT token to use during registration
  token: eyJhbGciOiJFZERTQSIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJFbGVtZW50YWxSZWdpc3RyYXRpb25SZWNvbmNpbGVyIiwic3ViIjoiaHR0cDovLzE5Mi4xNjguMTIyLjEwOjMwMDA5L2VsZW1lbnRhbC92MS9uYW1lc3BhY2VzL2RlZmF1bHQvcmVnaXN0cmF0aW9ucy9teS1yZWdpc3RyYXRpb24iLCJhdWQiOlsiaHR0cDovLzE5Mi4xNjguMTIyLjEwOjMwMDA5L2VsZW1lbnRhbC92MS9uYW1lc3BhY2VzL2RlZmF1bHQvcmVnaXN0cmF0aW9ucy9teS1yZWdpc3RyYXRpb24iXSwibmJmIjoxNjk5ODY0NzIwLCJpYXQiOjE2OTk4NjQ3MjB9.YQsYZoaZ3tGV6z5aXo1e9LmGdA-wQOtmmpi4yAAfXcqh6_S6iIjgblXqw6koQJCzhBMy2-APPQL0ANEBcAljBQ
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
  osPlugin: /usr/lib/elemental/plugins/elemental.so
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
For in-depth info and troubleshooting, please read the [documentation](./PLUGIN_ELEMENTAL.md)

### Dummy Plugin

The Dummy plugin is a very simple plugin, as the name suggests, that can be exploited to automate OS management by external means.  
You can consult the [documentation](./PLUGIN_DUMMY.md) for more details.
