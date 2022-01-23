package process

import (
	"context"
	"crypto/tls"
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/pb"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net"
	"net/http"
	"testing"
	"time"
)

func TestProtobufProcess(t *testing.T) {
	sAddr, err := startHttps(t)
	assert.NoError(t, err)

	h := &PbAppProcessor{}
	ac := &action.LocalAction{
		DefaultAction: action.DefaultAction{},
		TargetAddr:    []string{sAddr},
	}
	ac.Init()
	err = h.Init(ac)
	assert.NoError(t, err)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	go func() {
		for {
			c, err := ln.Accept()
			assert.NoError(t, err)
			go func() {
				_, _ = h.ProcessConn(enet.NewReaderConn(c))
				_ = c.Close()
			}()
		}
	}()

	client := http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		MaxIdleConns:    10000,
		IdleConnTimeout: 30 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			conn, err := net.Dial("tcp", ln.Addr().String())
			_, err = pb.WriteMessage(conn, &pb.AppInfo{AppAddr: sAddr})
			return conn, err
		},
	}}

	resp, err := client.Get(fmt.Sprintf("https://%s%s", ln.Addr().String(), "/now"))
	assert.NoError(t, err)
	assert.NotEmpty(t, resp)
}

func TestProtobufUdpProcess(t *testing.T) {
	finish := make(chan struct{}, 0)
	lAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	assert.NoError(t, err)

	udpServer, err := net.ListenUDP("udp", lAddr)
	assert.NoError(t, err)

	h := &PbAppProcessor{}
	ac := &action.LocalAction{
		DefaultAction: action.DefaultAction{},
		TargetAddr:    []string{udpServer.LocalAddr().String()},
	}
	ac.Init()
	err = h.Init(ac)
	assert.NoError(t, err)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	go func() {
		for {
			c, err := ln.Accept()
			assert.NoError(t, err)
			go func() {
				_, _ = h.ProcessConn(enet.NewReaderConn(c))
				_ = c.Close()
			}()
		}
	}()

	data := []byte{1, 2, 3, 4}
	dataReturn := []byte{4, 5, 6, 7}
	conn, err := net.Dial("tcp", ln.Addr().String())
	_, err = pb.WriteMessage(conn, &pb.AppInfo{AppAddr: udpServer.LocalAddr().String(), ConnType: pb.ConnType_udp})

	go func() {
		b := make([]byte, 1024)
		n, addr, err := udpServer.ReadFrom(b)
		assert.NoError(t, err)
		assert.Equal(t, b[:n], data)

		_, err = udpServer.WriteTo(dataReturn, addr)
		assert.NoError(t, err)
		finish <- struct{}{}
	}()

	c := enet.NewTcpPacketConn(conn)
	_, err = c.WriteTo(data, udpServer.LocalAddr())
	assert.NoError(t, err)

	<-finish
	b := make([]byte, 1024)
	n, addr, err := c.ReadFrom(b)
	assert.NoError(t, err)
	assert.Equal(t, dataReturn, b[:n])
	assert.Equal(t, addr.String(), udpServer.LocalAddr().String())
}
