package cert

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"time"
)

var _ Generator = (*X509Generator)(nil)

type Generator interface {
	CreateRootCa() ([]byte, []byte, error)
	CreateCert(dnsName string) ([]byte, []byte, error)
	InitRootCa(rootCa []byte, rootKey []byte) error
}

type X509Generator struct {
	rootCert       *x509.Certificate
	rootRsaPrivate *rsa.PrivateKey
	subject        pkix.Name
}

func NewX509Generator(subject pkix.Name) *X509Generator {
	return &X509Generator{
		subject: subject,
	}
}

func (cg *X509Generator) InitRootCa(rootCa []byte, rootKey []byte) error {
	var err error
	caBlock, _ := pem.Decode(rootCa)
	cg.rootCert, err = x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return err
	}

	keyBlock, _ := pem.Decode(rootKey)
	cg.rootRsaPrivate, err = x509.ParsePKCS1PrivateKey(keyBlock.Bytes)
	if err != nil {
		return err
	}
	return nil
}

func (cg *X509Generator) CreateCert(dnsName string) ([]byte, []byte, error) {
	return cg.create(false, dnsName)
}

func (cg *X509Generator) CreateRootCa() ([]byte, []byte, error) {
	return cg.create(true, "")
}

func (cg *X509Generator) create(isRootCa bool, dnsName string) ([]byte, []byte, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	template := &x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               cg.subject,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(3, 0, 0),
		BasicConstraintsValid: true,
		IsCA:                  false,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageDataEncipherment,
		DNSNames: []string{dnsName},
	}

	if isRootCa {
		template.IsCA = true
		template.KeyUsage |= x509.KeyUsageCertSign
	}

	priKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}
	var ca []byte
	if !isRootCa {
		if cg.rootCert == nil || cg.rootRsaPrivate == nil {
			return nil, nil, errors.New("root ca is not exist")
		}
		ca, err = x509.CreateCertificate(rand.Reader, template, cg.rootCert, &priKey.PublicKey, cg.rootRsaPrivate)
	} else {
		ca, err = x509.CreateCertificate(rand.Reader, template, template, &priKey.PublicKey, priKey)
	}
	if err != nil {
		return nil, nil, err
	}

	caPem := &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: ca,
	}
	ca = pem.EncodeToMemory(caPem)

	buf := x509.MarshalPKCS1PrivateKey(priKey)
	keyPem := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: buf,
	}
	key := pem.EncodeToMemory(keyPem)
	return ca, key, nil
}
