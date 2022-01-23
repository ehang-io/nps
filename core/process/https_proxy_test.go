package process

import (
	"crypto/tls"
	"crypto/x509/pkix"
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/cert"
	"ehang.io/nps/lib/enet"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
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
		assert.NoError(t, os.WriteFile(filepath.Join(os.TempDir(), "cert.pem"), cert, 0600))
		assert.NoError(t, os.WriteFile(filepath.Join(os.TempDir(), "key.pem"), key, 0600))
	})
	return filepath.Join(os.TempDir(), "cert.pem"), filepath.Join(os.TempDir(), "key.pem")
}

func TestHttpsProxyProcess(t *testing.T) {
	sAddr, err := startHttps(t)
	certFilePath, keyFilePath := createCertFile(t)
	h := HttpsProxyProcess{
		HttpProxyProcess: HttpProxyProcess{},
		CertFile:         certFilePath,
		KeyFile:          keyFilePath,
	}
	ac := &action.LocalAction{}
	ac.Init()
	assert.NoError(t, h.Init(ac))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	go func() {
		for {
			c, err := ln.Accept()
			assert.NoError(t, err)
			go h.ProcessConn(enet.NewReaderConn(c))
		}
	}()

	transport := &http.Transport{
		Proxy: func(_ *http.Request) (*url.URL, error) {
			return url.Parse(fmt.Sprintf("https://%s", ln.Addr().String()))
		},
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Get(fmt.Sprintf("https://%s/now", sAddr))
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}

func TestHttpsProxyProcessBasic(t *testing.T) {
	certFilePath, keyFilePath := createCertFile(t)
	sAddr, err := startHttps(t)
	h := HttpsProxyProcess{
		HttpProxyProcess: HttpProxyProcess{
			BasicAuth: map[string]string{"aaa": "bbb"},
		},
		CertFile: certFilePath,
		KeyFile:  keyFilePath,
	}
	ac := &action.LocalAction{}
	ac.Init()
	assert.NoError(t, h.Init(ac))

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	go func() {
		for {
			c, err := ln.Accept()
			assert.NoError(t, err)
			go h.ProcessConn(enet.NewReaderConn(c))
		}
	}()

	transport := &http.Transport{
		Proxy: func(_ *http.Request) (*url.URL, error) {
			return url.Parse(fmt.Sprintf("https://%s", ln.Addr().String()))
		},
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,
		},
	}

	client := &http.Client{Transport: transport}
	resp, err := client.Get(fmt.Sprintf("https://%s/now", sAddr))
	assert.Error(t, err)
	transport.Proxy = func(_ *http.Request) (*url.URL, error) {
		return url.Parse(fmt.Sprintf("https://%s:%s@%s", "aaa", "bbb", ln.Addr().String()))
	}

	resp, err = client.Get(fmt.Sprintf("https://%s/now", sAddr))
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
}
