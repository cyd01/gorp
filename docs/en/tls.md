# TLS and mTLS

GORP uses TLS in two places:

- **Downstream**: client $\to$ GORP;
- **Upstream**: GORP $\to$ backend.

## Classic Downstream TLS

```yaml
listeners:
  - name: https
    type: https
    address: ":443"
    tls:
      cert_file: "/etc/gorp/tls/server.crt"
      key_file: "/etc/gorp/tls/server.key"
      key_passphrase: "change-me"
```

Set `key_passphrase` when the private key uses encrypted legacy PEM encoding.
This applies to downstream TLS listener certificates, including TCP SNI route
certificates. For `dynamic`, the same setting decrypts the signing CA key.

Private keys must be PEM encoded. Unencrypted PKCS#8 (`PRIVATE KEY`), RSA
PKCS#1 (`RSA PRIVATE KEY`), and EC SEC1 (`EC PRIVATE KEY`) keys are supported.
Encrypted legacy PEM keys are supported with `key_passphrase`; encrypted PKCS#8
(`ENCRYPTED PRIVATE KEY`) is not supported.

## Minimum TLS Version

```yaml
tls:
  cert_file: server.crt
  key_file: server.key
  min_version: "TLS1.2"
```

The forms `TLS1.0`, `TLS1.1`, `TLS1.2`, `TLS1.3` are accepted, as well as the corresponding `TLSV1.x` and `1.x` forms.

## Downstream mTLS

To require a client certificate signed by a given CA:

```yaml
listeners:
  - name: api
    type: https
    address: ":8443"
    tls:
      cert_file: server.crt
      key_file: server.key
      ca_file: clients-ca.crt
```

The client must present a valid certificate signed by this CA.

GORP then adds the verified client certificate to `X-Forwarded-Client-Cert` for HTTPS, HTTP/3, and dynamic HTTPS listeners. Any value provided by the client in this header is replaced.

## CRL Revocation

```yaml
tls:
  cert_file: server.crt
  key_file: server.key
  ca_file: clients-ca.crt
  crl_file: clients.crl
```

If a CRL is provided, it is used to verify the client certificate's serial number.

## OCSP Revocation

```yaml
tls:
  cert_file: server.crt
  key_file: server.key
  ca_file: clients-ca.crt
  ocsp_url: "https://ocsp.example.com"
```

When `crl_file` is not defined, the code can use `ocsp_url`. If the URL is absent, the server can use the OCSP URL present in the client certificate.

**CRL and OCSP are mutually exclusive in the current implementation: if a CRL is configured, it takes priority.**

## Upstream TLS with Private CA

```yaml
routes:
  - name: internal-api
    prefix: "/api"
    backends:
      - name: api
        url: "https://api.internal:8443"
        tls:
          ca_file: "/etc/gorp/ca/internal-ca.crt"
          server_name: "api.internal"
```

## Upstream TLS with Client Certificate

```yaml
- name: mtls-api
  url: "https://api.internal:8443"
  tls:
    ca_file: "/etc/gorp/ca/internal-ca.crt"
    cert_file: "/etc/gorp/tls/gorp-client.crt"
    key_file: "/etc/gorp/tls/gorp-client.key"
    key_passphrase: "change-me"
```

The upstream `key_passphrase` unlocks a password-protected client private key.
