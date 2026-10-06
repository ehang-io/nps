package nps_mux

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"testing"
	"time"
)

// A mux over TLS must still reach the TCP socket under it, or the socket
// options and the bandwidth estimate that the queueing work depends on are
// silently lost.
func TestUnderlyingReachesTheSocketThroughTLS(t *testing.T) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "t"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour)}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go func() {
		c, err := l.Accept()
		if err == nil {
			_ = tls.Server(c, &tls.Config{Certificates: []tls.Certificate{cert}}).Handshake()
		}
	}()
	raw, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	c := tls.Client(raw, &tls.Config{InsecureSkipVerify: true})
	if err := c.Handshake(); err != nil {
		t.Fatal(err)
	}
	if _, ok := underlying(c).(*net.TCPConn); !ok {
		t.Fatalf("underlying(tls conn) is %T, want *net.TCPConn", underlying(c))
	}
	if r, err := getRawConn(c); err != nil || r == nil {
		t.Fatalf("no raw conn through TLS: %v %v", r, err)
	}
}
