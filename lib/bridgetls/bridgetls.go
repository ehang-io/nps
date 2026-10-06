// Package bridgetls is the transport security of the bridge connection: every
// connection a client opens to the server, whatever it is for, is TLS 1.3 from
// its first byte.
//
// TLS here only encrypts. The server's certificate is made up at start and the
// client does not check it, so TLS alone would let anyone on the path answer
// for the server. What proves that the other end is the right one is the vkey,
// which both sides hold and which never crosses the wire: after the handshake
// each side sends an HMAC of this session's exported keying material, see
// ClientProof and ServerProof. A man in the middle runs two TLS sessions, one
// to each end, and the two have different exported values, so a proof made on
// one is worthless on the other, and without the vkey he cannot make his own.
//
// What this does not do is hide the vkey from a man in the middle who simply
// answers as the server: the client's proof is the first thing he sees, it is
// an HMAC of the vkey under a value he knows, and so a weak vkey can be
// guessed from it offline. The vkey must be random; a password-authenticated
// key exchange would remove the limit, at the price of a much larger protocol.
package bridgetls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"net"
	"time"
)

// HandshakeTimeout bounds the TLS handshake on both ends, so a peer that
// connects and says nothing does not hold a goroutine and a socket.
const HandshakeTimeout = 10 * time.Second

// ServerConfig makes the server's TLS configuration around a key and a
// certificate that exist only until it exits. Nothing pins them, so there is
// no file to keep, back up or lose, and a restart changes nothing for clients.
func ServerConfig() (*tls.Config, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}
	name := make([]byte, 8)
	if _, err := rand.Read(name); err != nil {
		return nil, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		// no fixed name for a probe to recognise
		Subject:     pkix.Name{CommonName: hex.EncodeToString(name)},
		NotBefore:   time.Now().Add(-time.Hour),
		NotAfter:    time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}},
		MinVersion:   tls.VersionTLS13,
	}, nil
}

// ClientConfig is the client's TLS configuration. It accepts any certificate
// on purpose: the server is identified by the proof that follows the
// handshake, not by anything in the certificate.
func ClientConfig() *tls.Config {
	return &tls.Config{
		MinVersion:         tls.VersionTLS13,
		InsecureSkipVerify: true,
	}
}

// Client runs the client side of the handshake on c.
func Client(c net.Conn, cfg *tls.Config) (*tls.Conn, error) {
	return handshake(tls.Client(c, cfg), c)
}

// Server runs the server side of the handshake on c.
func Server(c net.Conn, cfg *tls.Config) (*tls.Conn, error) {
	return handshake(tls.Server(c, cfg), c)
}

func handshake(t *tls.Conn, c net.Conn) (*tls.Conn, error) {
	_ = c.SetDeadline(time.Now().Add(HandshakeTimeout))
	err := t.Handshake()
	_ = c.SetDeadline(time.Time{})
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	return t, nil
}

// TCPConn digs the TCP socket out from under c and any TLS layers over it, for
// the options that only the socket has, keepalive among them.
func TCPConn(c net.Conn) (*net.TCPConn, bool) {
	for {
		switch v := c.(type) {
		case *net.TCPConn:
			return v, true
		case interface{ NetConn() net.Conn }:
			c = v.NetConn()
		default:
			return nil, false
		}
	}
}
