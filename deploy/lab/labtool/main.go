// labtool is the small helper the two-container lab runs inside its
// containers: a static web server for the target, a stand-in for GitHub's
// release downloads, a timed GET, the web login-and-push the nps panel does,
// the alerts the server has raised, and a man in the middle that relays.
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"time"

	"ehang.io/nps/lib/bridgetls"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: labtool web|github|get|fresh|push|alerts|mitm ...")
		os.Exit(2)
	}
	a := os.Args[2:]
	switch os.Args[1] {
	case "web": // web <dir> <addr>
		fmt.Println(http.ListenAndServe(a[1], http.FileServer(http.Dir(a[0]))))
	case "github": // github <release dir> <cert out>
		github(a[0], a[1])
	case "get": // get <url> <n>
		os.Exit(get(a[0], a[1]))
	case "push": // push <web addr> <user> <password> <client id> <tag>
		push(a[0], a[1], a[2], a[3], a[4])
	case "fresh": // fresh <url> <n>
		fresh(a[0], a[1])
	case "alerts": // alerts <web addr> <user> <password>
		alerts(a[0], a[1], a[2])
	case "mitm": // mitm <listen> <upstream>
		mitm(a[0], a[1])
	default:
		os.Exit(2)
	}
}

// mitm is the attack a pinned or static secret cannot stop: it terminates the
// client's TLS with a certificate of its own, opens its own TLS to the real
// server, and carries every application byte across unchanged.
func mitm(listen, upstream string) {
	cfg, err := bridgetls.ServerConfig()
	if err != nil {
		panic(err)
	}
	l, err := net.Listen("tcp", listen)
	if err != nil {
		panic(err)
	}
	for {
		raw, err := l.Accept()
		if err != nil {
			return
		}
		go func() {
			down, err := bridgetls.Server(raw, cfg)
			if err != nil {
				return
			}
			u, err := net.Dial("tcp", upstream)
			if err != nil {
				down.Close()
				return
			}
			up, err := bridgetls.Client(u, bridgetls.ClientConfig())
			if err != nil {
				down.Close()
				return
			}
			var toServer, toClient int64
			done := make(chan struct{})
			go func() { toServer, _ = io.Copy(up, down); up.Close(); close(done) }()
			toClient, _ = io.Copy(down, up)
			down.Close()
			<-done
			fmt.Printf("mitm: relayed a session, %d bytes to the server, %d to the client\n", toServer, toClient)
		}()
	}
}

// fresh makes n requests, each on a connection of its own, and prints the time
// to the first byte of each reply and the median: the cost of a new
// connection, which a kept one hides.
func fresh(u, n string) {
	count, err := strconv.Atoi(n)
	if err != nil || count < 1 {
		fmt.Println("  fresh: the count must be a positive number")
		os.Exit(2)
	}
	var ds []time.Duration
	for i := 0; i < count; i++ {
		c := http.Client{
			Timeout:   20 * time.Second,
			Transport: &http.Transport{DisableKeepAlives: true},
			// A redirect is a second request, on a second connection, and
			// would count twice.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
		t0 := time.Now()
		resp, err := c.Get(u)
		if err != nil {
			fmt.Printf("  fresh: %v\n", err)
			os.Exit(1)
		}
		if resp.StatusCode != 200 {
			fmt.Printf("  fresh: %s answered %s\n", u, resp.Status)
			os.Exit(1)
		}
		ds = append(ds, time.Since(t0))
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		time.Sleep(150 * time.Millisecond)
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	fmt.Printf("median %d ms (min %d, max %d, n=%d)\n", ds[len(ds)/2].Milliseconds(), ds[0].Milliseconds(), ds[len(ds)-1].Milliseconds(), len(ds))
}

func alerts(web, user, pass string) {
	jar, _ := cookiejar.New(nil)
	c := http.Client{Jar: jar, Timeout: 15 * time.Second}
	if _, err := c.PostForm(web+"/login/verify", url.Values{"username": {user}, "password": {pass}}); err != nil {
		fmt.Println("login:", err)
		os.Exit(1)
	}
	r, err := c.Get(web + "/stats/alerts")
	if err != nil {
		fmt.Println("alerts:", err)
		os.Exit(1)
	}
	b, _ := io.ReadAll(r.Body)
	fmt.Println(string(b))
}

// github answers every path with the file of that name in dir, over TLS with
// a certificate for github.com that it writes to certOut, to be trusted with
// SSL_CERT_FILE. Release downloads are all the self-updater asks of GitHub.
func github(dir, certOut string) {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "github.com"},
		DNSNames:  []string{"github.com"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	os.WriteFile(certOut, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
	srv := &http.Server{
		Addr: ":443",
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			f := filepath.Join(dir, filepath.Base(r.URL.Path))
			fmt.Println("github:", r.URL.Path, "->", f)
			http.ServeFile(w, r, f)
		}),
		TLSConfig: &tls.Config{Certificates: []tls.Certificate{{Certificate: [][]byte{der}, PrivateKey: key}}},
	}
	fmt.Println(srv.ListenAndServeTLS("", ""))
}

func get(u, n string) int {
	count, _ := strconv.Atoi(n)
	bad := 0
	for i := 0; i < count; i++ {
		t0 := time.Now()
		c := http.Client{Timeout: 15 * time.Second}
		resp, err := c.Get(u)
		if err != nil {
			fmt.Printf("  GET failed after %v: %v\n", time.Since(t0).Round(time.Millisecond), err)
			bad++
			continue
		}
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		fmt.Printf("  GET %d, %d bytes, %v\n", resp.StatusCode, len(b), time.Since(t0).Round(time.Millisecond))
		if resp.StatusCode != 200 {
			bad++
		}
		time.Sleep(200 * time.Millisecond)
	}
	return bad
}

func push(web, user, pass, id, tag string) {
	jar, _ := cookiejar.New(nil)
	c := http.Client{Jar: jar, Timeout: 15 * time.Second}
	r, err := c.PostForm(web+"/login/verify", url.Values{"username": {user}, "password": {pass}})
	if err != nil {
		fmt.Println("login:", err)
		os.Exit(1)
	}
	b, _ := io.ReadAll(r.Body)
	fmt.Println("login:", string(b))
	r, err = c.PostForm(web+"/client/pushupdate", url.Values{"id": {id}, "tag": {tag}})
	if err != nil {
		fmt.Println("push:", err)
		os.Exit(1)
	}
	b, _ = io.ReadAll(r.Body)
	fmt.Println("push:", string(b))
}
