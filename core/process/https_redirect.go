package process

import (
	"ehang.io/nps/lib/cert"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/enet"
	"github.com/pkg/errors"
)

// HttpsRedirectProcess is used to forward https request by ClientHelloMsg
type HttpsRedirectProcess struct {
	DefaultProcess
	Host string `json:"host" required:"true" placeholder:"https.nps.com" zh_name:"域名"`
}

func (hrp *HttpsRedirectProcess) GetName() string {
	return "https_redirect"
}

func (hrp *HttpsRedirectProcess) GetZhName() string {
	return "https透传"
}

// ProcessConn is used to determine whether to hit the host rule
func (hrp *HttpsRedirectProcess) ProcessConn(c enet.Conn) (bool, error) {
	clientMsg := cert.ClientHelloMsg{}
	buf, err := c.AllBytes()
	if err != nil {
		return false, errors.Wrap(err, "get bytes")
	}
	if !clientMsg.Unmarshal(buf[5:]) {
		return false, errors.New("can not unmarshal client hello message")
	}
	if common.HostContains(hrp.Host, clientMsg.GetServerName()) {
		if err = c.Reset(0); err != nil {
			return false, errors.Wrap(err, "reset reader connection")
		}
		return true, errors.Wrap(hrp.ac.RunConn(c), "run action")
	}
	return false, nil
}
