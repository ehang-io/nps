package component

import (
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"ehang.io/nps/component/bridge"
	"ehang.io/nps/component/client"
	"ehang.io/nps/lib/cert"
	"ehang.io/nps/lib/pb"
	"github.com/lucas-clemente/quic-go"
	uuid "github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

var createCertOnce sync.Once

func createCertFile(t *testing.T) (string, string) {
	createCertOnce.Do(func() {
		g := cert.NewX509Generator(pkix.Name{
			Country:            []string{"CN"},
			Organization:       []string{"Ehang.io"},
			OrganizationalUnit: []string{"nps"},
			Province:           []string{"Beijing"},
			CommonName:         "nps",
			Locality:           []string{"Beijing"},
		})
		cert, key, err := g.CreateRootCa()
		assert.NoError(t, err)
		assert.NoError(t, os.WriteFile(filepath.Join(os.TempDir(), "root_cert.pem"), cert, 0600))
		assert.NoError(t, os.WriteFile(filepath.Join(os.TempDir(), "root_key.pem"), key, 0600))
		assert.NoError(t, g.InitRootCa(cert, key))
		cert, key, err = g.CreateCert("bridge.nps.ehang.io")
		assert.NoError(t, err)
		assert.NoError(t, os.WriteFile(filepath.Join(os.TempDir(), "bridge_cert.pem"), cert, 0600))
		assert.NoError(t, os.WriteFile(filepath.Join(os.TempDir(), "bridge_key.pem"), key, 0600))
		cert, key, err = g.CreateCert("client.nps.ehang.io")
		assert.NoError(t, err)
		assert.NoError(t, os.WriteFile(filepath.Join(os.TempDir(), "client_cert.pem"), cert, 0600))
		assert.NoError(t, os.WriteFile(filepath.Join(os.TempDir(), "client_key.pem"), key, 0600))
	})
	return filepath.Join(os.TempDir(), "cert.pem"), filepath.Join(os.TempDir(), "key.pem")
}

func TestTcpConnect(t *testing.T) {
	createCertFile(t)
	bridgeLn, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)

	buf, err := ioutil.ReadFile(filepath.Join(os.TempDir(), "root_cert.pem"))
	assert.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(buf)

	crt, err := tls.LoadX509KeyPair(filepath.Join(os.TempDir(), "bridge_cert.pem"), filepath.Join(os.TempDir(), "bridge_key.pem"))
	assert.NoError(t, err)

	bridgeTlsConfig := &tls.Config{
		Certificates: []tls.Certificate{crt},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
	}

	crt, err = tls.LoadX509KeyPair(filepath.Join(os.TempDir(), "client_cert.pem"), filepath.Join(os.TempDir(), "client_key.pem"))
	assert.NoError(t, err)

	clientConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{crt},
	}
	go func() {
		assert.NoError(t, bridge.StartTcpBridge(bridgeLn, bridgeTlsConfig, func(s string) bool {
			return true
		}, func(s string) bool {
			sn, err := cert.GetCertSnFromConfig(clientConfig)
			assert.NoError(t, err)
			assert.Equal(t, sn, s)
			return true
		}))
	}()
	var c *client.Client
	go func() {
		id, err := cert.GetCertSnFromConfig(clientConfig)
		assert.NoError(t, err)

		creator := client.TcpTunnelCreator{}
		connId := uuid.NewV1().String()
		controlLn, err := creator.NewMux(bridgeLn.Addr().String(),
			&pb.ConnRequest{Id: id, ConnType: &pb.ConnRequest_NpcInfo{NpcInfo: &pb.NpcInfo{TunnelId: connId, IsControlTunnel: true}}}, clientConfig)
		assert.NoError(t, err)
		dataLn, err := creator.NewMux(bridgeLn.Addr().String(),
			&pb.ConnRequest{Id: id, ConnType: &pb.ConnRequest_NpcInfo{NpcInfo: &pb.NpcInfo{TunnelId: connId, IsControlTunnel: false}}}, clientConfig)
		assert.NoError(t, err)
		c = client.NewClient(controlLn, dataLn)
		c.Run()
	}()
	timeout := time.NewTimer(time.Second * 30)
	ticker := time.NewTicker(time.Millisecond * 100)
	for {
		select {
		case <-ticker.C:
			if c != nil && c.HasPong() {
				c.Close()
				return
			}
		case <-timeout.C:
			t.Fail()
			return
		}
	}
}

func TestQUICConnect(t *testing.T) {
	createCertFile(t)
	bridgePacketConn, err := net.ListenPacket("udp", "127.0.0.1:0")
	assert.NoError(t, err)

	buf, err := ioutil.ReadFile(filepath.Join(os.TempDir(), "root_cert.pem"))
	assert.NoError(t, err)
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(buf)

	crt, err := tls.LoadX509KeyPair(filepath.Join(os.TempDir(), "bridge_cert.pem"), filepath.Join(os.TempDir(), "bridge_key.pem"))
	assert.NoError(t, err)

	bridgeTlsConfig := &tls.Config{
		Certificates: []tls.Certificate{crt},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    pool,
		NextProtos:   []string{"quic-nps"},
	}

	crt, err = tls.LoadX509KeyPair(filepath.Join(os.TempDir(), "client_cert.pem"), filepath.Join(os.TempDir(), "client_key.pem"))
	assert.NoError(t, err)

	clientConfig := &tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{crt},
		NextProtos:         []string{"quic-nps"},
	}
	go func() {
		assert.NoError(t, bridge.StartQUICBridge(bridgePacketConn, bridgeTlsConfig, &quic.Config{
			MaxIncomingStreams:    1000000,
			MaxIncomingUniStreams: 1000000,
			MaxIdleTimeout:        time.Minute,
			KeepAlive:             true,
		}, func(s string) bool {
			sn, err := cert.GetCertSnFromConfig(clientConfig)
			assert.NoError(t, err)
			assert.Equal(t, sn, s)
			return true
		}))
	}()
	var c *client.Client
	go func() {
		id, err := cert.GetCertSnFromConfig(clientConfig)
		assert.NoError(t, err)

		creator := client.QUICTunnelCreator{}
		connId := uuid.NewV1().String()
		controlLn, err := creator.NewMux(bridgePacketConn.LocalAddr().String(),
			&pb.ConnRequest{Id: id, ConnType: &pb.ConnRequest_NpcInfo{NpcInfo: &pb.NpcInfo{TunnelId: connId, IsControlTunnel: true}}}, clientConfig)
		assert.NoError(t, err)
		dataLn, err := creator.NewMux(bridgePacketConn.LocalAddr().String(),
			&pb.ConnRequest{Id: id, ConnType: &pb.ConnRequest_NpcInfo{NpcInfo: &pb.NpcInfo{TunnelId: connId, IsControlTunnel: false}}}, clientConfig)
		assert.NoError(t, err)
		assert.NotEmpty(t, dataLn)
		assert.NotEmpty(t, controlLn)
		c = client.NewClient(controlLn, dataLn)
		c.Run()
	}()
	timeout := time.NewTimer(time.Second * 30)
	ticker := time.NewTicker(time.Millisecond * 100)
	for {
		select {
		case <-ticker.C:
			if c != nil && c.HasPong() {
				c.Close()
				return
			}
		case <-timeout.C:
			t.Fail()
			return
		}
	}
}
