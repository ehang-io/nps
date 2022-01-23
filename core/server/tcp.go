package server

import (
	"ehang.io/nps/core/handler"
	"ehang.io/nps/lib/enet"
	"ehang.io/nps/lib/logger"
	"ehang.io/nps/lib/pool"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
	"io"
	"net"
)

var bp = pool.NewBufferPool(1500)

type TcpServer struct {
	BaseServer
	ServerAddr string `json:"server_addr" required:"true" placeholder:"0.0.0.0:8080 or :8080" zh_name:"监听地址"`
	listener   net.Listener
	gp         *ants.PoolWithFunc
}

func (cm *TcpServer) GetServerAddr() string {
	if cm.listener == nil {
		return cm.ServerAddr
	}
	return cm.listener.Addr().String()
}

func (cm *TcpServer) Init() error {
	var err error
	cm.handlers = make(map[string]handler.Handler, 0)
	if err = cm.listen(); err != nil {
		return err
	}
	cm.gp, err = ants.NewPoolWithFunc(1000000, func(i interface{}) {
		rc := enet.NewReaderConn(i.(net.Conn))
		buf := bp.Get()
		defer bp.Put(buf)

		if _, err := io.ReadAtLeast(rc, buf, 3); err != nil {
			logger.Warn("read handle type fom connection failed", zap.String("remote addr", rc.RemoteAddr().String()))
			_ = rc.Close()
			return
		}
		logger.Debug("read handle type", zap.Uint8("type 1", buf[0]), zap.Uint8("type 2", buf[1]),
			zap.Uint8("type 3", buf[2]), zap.String("remote addr", rc.RemoteAddr().String()))

		for _, h := range cm.handlers {
			err = rc.Reset(0)
			if err != nil {
				logger.Warn("reset connection error", zap.Error(err), zap.String("remote addr", rc.RemoteAddr().String()))
				_ = rc.Close()
				return
			}
			ok, err := h.HandleConn(buf, rc)
			if err != nil {
				logger.Warn("handle connection error", zap.Error(err), zap.String("remote addr", rc.RemoteAddr().String()))
				return
			}
			if ok {
				logger.Debug("handle connection success", zap.String("remote addr", rc.RemoteAddr().String()))
				return
			}
		}
	})
	return nil
}

func (cm *TcpServer) GetName() string {
	return "tcp"
}

func (cm *TcpServer) GetZhName() string {
	return "tcp服务"
}

// create a listener accept user and npc
func (cm *TcpServer) listen() error {
	var err error
	cm.listener, err = net.Listen("tcp", cm.ServerAddr)
	if err != nil {
		return err
	}
	return nil
}

func (cm *TcpServer) Serve() {
	for {
		c, err := cm.listener.Accept()
		if err != nil {
			logger.Error("accept enet error", zap.Error(err))
			break
		}
		_ = cm.gp.Invoke(c)
	}
}
