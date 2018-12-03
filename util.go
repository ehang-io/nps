package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/golang/snappy"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

var (
	disabledRedirect = errors.New("disabled redirect.")
)

const (
	COMPRESS_NONE = iota
	COMPRESS_SNAPY_ENCODE
	COMPRESS_SNAPY_DECODE
	COMPRESS_GZIP_ENCODE
	COMPRESS_GZIP_DECODE
)

func BadRequest(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
}

//发送请求并转为bytes
func GetEncodeResponse(req *http.Request) ([]byte, error) {
	var respBytes []byte
	client := new(http.Client)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return disabledRedirect
	}
	resp, err := client.Do(req)
	disRedirect := err != nil && strings.Contains(err.Error(), disabledRedirect.Error())
	if err != nil && !disRedirect {
		return respBytes, err
	}
	if !disRedirect {
		defer resp.Body.Close()
	} else {
		resp.Body = nil
		resp.ContentLength = 0
	}
	respBytes, err = EncodeResponse(resp)
	return respBytes, nil
}

// 将request转为bytes
func EncodeRequest(r *http.Request) ([]byte, error) {
	raw := bytes.NewBuffer([]byte{})
	reqBytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		return nil, err
	}
	binary.Write(raw, binary.LittleEndian, bool(r.URL.Scheme == "https"))
	binary.Write(raw, binary.LittleEndian, reqBytes)
	return raw.Bytes(), nil
}

// 将字节转为request
func DecodeRequest(data []byte) (*http.Request, error) {
	req, err := http.ReadRequest(bufio.NewReader(bytes.NewReader(data[1:])))
	if err != nil {
		return nil, err
	}
	str := strings.Split(req.Host, ":")
	req.Host, err = getHost(str[0])
	if err != nil {
		return nil, err
	}
	scheme := "http"
	if data[0] == 1 {
		scheme = "https"
	}
	req.URL, _ = url.Parse(fmt.Sprintf("%s://%s%s", scheme, req.Host, req.RequestURI))
	req.RequestURI = ""
	return req, nil
}

//// 将response转为字节
func EncodeResponse(r *http.Response) ([]byte, error) {
	respBytes, err := httputil.DumpResponse(r, true)
	if err != nil {
		return nil, err
	}
	if config.Replace == 1 {
		respBytes = replaceHost(respBytes)
	}
	return respBytes, nil
}

// 将字节转为response
func DecodeResponse(data []byte) (*http.Response, error) {
	resp, err := http.ReadResponse(bufio.NewReader(bytes.NewReader(data)), nil)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func getHost(str string) (string, error) {
	for _, v := range config.SiteList {
		if v.Host == str {
			return v.Url + ":" + strconv.Itoa(v.Port), nil
		}
	}
	return "", errors.New("没有找到解析的的host!")
}

func replaceHost(resp []byte) []byte {
	str := string(resp)
	for _, v := range config.SiteList {
		str = strings.Replace(str, v.Url+":"+strconv.Itoa(v.Port), v.Host, -1)
		str = strings.Replace(str, v.Url, v.Host, -1)
	}
	return []byte(str)
}

func relay(in, out *Conn, compressType int) {
	buf := make([]byte, 32*1024)
	switch compressType {
	case COMPRESS_GZIP_ENCODE:
		w := gzip.NewWriter(in)
		for {
			n, err := out.Read(buf)
			if err != nil || err == io.EOF {
				break
			}
			if _, err = w.Write(buf[:n]); err != nil {
				break
			}
			if err = w.Flush(); err != nil {
				log.Println(err)
				break
			}
		}
		w.Close()
	case COMPRESS_SNAPY_ENCODE:
		w := snappy.NewBufferedWriter(in)
		for {
			n, err := out.Read(buf)
			if err != nil || err == io.EOF {
				break
			}
			if _, err = w.Write(buf[:n]); err != nil {
				break
			}
			if err = w.Flush(); err != nil {
				log.Println(err)
				break
			}
		}
		w.Close()
	case COMPRESS_GZIP_DECODE:
		r, err := gzip.NewReader(out)
		if err != nil {
			return
		}
		io.Copy(in, r)
	case COMPRESS_SNAPY_DECODE:
		r := snappy.NewReader(out)
		io.Copy(in, r)
	default:
		io.Copy(in, out)
	}
	out.Close()
}
