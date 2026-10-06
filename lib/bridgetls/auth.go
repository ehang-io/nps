package bridgetls

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"net"
)

// ProofLen is the length of a proof.
const ProofLen = sha256.Size

const exporterLabel = "EXPORTER-nps-bridge-auth"

// Exporter is a value both ends of one TLS session derive and nothing else
// does: another session, a man in the middle's second leg included, derives a
// different one (RFC 5705, RFC 8446 section 7.5). c is the connection that
// Client or Server returned.
func Exporter(c net.Conn) ([]byte, error) {
	t, ok := c.(*tls.Conn)
	if !ok {
		return nil, errors.New("bridgetls: not a TLS connection")
	}
	cs := t.ConnectionState()
	return cs.ExportKeyingMaterial(exporterLabel, nil, 32)
}

// ClientProof is what a client sends to say that it holds vkey: an HMAC over
// this session's exporter and everything it said in its hello, so neither can
// be swapped for another. The server tries it against each key it has, which
// is why the hello carries no identifier of the key.
func ClientProof(vkey string, exporter, hello []byte) []byte {
	m := hmac.New(sha256.New, []byte(vkey))
	m.Write([]byte("nps bridge client\x00"))
	m.Write(exporter)
	m.Write(hello)
	return m.Sum(nil)
}

// ServerProof is the answer: the same, labelled for the other direction and
// covering the client's proof, so it proves that the one who holds vkey saw
// this very exchange, and cannot be replayed to another client.
func ServerProof(vkey string, exporter, clientProof []byte) []byte {
	m := hmac.New(sha256.New, []byte(vkey))
	m.Write([]byte("nps bridge server\x00"))
	m.Write(exporter)
	m.Write(clientProof)
	return m.Sum(nil)
}

// Equal compares two proofs without leaking where they differ.
func Equal(a, b []byte) bool { return hmac.Equal(a, b) }

// Hello is what a client says about itself, laid out the way the server reads
// it and the way both feed it to the proof: the protocol revision and the
// client's version, each with its length in front, then the four byte tag of
// what the connection is for.
func Hello(protocol, version, work string) []byte {
	var out []byte
	for _, v := range []string{protocol, version} {
		out = binary.LittleEndian.AppendUint32(out, uint32(len(v)))
		out = append(out, v...)
	}
	return append(out, work...)
}
