package server

import (
	"bufio"
	"github.com/cnlh/easyProxy/utils"
	"log"
	"net/http"
	"net/http/httputil"
	"sync"
)

type process func(c *utils.Conn, s *TunnelModeServer) error

//tcp隧道模式
func ProcessTunnel(c *utils.Conn, s *TunnelModeServer) error {
	_, _, rb, err, r := c.GetHost()
	if err == nil {
		if err := s.auth(r, c, s.config.U, s.config.P); err != nil {
			return err
		}
	}
	return s.dealClient(c, s.config, s.config.Target, "", rb)
}

//http代理模式
func ProcessHttp(c *utils.Conn, s *TunnelModeServer) error {
	method, addr, rb, err, r := c.GetHost()
	if err != nil {
		log.Println(err)
		c.Close()
		return err
	}
	if err := s.auth(r, c, s.config.U, s.config.P); err != nil {
		return err
	}
	return s.dealClient(c, s.config, addr, method, rb)
}

//多客户端域名代理
func ProcessHost(c *utils.Conn, s *TunnelModeServer) error {
	var (
		isConn = true
		link   *utils.Conn
		cnf    *ServerConfig
		host   *HostList
		wg     sync.WaitGroup
	)
	for {
		r, err := http.ReadRequest(bufio.NewReader(c))
		if err != nil {
			break
		}
		//首次获取conn
		if isConn {
			isConn = false
			if host, cnf, err = GetKeyByHost(r.Host); err != nil {
				log.Printf("the host %s is not found !", r.Host)
				break
			}

			if err = s.auth(r, c, cnf.U, cnf.P); err != nil {
				break
			}

			if link, err = s.GetTunnelAndWriteHost(utils.CONN_TCP, cnf, host.Target); err != nil {
				log.Println("get bridge tunnel error: ", err)
				break
			}

			if flag, err := link.ReadFlag(); err != nil || flag == utils.CONN_ERROR {
				log.Printf("the host %s connection to %s error", r.Host, host.Target)
				break
			} else {
				wg.Add(1)
				go func() {
					utils.Relay(c.Conn, link.Conn, cnf.CompressDecode, cnf.Crypt, cnf.Mux)
					wg.Done()
				}()
			}
		}
		utils.ChangeHostAndHeader(r, host.HostChange, host.HeaderChange, c.Conn.RemoteAddr().String())
		b, err := httputil.DumpRequest(r, true)
		if err != nil {
			break
		}
		if _, err := link.WriteTo(b, cnf.CompressEncode, cnf.Crypt); err != nil {
			break
		}
	}
	wg.Wait()
	if cnf != nil && cnf.Mux && link != nil {
		link.WriteTo([]byte(utils.IO_EOF), cnf.CompressEncode, cnf.Crypt)
		s.bridge.ReturnTunnel(link, getverifyval(cnf.VerifyKey))
	} else if link != nil {
		link.Close()
	}
	c.Close()
	return nil
}
