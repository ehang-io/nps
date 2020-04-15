// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// HTTP reverse proxy handler

package proxy

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
)

type HTTPError struct {
	error
	HTTPCode int
}

func NewHTTPError(code int, errmsg string) error {
	return &HTTPError{
		error:    errors.New(errmsg),
		HTTPCode: code,
	}
}

type ReverseProxy struct {
	*httputil.ReverseProxy
	WebSocketDialContext func(ctx context.Context, network, addr string) (net.Conn, error)
}

func IsWebsocketRequest(req *http.Request) bool {
	containsHeader := func(name, value string) bool {
		items := strings.Split(req.Header.Get(name), ",")
		for _, item := range items {
			if value == strings.ToLower(strings.TrimSpace(item)) {
				return true
			}
		}
		return false
	}
	return containsHeader("Connection", "upgrade") && containsHeader("Upgrade", "websocket")
}

func NewSingleHostReverseProxy(target *url.URL) *ReverseProxy {
	rp := &ReverseProxy{
		ReverseProxy:         httputil.NewSingleHostReverseProxy(target),
		WebSocketDialContext: nil,
	}
	rp.ErrorHandler = rp.errHandler
	return rp
}

func NewReverseProxy(orp *httputil.ReverseProxy) *ReverseProxy {
	rp := &ReverseProxy{
		ReverseProxy:         orp,
		WebSocketDialContext: nil,
	}
	rp.ErrorHandler = rp.errHandler
	return rp
}

func (p *ReverseProxy) errHandler(rw http.ResponseWriter, r *http.Request, e error) {
	if e == io.EOF {
		rw.WriteHeader(521)
		//rw.Write(getWaitingPageContent())
	} else {
		if httperr, ok := e.(*HTTPError); ok {
			rw.WriteHeader(httperr.HTTPCode)
		} else {
			rw.WriteHeader(http.StatusNotFound)
		}
		rw.Write([]byte("error: " + e.Error()))
	}
}

func (p *ReverseProxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if IsWebsocketRequest(req) {
		p.serveWebSocket(rw, req)
	} else {
		p.ReverseProxy.ServeHTTP(rw, req)
	}
}

func (p *ReverseProxy) serveWebSocket(rw http.ResponseWriter, req *http.Request) {
	if p.WebSocketDialContext == nil {
		rw.WriteHeader(500)
		return
	}
	targetConn, err := p.WebSocketDialContext(req.Context(), "tcp", "")
	if err != nil {
		rw.WriteHeader(501)
		return
	}
	defer targetConn.Close()

	p.Director(req)

	hijacker, ok := rw.(http.Hijacker)
	if !ok {
		rw.WriteHeader(500)
		return
	}
	conn, _, errHijack := hijacker.Hijack()
	if errHijack != nil {
		rw.WriteHeader(500)
		return
	}
	defer conn.Close()

	req.Write(targetConn)
	Join(conn, targetConn)
}

func Join(c1 io.ReadWriteCloser, c2 io.ReadWriteCloser) (inCount int64, outCount int64) {
	var wait sync.WaitGroup
	pipe := func(to io.ReadWriteCloser, from io.ReadWriteCloser, count *int64) {
		defer to.Close()
		defer from.Close()
		defer wait.Done()

		*count, _ = io.Copy(to, from)
	}

	wait.Add(2)
	go pipe(c1, c2, &inCount)
	go pipe(c2, c1, &outCount)
	wait.Wait()
	return
}
