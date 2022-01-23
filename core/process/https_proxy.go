package process

import (
	"crypto/tls"
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/enet"
)

type HttpsProxyProcess struct {
	CertFile string `json:"cert_file" required:"true" placeholder:"/var/cert/cert.pem" zh_name:"cert文件路径"`
	KeyFile  string `json:"key_file" required:"true" placeholder:"/var/cert/key.pem" zh_name:"key文件路径"`
	config   *tls.Config
	HttpProxyProcess
}

func (hpp *HttpsProxyProcess) GetName() string {
	return "https_proxy"
}

func (hpp *HttpsProxyProcess) GetZhName() string {
	return "https代理"
}

func (hpp *HttpsProxyProcess) Init(ac action.Action) error {
	cer, err := tls.LoadX509KeyPair(hpp.CertFile, hpp.KeyFile)
	if err != nil {
		return err
	}
	hpp.config = &tls.Config{Certificates: []tls.Certificate{cer}}
	hpp.ac = ac
	return nil
}

func (hpp *HttpsProxyProcess) ProcessConn(c enet.Conn) (bool, error) {
	return hpp.HttpProxyProcess.ProcessConn(enet.NewReaderConn(tls.Server(c, hpp.config)))
}
