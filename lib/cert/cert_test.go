package cert

import (
	"crypto/tls"
	"crypto/x509/pkix"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestGetCertSerialNumber(t *testing.T) {
	g := NewX509Generator(pkix.Name{
		Country:            []string{"CN"},
		Organization:       []string{"Ehang.io"},
		OrganizationalUnit: []string{"nps"},
		Province:           []string{"Beijing"},
		CommonName:         "nps",
		Locality:           []string{"Beijing"},
	})
	cert, key, err := g.CreateRootCa()
	assert.NoError(t, err)
	assert.NoError(t, os.WriteFile(filepath.Join(os.TempDir(), "cert.pem"), cert, 0600))
	assert.NoError(t, os.WriteFile(filepath.Join(os.TempDir(), "key.pem"), key, 0600))
	assert.NoError(t, err)

	cliCrt, err := tls.LoadX509KeyPair(filepath.Join(os.TempDir(), "cert.pem"), filepath.Join(os.TempDir(), "key.pem"))
	assert.NoError(t, err)

	config := &tls.Config{
		Certificates: []tls.Certificate{cliCrt},
	}
	sn1, err := GetCertSnFromConfig(config)
	assert.NoError(t, err)
	assert.NotEmpty(t, sn1)

	sn2, err := GetCertSnFromEncode(cert)
	assert.NoError(t, err)
	assert.NotEmpty(t, sn2)

	assert.Equal(t, sn1, sn2)
}
