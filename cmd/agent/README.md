# Elemental CAPI Agent

## Usage

1. On a clean host, install Elemental first:  

    ```bash
    elemental-agent --install
    ```

    When using the `unmanaged` installer, the agent will fetch the remote `ElementalRegistration` to override the local config.  
    Also a call to `hostnamectl` will be triggered to set the hostname according to `ElementalRegistration` config.  
    Finally, a new `ElementalHost` will be created using the selected hostname as unique identifier.  
    It is *not* possible to run `elemental --install` twice on the same machine, before `elemental --reset` has been called.  

1. Operating normally:  

    ```bash
    elemental-agent
    ```

    During normal operation, the agent may receive instructions from the Elemental API to reset this host.  
    When using the `unmanaged` installer, a `reset.needed` sentinel file will be created in the configured work directory (`/var/lib/elemental/agent` by default).  
    The agent will then terminate.  

1. Resetting the host:  

    If using the `unmanaged` installer, the `reset.needed` sentinel file must be deleted before reset.  
    This guarantees the host administrator reset the machine correctly. For example by uninstalling and stopping running services (for example `k3s`), removing configuration files, etc.

    ```bash
    elemental-agent --reset
    ```

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
  # Add SMBIOS labels (not implemented yet)
  noSmbios: false
  # Enable agent debug logs
  debug: false
  # Which OS installer to use. "unmanaged" or "elemental"
  installer: "unmanaged"
  # The period used by the agent to sync with the Elemental API
  reconciliation: 1m
  # Allow 'http' scheme
  insecureAllowHttp: false
  # Skip TLS verification when communicating with the Elemental API
  insecureSkipTLSVerify: false
  # Use the system's cert pool for TLS verification
  useSystemCertPool: false
```
