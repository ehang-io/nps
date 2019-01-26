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
	return s.dealClient(c, s.config, s.task.Target, "", nil)
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
		host   *utils.Host
		wg     sync.WaitGroup
	)
	for {
		r, err := http.ReadRequest(bufio.NewReader(c))
		if err != nil {
			break
		}
		//首次获取conn
		if isConn {
			if host, err = GetInfoByHost(r.Host); err != nil {
				log.Printf("the host %s is not found !", r.Host)
				break
			}

			host.Client.Cnf.CompressDecode, host.Client.Cnf.CompressEncode = utils.GetCompressType(host.Client.Cnf.Compress)

			if err = s.auth(r, c, host.Client.Cnf.U, host.Client.Cnf.P); err != nil {
				break
			}
			if link, err = s.GetTunnelAndWriteHost(utils.CONN_TCP, host.Client.Id, host.Client.Cnf, host.Target); err != nil {
				log.Println("get bridge tunnel error: ", err)
				break
			}
			if flag, err := link.ReadFlag(); err != nil || flag == utils.CONN_ERROR {
				log.Printf("the host %s connection to %s error", r.Host, host.Target)
				break
			} else {
				wg.Add(1)
				go func() {
					out, _ := utils.Relay(c.Conn, link.Conn, host.Client.Cnf.CompressDecode, host.Client.Cnf.Crypt, host.Client.Cnf.Mux)
					wg.Done()
					s.FlowAddHost(host, 0, out)
				}()
			}
			isConn = false
		}
		//根据设定，修改header和host
		utils.ChangeHostAndHeader(r, host.HostChange, host.HeaderChange, c.Conn.RemoteAddr().String())
		b, err := httputil.DumpRequest(r, true)
		if err != nil {
			break
		}
		s.FlowAddHost(host, int64(len(b)), 0)
		if _, err := link.WriteTo(b, host.Client.Cnf.CompressEncode, host.Client.Cnf.Crypt); err != nil {
			break
		}
	}
	wg.Wait()
	if host != nil && host.Client.Cnf != nil && host.Client.Cnf.Mux && link != nil {
		link.WriteTo([]byte(utils.IO_EOF), host.Client.Cnf.CompressEncode, host.Client.Cnf.Crypt)
		s.bridge.ReturnTunnel(link, host.Client.Id)
	} else if link != nil {
		link.Close()
	}

	if isConn {
		s.writeConnFail(c.Conn)
	}
	c.Close()
	return nil
}
