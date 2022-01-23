package cert

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"testing"
)

func TestCreateCert(t *testing.T) {
	dnsName := "ehang.io"
	g := NewX509Generator(pkix.Name{
		Country:            []string{"CN"},
		Organization:       []string{"ehang.io"},
		OrganizationalUnit: []string{"nps"},
		Province:           []string{"Beijing"},
		CommonName:         "nps",
		Locality:           []string{"Beijing"},
	})
	// generate root ca
	rootCa, rootKey, err := g.CreateRootCa()
	if err != nil {
		t.Fatal(err)
	}
	err = g.InitRootCa(rootCa, rootKey)
	if err != nil {
		t.Fatal(err)
	}

	// generate npc cert
	clientCa, _, err := g.CreateCert(dnsName)
	if err != nil {
		t.Fatal(err)
	}

	// verify npc cert by root cert
	roots := x509.NewCertPool()
	ok := roots.AppendCertsFromPEM(rootCa)
	if !ok {
		panic("failed to parse root certificate")
	}

	block, _ := pem.Decode(clientCa)
	if block == nil {
		t.Fatal("failed to parse certificate PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		t.Fatal("failed to parse certificate: " + err.Error())
	}

	opts := x509.VerifyOptions{
		Roots:         roots,
		DNSName:       dnsName,
		Intermediates: x509.NewCertPool(),
	}

	if _, err := cert.Verify(opts); err != nil {
		t.Fatal("failed to verify certificate: " + err.Error())
	}

}
