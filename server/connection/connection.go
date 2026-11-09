package connection

import (
	"crypto/tls"
	"fmt"
	"net"
	"os"
	"strconv"

	"ehang.io/nps/lib/crypt"
	"ehang.io/nps/lib/pmux"
	"github.com/astaxie/beego"
	"github.com/astaxie/beego/logs"
)

var pMux *pmux.PortMux
var bridgePort string
var httpsPort string
var httpPort string
var webPort string

func InitConnectionService() {
	bridgePort = beego.AppConfig.String("bridge_port")
	httpsPort = beego.AppConfig.String("https_proxy_port")
	httpPort = beego.AppConfig.String("http_proxy_port")
	webPort = beego.AppConfig.String("web_port")

	if httpPort == bridgePort || httpsPort == bridgePort || webPort == bridgePort {
		port, err := strconv.Atoi(bridgePort)
		if err != nil {
			logs.Error("bridge_port %q is not a valid port: %s", bridgePort, err.Error())
			os.Exit(1)
		}
		pMux, err = pmux.NewPortMux(port, beego.AppConfig.String("web_host"))
		if err != nil {
			logs.Error("start port mux on %d error: %s", port, err.Error())
			os.Exit(1)
		}
	}
}

func GetBridgeListener(tp string) (net.Listener, error) {
	logs.Info("server start, the bridge type is %s, the bridge port is %s", tp, bridgePort)
	var p int
	var err error
	if p, err = strconv.Atoi(bridgePort); err != nil {
		return nil, err
	}
	if pMux != nil {
		if tp == "tls" {
			return nil, fmt.Errorf("bridge_port is shared with the web/https port, tls bridge needs its own port")
		}
		return pMux.GetClientListener(), nil
	}
	l, err := net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(beego.AppConfig.String("bridge_ip")), Port: p})
	if err != nil {
		return nil, err
	}
	if tp == "tls" {
		cfg, err := crypt.ServerTlsConfig(beego.AppConfig.DefaultString("bridge_cert_file", ""), beego.AppConfig.DefaultString("bridge_key_file", ""))
		if err != nil {
			_ = l.Close()
			return nil, err
		}
		logs.Info("bridge uses tls, clients must connect with conn_type=tls")
		return tls.NewListener(l, cfg), nil
	}
	return l, nil
}

func GetHttpListener() (net.Listener, error) {
	if pMux != nil && httpPort == bridgePort {
		logs.Info("start http listener, port is %s", bridgePort)
		return pMux.GetHttpListener(), nil
	}
	logs.Info("start http listener, port is %s", httpPort)
	return getTcpListener(beego.AppConfig.String("http_proxy_ip"), httpPort)
}

func GetHttpsListener() (net.Listener, error) {
	if pMux != nil && httpsPort == bridgePort {
		logs.Info("start https listener, port is %s", bridgePort)
		return pMux.GetHttpsListener(), nil
	}
	logs.Info("start https listener, port is %s", httpsPort)
	return getTcpListener(beego.AppConfig.String("http_proxy_ip"), httpsPort)
}

func GetWebManagerListener() (net.Listener, error) {
	if pMux != nil && webPort == bridgePort {
		logs.Info("web management start, access port is %s", bridgePort)
		return pMux.GetManagerListener(), nil
	}
	logs.Info("web management start, access port is %s", webPort)
	return getTcpListener(beego.AppConfig.String("web_ip"), webPort)
}

func getTcpListener(ip, p string) (net.Listener, error) {
	port, err := strconv.Atoi(p)
	if err != nil {
		return nil, fmt.Errorf("listen port %q is not a valid number: %w", p, err)
	}
	if ip == "" {
		ip = "0.0.0.0"
	}
	return net.ListenTCP("tcp", &net.TCPAddr{IP: net.ParseIP(ip), Port: port})
}
