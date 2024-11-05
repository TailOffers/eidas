[![Go Reference](https://pkg.go.dev/badge/github.com/apple/eidas.svg)](https://pkg.go.dev/github.com/apple/eidas)
[![Go Report Card](https://goreportcard.com/badge/github.com/apple/eidas)](https://goreportcard.com/report/github.com/apple/eidas)

# eIDAS
Tools for reading and creating eIDAS certificate signing requests

## Generating a Certificate Signing Request (CSR)

### With go (requires go 1.22 or higher):
```bash
go get github.com/apple/eidas/cmd/cli
```

```bash
go run github.com/apple/eidas/cmd/cli \
  -country-code GB \
  -organization-name "Your Organization Limited" \
  -organization-id PSDGB-FCA-123456 \
  -common-name 0123456789abcdef
```

### Open Banking Flags
* `-common-name` should be the same as the `organisation_id` field from your entry in the Open Banking Directory.
* `-organization-id` should be in the form of `PSD<Regulator Country Code>-<Regulator>-<Unique ID>`
* `-organization-name` should be your official company name.
* `-country-code` should be an ISO 3166-1 alpha-2 country code.

### Other flags
You can see the available flags with
```
go run github.com/apple/eidas/cmd/cli -help
```

By default this will generate two files: `out.csr` and `out.key` containing the CSR and the private key, respectively.

It will also print the SHA256 sum of the CSR to stdout.

To print out the details of the CSR for debugging, run:
```
openssl req -in out.csr -text -noout -nameopt multiline
```

## Notes on CSR format

For both QWAC and QSEAL types the following attributes are required in the CSR:

### [Subject](https://tools.ietf.org/html/rfc5280#section-4.1.2.6)
* Must contain country code, organisation name and common name.
* Must also contain the organisation ID. Organisation ID (ITU-T X.520 10/2012 Section 6.4.4) isn't supported by most tools by default (including OpenSSL and go) but this can be added to the subject as a custom name with the ASN.1 OID of `2.5.4.97`. Should be something like `PSDGB-FCA-123456`.
* It's not specified in the standards (AFAICT) but these should be in a defined order:
  1. Country Code (C=)
  1. Organization Name (O=)
  1. Organization ID (2.5.4.97=)
  1. Common Name (CN=)

### Key Parameters
* Key should be 2048-bit RSA.
* Signature algorithm should be `SHA256WithRSA`.

### Extensions

#### [Key Usage](https://tools.ietf.org/html/rfc5280#section-4.2.1.3)
* X509v3 Key Usage extension should be marked as `critical`.

| QWAC | QSEAL |
| --- | --- |
| Digital Signature | Digital Signature |
| Non Repudiation | |

#### [Extended Key Usage](https://tools.ietf.org/html/rfc5280#section-4.2.1.12)

| QWAC | QSEAL |
| --- | --- |
| TLS Web Server Authentication | |
| TLS Web Client Authentication | |

Note: For QSEAL, a CSR is expected to not have an extended key usage section at all, rather than an empty one.

#### [Subject Key Identifier](https://tools.ietf.org/html/rfc5280#section-4.2.1.2)
* Should be the 160-bit SHA1 sum of the PKCS1 public key.

#### [qcStatements](https://tools.ietf.org/html/rfc3739.html#section-3.2.6)
This is an extension used by eIDAS as documented here [ETSI TS 119 495 Annex A](https://www.etsi.org/deliver/etsi_ts/119400_119499/119495/01.02.01_60/ts_119495v010201p.pdf).
The required parameters included in this are the Competent Authority's name and ID, e.g. "Financial Conduct Authority" and "GB-FCA", and the roles the TPP requires, e.g. "PSP_AI" (Account Information).
