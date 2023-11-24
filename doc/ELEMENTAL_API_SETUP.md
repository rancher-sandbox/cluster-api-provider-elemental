# Elemental API Setup

This document describes the possibilities when exposing the Elemental API service.  

## Recommended configuration

```bash
ELEMENTAL_API_ENDPOINT="my.elemental.api.endpoint.com" \
clusterctl init --bootstrap "-" --control-plane "-" --infrastructure elemental:v0.3.0
```

The most reliable way to serve the Elemental API is through an Ingress controller, making use of a public [ACME Issuer](https://cert-manager.io/docs/configuration/acme/).  
Additionally it is recommended to keep the Elemental API under a private network, therefore using the [DNS01 challenge type](https://cert-manager.io/docs/configuration/acme/dns01/) to refresh certificates.  

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: elemental-api
  namespace: elemental-system
  annotations:
    cert-manager.io/issuer: "my-acme-issuer"
spec:
  tls:
  - hosts:
    - my.elemental.api.endpoint.com
    secretName: my-elemental-api-endpoint-com
  rules:
  - host: my.elemental.api.endpoint.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: elemental-controller-manager
            port:
              number: 9090
```

This allows to configure the `elemental-agent` to use the system's certificate pool, which can be managed and updated in a more convenient way, for example by simply installing the `ca-certificates-mozilla` package.  
This setting can be included when creating any `ElementalRegistration`:  

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ElementalRegistration
metadata:
  name: my-registration
  namespace: default
spec:
  config:
    elemental:
      agent:
        useSystemCertPool: true
```

## Default self-signed CA

By default this provider creates a self signed `cert-manager` CA Issuer.  

```bash
kubectl -n elemental-system get issuers -o wide
NAME                   READY   STATUS                AGE
elemental-ca           True    Signing CA verified   59m
elemental-selfsigned   True                          59m
```

The following certificates are also created and loaded to the Elemental controller by default:  

```bash
kubectl -n elemental-system get certificates -o wide
NAME                READY   SECRET              ISSUER                 STATUS                                          AGE
elemental-api-ca    True    elemental-api-ca    elemental-selfsigned   Certificate is up to date and has not expired   63m
elemental-api-ssl   True    elemental-api-ssl   elemental-ca           Certificate is up to date and has not expired   63m
```

The `elemental-api-ssl` certificate can be used out of the box when configuring the `ELEMENTAL_API_ENABLE_TLS="\"true\""` variable.  
The certificate's `dnsName` is configured with the `ELEMENTAL_API_ENDPOINT` variable. This variable must always be set when istalling the controller.  
It will not only be used to generate the default certificate, but it will also be used to automatically generate the `spec.config.elemental.registration.uri` field of every new `ElementalRegistration`.  
This will make the Elemental API use the certificate and listen to TLS connections. Note that this certificate has a default expiration of `1 year` and the controller needs to be manually restarted after certificate renewal.  

The `elemental-api-ca` certificate can also be included by default in any new `ElementalRegistration`.  
This allows for a quick and convenient way to make the `elemental-agent` trust the self-signed certificate.  
This behavior can be enabled when using the `ELEMENTAL_ENABLE_DEFAULT_CA="\"true\""` variable.  
By doing so, the controller will initialize the `ElementalRegistration` `spec.config.elemental.registration.caCert` field with the CA cert defined by the `ELEMENTAL_API_TLS_CA`.  
By default this is configured to be the `/etc/elemental/ssl/ca.crt` mounted from `elemental-api-ssl` certificate's secret.  

For example, to enable the TLS listener and use the default CA:  

```bash
ELEMENTAL_API_ENDPOINT="my.elemental.api.endpoint.com" \
ELEMENTAL_API_ENABLE_TLS="\"true\"" \
ELEMENTAL_ENABLE_DEFAULT_CA="\"true\"" \
clusterctl init --bootstrap "-" --control-plane "-" --infrastructure elemental:v0.3.0
```

Now when creating a new `ElementalRegistration` you should see the `caCert` field being populated by default:

```bash
cat << EOF | kubectl apply -f -
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ElementalRegistration
metadata:
  name: my-registration
  namespace: default
spec: {}
EOF
```

```bash
kubectl get elementalregistration my-registration -o yaml 
```

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ElementalRegistration
metadata:
  name: my-registration
  namespace: default
spec:
  config:
    elemental:
      registration:
        caCert: |
          -----BEGIN CERTIFICATE-----
          MIIDEDCCAfigAwIBAgIQB6v+n9ClHeesS7NRRRgN1TANBgkqhkiG9w0BAQsFADAi
          MSAwHgYDVQQDExdlbGVtZW50YWwtc2VsZnNpZ25lZC1jYTAeFw0yMzExMjMxNDA1
          MjNaFw0zNDA5MTYxNDA1MjNaMCIxIDAeBgNVBAMTF2VsZW1lbnRhbC1zZWxmc2ln
          bmVkLWNhMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAtg6TCCdtHlKu
          IHyYp24aZZxJ/iuNjFzxVgDaaukr+13Po0Iz6oVFRmxBzz3H74jwCAq7j6aw42id
          u52ZWH5A8eHlo5W8hvuEhb1B/F52wpXA0UTi8pil4AEd2rO7QQQi+UkHuZy4k69W
          IEzTE9OQPLiLPHaxgRD0DP8X7ick0JYs/VQrEtsiZy9K7dhtN0UTBsHFUWUJWYKU
          jI5Mj3Ah7SFH1ry8BdLPtiUxFggxUeBq3C7m3r6s1vvXvPvDU1Vr7R0iyKGDAEcI
          08dkZnbYr8LHyUXXuWoKxgg96oB9sdV5A80eXIlhGIFTTIBBzclqMr0B6xHmMkrA
          CRw05ufB3wIDAQABo0IwQDAOBgNVHQ8BAf8EBAMCAqQwDwYDVR0TAQH/BAUwAwEB
          /zAdBgNVHQ4EFgQUClau+YzBMKTmt9Yr1bcnRoYTHYEwDQYJKoZIhvcNAQELBQAD
          ggEBAI4nXRUswqBWqVVVpAt4EkHRbsS2UnUpZnBhpnD2k9wbLvzupH5xBl5cdRD6
          F4aubIorWLEmMfPHwvkruEOQFujJD7ZVgUh5sHfFsn73t1nAzRnQBmtb7vMt/DPt
          ZxDUMKNaJXmbB+mC+85h6MfOxAWqVPdgSj0WYBRaWRWRKcMxW/hqJxQ775e0bxau
          +YHQKpDj+TLE38ZEMkpCRgAj1UOV2CauRc0c3b0tu5qNYAagN2IKGAt8vWVx/RnN
          wp7wGl9ayPIwLh8iqaDP/rsYYiSb9QbNE7D9hDw0l6ZvRsNgg4QLkiYgbdfc4yH/
          66ltSv8CdT37o7DtKaJqaqecYK0=
          -----END CERTIFICATE-----
        uri: https://my.elemental.api.endpoint.com/elemental/v1/namespaces/default/registrations/my-registration
```

## Using Ingress

Ingress can better take care of certificates rotation and integration with `cert-manager`.  
When using a TLS termination proxy, you can configure this provider with the `ELEMENTAL_API_ENABLE_TLS="\"false\""` variable, which is also the default value.  
If using the default self-signed CA, you can still configure `ELEMENTAL_ENABLE_DEFAULT_CA="\"true\""` and use the already generated `elemental-api-ssl` certificate to configure the Ingress `tls` settings.
For example:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: elemental-api
  namespace: elemental-system
spec:
  tls:
  - hosts:
      - my.elemental.api.endpoint.com
    secretName: elemental-api-ssl
  rules:
  - host: my.elemental.api.endpoint.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: elemental-controller-manager
            port:
              number: 9090
```

When using a certificate signed by a different CA, you have different options.  
One option is to explicitly define the CA certificate to trust in each `ElementalRegistration`.  
For example:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ElementalRegistration
metadata:
  name: my-registration
  namespace: default
spec:
  config:
    elemental:
      registration:
        caCert: |
          -----BEGIN CERTIFICATE-----
               MY SELF-SIGNED CA
          -----END CERTIFICATE-----
```

Another option is to configure the `elemental-agent` to use the system's certificate pool.  

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ElementalRegistration
metadata:
  name: my-registration
  namespace: default
spec:
  config:
    elemental:
      agent:
        useSystemCertPool: true
```

## Using different Load Balancers

The `ElementalRegistration` `spec.config.elemental.registration.uri` is normally populated automatically by the provider, from the `ELEMENTAL_API_PROTOCOL` and `ELEMENTAL_API_ENDPOINT` environment variables.  
Howeverm it can also be set arbitrarily, for example to route different registrations to different load balancers.  
This must be the fully qualified URI of the registration, including the registration name and namespace.  
For example:  

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ElementalRegistration
metadata:
  name: my-registration
  namespace: default
spec:
  config:
    elemental:
      registration:
        uri: https://my.elemental.api.endpoint.com/elemental/v1/namespaces/default/registrations/my-registration
---
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ElementalRegistration
metadata:
  name: my-alternative-registration
  namespace: default
spec:
  config:
    elemental:
      registration:
        uri: https://my.alternative.api.endpoint.com/elemental/v1/namespaces/default/registrations/my-alternative-registration
```

Note that this mechanism can also be exploited to connect to non standard ports.  
For example when exposing the Elemental API on a nodeport (for ex. `30009`), the uri can be configured to include the port:

```yaml
apiVersion: infrastructure.cluster.x-k8s.io/v1beta1
kind: ElementalRegistration
metadata:
  name: my-registration
  namespace: default
spec:
  config:
    elemental:
      registration:
        uri: https://my.elemental.api.endpoint.com:30009/elemental/v1/namespaces/default/registrations/my-registration
```  
