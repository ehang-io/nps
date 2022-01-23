// +build !linux

package process

import (
	"ehang.io/nps/lib/enet"
)

type TransparentProcess struct {
	DefaultProcess
}

func (tp *TransparentProcess) GetName() string {
	return "transparent"
}

func (tp *TransparentProcess) GetZhName() string {
	return "透明代理"
}

func (tp *TransparentProcess) ProcessConn(c enet.Conn) (bool, error) {
	return false, nil
}
