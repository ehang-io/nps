package crypt

import (
	crand "crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"log"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/astaxie/beego/logs"
)

var (
	certMu   sync.RWMutex
	cert     tls.Certificate
	initOnce sync.Once
	initErr  error

	// 证书文件缓存：key 为 cert+key 路径，文件修改时间变化时重新加载
	certCache sync.Map
)

type certEntry struct {
	cert    tls.Certificate
	certMod time.Time
	keyMod  time.Time
}

// InitTls 初始化内建的自签证书。多次调用只会生成一次（RSA 生成较慢）。
func InitTls() {
	initOnce.Do(func() {
		c, k, err := generateKeyPair("NPS Org")
		if err != nil {
			initErr = err
			return
		}
		crt, err := tls.X509KeyPair(c, k)
		if err != nil {
			initErr = err
			return
		}
		certMu.Lock()
		cert = crt
		certMu.Unlock()
	})
	if initErr != nil {
		log.Fatalln("Error initializing crypto certs", initErr)
	}
}

// BuiltinCert 返回内建自签证书（会自动初始化）
func BuiltinCert() tls.Certificate {
	InitTls()
	certMu.RLock()
	defer certMu.RUnlock()
	return cert
}

// LoadCertFile 从文件加载证书对，按文件修改时间缓存，便于证书热替换。
func LoadCertFile(certFile, keyFile string) (tls.Certificate, error) {
	if certFile == "" || keyFile == "" {
		return tls.Certificate{}, errors.New("cert file and key file are both required")
	}
	certInfo, err := os.Stat(certFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	keyInfo, err := os.Stat(keyFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	key := filepath.Clean(certFile) + "|" + filepath.Clean(keyFile)
	if v, ok := certCache.Load(key); ok {
		if e, ok := v.(*certEntry); ok && e.certMod.Equal(certInfo.ModTime()) && e.keyMod.Equal(keyInfo.ModTime()) {
			return e.cert, nil
		}
	}
	crt, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return tls.Certificate{}, err
	}
	certCache.Store(key, &certEntry{cert: crt, certMod: certInfo.ModTime(), keyMod: keyInfo.ModTime()})
	return crt, nil
}

// ServerTlsConfig 生成服务端 TLS 配置。
// certFile/keyFile 为空时使用内建自签证书，否则读取指定证书（不可用时回退到自签并记录告警）。
func ServerTlsConfig(certFile, keyFile string) (*tls.Config, error) {
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	if strings.TrimSpace(certFile) == "" && strings.TrimSpace(keyFile) == "" {
		cfg.Certificates = []tls.Certificate{BuiltinCert()}
		return cfg, nil
	}
	crt, err := LoadCertFile(certFile, keyFile)
	if err != nil {
		logs.Warn("load tls certificate %s/%s error: %v, falling back to the built-in self-signed certificate", certFile, keyFile, err)
		cfg.Certificates = []tls.Certificate{BuiltinCert()}
		return cfg, nil
	}
	cfg.Certificates = []tls.Certificate{crt}
	return cfg, nil
}

// WrapServerTls 用 TLS 包装一条已建立的连接。握手在首次读写时进行，
// 调用方应自行设置握手超时。
func WrapServerTls(c net.Conn, certFile, keyFile string) (net.Conn, error) {
	cfg, err := ServerTlsConfig(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return tls.Server(c, cfg), nil
}

// ClientTlsConfig 生成客户端 TLS 配置。
// verify 为 true 时校验服务端证书；为 false 时接受自签证书（nps 默认的自签部署方式）。
func ClientTlsConfig(serverAddr string, verify bool) *tls.Config {
	host := serverAddr
	if h, _, err := net.SplitHostPort(serverAddr); err == nil {
		host = h
	}
	return &tls.Config{
		MinVersion:         tls.VersionTLS12,
		ServerName:         host,
		InsecureSkipVerify: !verify,
	}
}

// WrapClientTls 用 TLS 包装一条已建立的连接
func WrapClientTls(c net.Conn, serverAddr string, verify bool) net.Conn {
	return tls.Client(c, ClientTlsConfig(serverAddr, verify))
}

// NewTlsServerConn 兼容旧接口：用内建证书包装连接
func NewTlsServerConn(conn net.Conn) net.Conn {
	cfg, err := ServerTlsConfig("", "")
	if err != nil {
		logs.Error(err)
		return conn
	}
	return tls.Server(conn, cfg)
}

// NewTlsClientConn 兼容旧接口：不校验服务端证书
func NewTlsClientConn(conn net.Conn) net.Conn {
	return tls.Client(conn, ClientTlsConfig("", false))
}

func generateKeyPair(CommonName string) (rawCert, rawKey []byte, err error) {
	// 生成自签证书：带上 SAN，客户端开启校验时也能用 IP / 域名访问
	priv, err := rsa.GenerateKey(crand.Reader, 2048)
	if err != nil {
		return
	}
	validFor := time.Hour * 24 * 365 * 10 // ten years
	notBefore := time.Now()
	notAfter := notBefore.Add(validFor)
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := crand.Int(crand.Reader, serialNumberLimit)
	if err != nil {
		return
	}
	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"My Company Name LTD."},
			CommonName:   CommonName,
			Country:      []string{"US"},
		},
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{CommonName, "nps.local", "localhost"},
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")},
	}
	derBytes, err := x509.CreateCertificate(crand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return
	}

	rawCert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	rawKey = pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})

	return
}
