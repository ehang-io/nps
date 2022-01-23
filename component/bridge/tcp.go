package bridge

import (
	"crypto/tls"
	"ehang.io/nps/lib/logger"
	"ehang.io/nps/lib/pb"
	"ehang.io/nps/lib/pool"
	"ehang.io/nps/transport"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
	"io"
	"net"
	"sync"
)

type tcpServer struct {
	config      *tls.Config // config must contain root ca and server cert
	ln          net.Listener
	gp          *ants.PoolWithFunc
	manager     *manager
	serverCheck func(id string) bool
	clientCheck func(id string) bool
}

func NewTcpServer(ln net.Listener, config *tls.Config, serverCheck, clientCheck func(string) bool) (*tcpServer, error) {
	h := &tcpServer{
		config:      config,
		ln:          ln,
		serverCheck: serverCheck,
		clientCheck: clientCheck,
		manager:     NewManager(),
	}
	var err error
	h.gp, err = ants.NewPoolWithFunc(1000000, func(i interface{}) {
		conn := i.(net.Conn)
		cr := &pb.ConnRequest{}
		_, err := pb.ReadMessage(conn, cr)
		if err != nil {
			_ = conn.Close()
			logger.Warn("read message error", zap.Error(err))
			return
		}
		switch cr.ConnType.(type) {
		case *pb.ConnRequest_AppInfo:
			h.handleApp(cr, conn)
		case *pb.ConnRequest_NpcInfo:
			h.handleClient(cr, conn)
		}
	})
	return h, err
}

func (h *tcpServer) handleApp(cr *pb.ConnRequest, conn net.Conn) {
	if !h.serverCheck(cr.GetId()) {
		_ = conn.Close()
		logger.Error("check server id error", zap.String("id", cr.GetId()))
		return
	}
	clientConn, err := h.manager.GetDataConn(cr.GetAppInfo().GetNpcId())
	if err != nil {
		logger.Error("get client error", zap.Error(err), zap.String("app_info", cr.String()))
		return
	}
	_, err = pb.WriteMessage(clientConn, &pb.ClientRequest{ConnType: &pb.ClientRequest_AppInfo{AppInfo: cr.GetAppInfo()}})
	if err != nil {
		_ = clientConn.Close()
		_ = conn.Close()
		logger.Error("write app_info error", zap.Error(err))
		return
	}
	var wg sync.WaitGroup
	wg.Add(2)
	_ = pool.CopyConnGoroutinePool.Invoke(pool.CopyConnGpParams{Writer: conn, Reader: clientConn, Wg: &wg})
	_ = pool.CopyConnGoroutinePool.Invoke(pool.CopyConnGpParams{Writer: clientConn, Reader: conn, Wg: &wg})
	wg.Wait()
}

func (h *tcpServer) responseClient(conn io.Writer, success bool, msg string) error {
	_, err := pb.WriteMessage(conn, &pb.NpcResponse{Success: success, Message: msg})
	return err
}

func (h *tcpServer) handleClient(cr *pb.ConnRequest, conn net.Conn) {
	if !h.clientCheck(cr.GetId()) {
		_ = conn.Close()
		logger.Error("check server id error", zap.String("id", cr.GetId()))
		_ = h.responseClient(conn, false, "id check failed")
		return
	}
	yc := transport.NewYaMux(conn, nil)
	err := yc.Client()
	if err != nil {
		_ = conn.Close()
		_ = h.responseClient(conn, false, "client failed")
		logger.Error("new yamux client error", zap.Error(err), zap.String("remote address", conn.RemoteAddr().String()))
		return
	}
	_ = h.responseClient(conn, true, "success")
	err = h.manager.SetClient(cr.GetId(), cr.GetNpcInfo().GetTunnelId(), cr.GetNpcInfo().GetIsControlTunnel(), yc)
	if err != nil {
		_ = conn.Close()
		logger.Error("set client error", zap.Error(err), zap.String("info", cr.String()))
	}
}

func (h *tcpServer) run() error {
	h.ln = tls.NewListener(h.ln, h.config)
	for {
		conn, err := h.ln.Accept()
		if err != nil {
			logger.Error("Accept conn error", zap.Error(err))
			return err
		}
		_ = h.gp.Invoke(conn)
	}
}
