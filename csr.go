// Package eidas provides tools for generating eIDAS OBWAC & OBSEAL certificate
// signing requests.
package eidas

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/binary"
	"fmt"
	"log"

	"github.com/creditkudos/eidas/qcstatements"
)

type CertificateOption func(*x509.CertificateRequest)

// WithDNSName adds the given domain as a Subject Alternate Name to the CSR.
func WithDNSName(domain string) CertificateOption {
	return func(req *x509.CertificateRequest) {
		req.DNSNames = append(req.DNSNames, domain)
	}
}

// GenerateCSRWithKey builds a certificate signing request for an organization based on an existing private key.
// qcType should be one of qcstatements.QSEALType or qcstatements.QWACType.
func GenerateCSRWithKey(
	countryCode string, orgName string, orgID string, commonName string, roles []qcstatements.Role, qcType asn1.ObjectIdentifier, priv crypto.Signer, opts ...CertificateOption) ([]byte, error) {
	if _, ok := priv.Public().(*rsa.PublicKey); !ok {
		return nil, fmt.Errorf("only RSA keys are currently supported but got: %T", priv.Public())
	}
	ca, err := qcstatements.CompetentAuthorityForCountryCode(countryCode)
	if err != nil {
		return nil, fmt.Errorf("eidas: %v", err)
	}

	qc, err := qcstatements.Serialize(roles, *ca, qcType)
	if err != nil {
		return nil, fmt.Errorf("eidas: %v", err)
	}

	keyUsage, err := keyUsageForType(qcType)
	if err != nil {
		return nil, err
	}
	extendedKeyUsage, err := extendedKeyUsageForType(qcType)
	if err != nil {
		return nil, err
	}

	extensions := []pkix.Extension{
		keyUsageExtension(keyUsage),
	}
	if len(extendedKeyUsage) != 0 {
		extensions = append(extensions, extendedKeyUsageExtension(extendedKeyUsage))
	}
	extensions = append(extensions, subjectKeyIdentifier(priv.Public().(*rsa.PublicKey)), qcStatementsExtension(qc))

	subject, err := buildSubject(countryCode, orgName, commonName, orgID)
	if err != nil {
		return nil, fmt.Errorf("failed to build CSR subject: %v", err)
	}
	req := &x509.CertificateRequest{
		Version:            0,
		RawSubject:         subject,
		SignatureAlgorithm: x509.SHA256WithRSA,
		PublicKeyAlgorithm: x509.RSA,
		ExtraExtensions:    extensions,
	}
	for _, opt := range opts {
		opt(req)
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, req, priv)
	if err != nil {
		return nil, fmt.Errorf("failed to generate csr: %v", err)
	}
	return csr, err
}

// GenerateCSR generates an RSA key and builds a certificate signing request for an organization.
// qcType should be one of qcstatements.QSEALType or qcstatements.QWACType.
func GenerateCSR(
	countryCode string, orgName string, orgID string, commonName string, roles []qcstatements.Role, qcType asn1.ObjectIdentifier, opts ...CertificateOption) ([]byte, *rsa.PrivateKey, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate key pair: %v", err)
	}

	csr, err := GenerateCSRWithKey(countryCode, orgName, orgID, commonName, roles, qcType, key, opts...)
	if err != nil {
		return nil, nil, err
	}
	return csr, key, nil
}

func keyUsageForType(t asn1.ObjectIdentifier) ([]x509.KeyUsage, error) {
	if t.Equal(qcstatements.QWACType) {
		return []x509.KeyUsage{
			x509.KeyUsageDigitalSignature,
		}, nil
	} else if t.Equal(qcstatements.QSEALType) {
		return []x509.KeyUsage{
			x509.KeyUsageDigitalSignature,
			x509.KeyUsageContentCommitment, // Also known as NonRepudiation.
		}, nil
	}
	return nil, fmt.Errorf("unknown QC type: %v", t)
}

func keyUsageExtension(usages []x509.KeyUsage) pkix.Extension {
	x := uint16(0)
	for _, usage := range usages {
		x |= (uint16(1) << (8 - uint(usage)))
	}
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, x)
	bits := asn1.BitString{
		Bytes:     b,
		BitLength: int(x509.KeyUsageDecipherOnly),
	}
	d, _ := asn1.Marshal(bits)
	return pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 15},
		Critical: true,
		Value:    d,
	}
}

func extendedKeyUsageForType(t asn1.ObjectIdentifier) ([]asn1.ObjectIdentifier, error) {
	if t.Equal(qcstatements.QWACType) {
		return []asn1.ObjectIdentifier{
			tLSWWWServerAuthUsage,
			tLSWWWClientAuthUsage,
		}, nil
	} else if t.Equal(qcstatements.QSEALType) {
		return []asn1.ObjectIdentifier{}, nil
	}
	return nil, fmt.Errorf("unknown QC type: %v", t)
}

var (
	tLSWWWServerAuthUsage = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 1}
	tLSWWWClientAuthUsage = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 3, 2}
)

func extendedKeyUsageExtension(usages []asn1.ObjectIdentifier) pkix.Extension {
	d, _ := asn1.Marshal(usages)

	return pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 37},
		Critical: false,
		Value:    d,
	}
}

func subjectKeyIdentifier(key *rsa.PublicKey) pkix.Extension {
	b := sha1.Sum(x509.MarshalPKCS1PublicKey(key))
	d, err := asn1.Marshal(b[:])
	if err != nil {
		log.Fatalf("failed to marshal subject key identifier: %v", err)
	}

	return pkix.Extension{
		Id:       asn1.ObjectIdentifier{2, 5, 29, 14},
		Critical: false,
		Value:    d,
	}
}

// QCStatementsExt represents the qcstatements x509 extension id.
var QCStatementsExt = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 3}

func qcStatementsExtension(data []byte) pkix.Extension {
	return pkix.Extension{
		Id:       QCStatementsExt,
		Critical: false,
		Value:    data,
	}
}

var oidCountryCode = asn1.ObjectIdentifier{2, 5, 4, 6}
var oidOrganizationName = asn1.ObjectIdentifier{2, 5, 4, 10}
var oidOrganizationID = asn1.ObjectIdentifier{2, 5, 4, 97}
var oidCommonName = asn1.ObjectIdentifier{2, 5, 4, 3}

// Explicitly build subject from attributes to keep ordering.
func buildSubject(countryCode string, orgName string, commonName string, orgID string) ([]byte, error) {
	s := pkix.Name{
		ExtraNames: []pkix.AttributeTypeAndValue{
			{
				Type:  oidCountryCode,
				Value: countryCode,
			},
			{
				Type:  oidOrganizationName,
				Value: orgName,
			},
			{
				Type:  oidOrganizationID,
				Value: orgID,
			},
			{
				Type:  oidCommonName,
				Value: commonName,
			},
		},
	}
	return asn1.Marshal(s.ToRDNSequence())
}
