package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
)

/*

  http.ReadRequest()
  http.ReadResponse()
  httputil.DumpRequest()
  httputil.DumpResponse()
*/

// 将request 的处理
func EncodeRequest(r *http.Request) ([]byte, error) {
	raw := bytes.NewBuffer([]byte{})
	// 写签名
	binary.Write(raw, binary.LittleEndian, []byte("sign"))
	reqBytes, err := httputil.DumpRequest(r, true)
	if err != nil {
		return nil, err
	}
	// 写body数据长度 + 1
	binary.Write(raw, binary.LittleEndian, int32(len(reqBytes)+1))
	// 判断是否为http或者https的标识1字节
	binary.Write(raw, binary.LittleEndian, bool(r.URL.Scheme == "https"))
	if err := binary.Write(raw, binary.LittleEndian, reqBytes); err != nil {
		return nil, err
	}
	return raw.Bytes(), nil
}

// 将字节转为request
func DecodeRequest(data []byte) (*http.Request, error) {
	if len(data) <= 100 {
		return nil, errors.New("待解码的字节长度太小")
	}
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
	raw := bytes.NewBuffer([]byte{})
	binary.Write(raw, binary.LittleEndian, []byte("sign"))
	respBytes, err := httputil.DumpResponse(r, true)
	if config.Replace == 1 {
		respBytes = replaceHost(respBytes)
	}
	if err != nil {
		return nil, err
	}
	binary.Write(raw, binary.LittleEndian, int32(len(respBytes)))
	if err := binary.Write(raw, binary.LittleEndian, respBytes); err != nil {
		return nil, err
	}
	return raw.Bytes(), nil
}

//// 将字节转为response
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
