package process

import (
	"ehang.io/nps/core/action"
	"ehang.io/nps/lib/cert"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/logger"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

type HttpsServeProcess struct {
	CertFile string `json:"cert_file" required:"true" placeholder:"/var/cert/cert.pem" zh_name:"cert文件路径"`
	KeyFile  string `json:"key_file" required:"true" placeholder:"/var/cert/key.pem" zh_name:"key文件路径"`
	HttpServeProcess
}

func (hsp *HttpsServeProcess) GetName() string {
	return "https_serve"
}
func (hsp *HttpsServeProcess) GetZhName() string {
	return "https服务"
}

func (hsp *HttpsServeProcess) Init(ac action.Action) error {
	hsp.tls = true
	err := hsp.HttpServeProcess.Init(ac)
	go hsp.httpServe.ServeTLS(hsp.CertFile, hsp.KeyFile)
	return err
}

func (hsp *HttpsServeProcess) ProcessConn(c enet.Conn) (bool, error) {
	clientMsg := cert.ClientHelloMsg{}
	b, err := c.AllBytes()
	if err != nil {
		return false, errors.Wrap(err, "get bytes")
	}
	if !clientMsg.Unmarshal(b[5:]) {
		return false, errors.New("can not unmarshal client hello message")
	}
	if common.HostContains(hsp.Host, clientMsg.GetServerName()) {
		logger.Debug("do https serve failed", zap.String("host", clientMsg.GetServerName()), zap.String("url", hsp.RouteUrl))
		if err := c.Reset(0); err != nil {
			return true, errors.Wrap(err, "reset reader connection")
		}
		return true, hsp.HttpServeProcess.ln.SendConn(c)
	}

	return false, nil
}
