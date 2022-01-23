package cert

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"github.com/pkg/errors"
)

// GetCertSnFromConfig return SerialNumber by tls.Config
func GetCertSnFromConfig(config *tls.Config) (string, error) {
	if len(config.Certificates) == 0 || len(config.Certificates[0].Certificate) == 0 {
		return "", errors.New("certificates is empty")
	}
	return GetCertSnFromBlock(config.Certificates[0].Certificate[0])
}

// GetCertSnFromEncode return SerialNumber by encoded cert
func GetCertSnFromEncode(b []byte) (string, error) {
	block, _ := pem.Decode(b)
	if block == nil {
		return "", errors.New("block is not a cert encoded")
	}
	return GetCertSnFromBlock(block.Bytes)
}

// GetCertSnFromBlock return SerialNumber by decode block
func GetCertSnFromBlock(block []byte) (string, error) {
	cert, err := x509.ParseCertificate(block)
	if err != nil {
		return "", errors.Wrap(err, "ParseCertificate")
	}
	return cert.SerialNumber.String(), nil
}
