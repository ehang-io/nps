package proxy

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"ehang.io/nps/bridge"
	"ehang.io/nps/lib/common"
	"ehang.io/nps/lib/conn"
	"ehang.io/nps/lib/file"
	"github.com/astaxie/beego/logs"
)

const (
	// udpSessionIdleTimeout is how long a session may stay silent before it is
	// swept away.
	udpSessionIdleTimeout = 120 * time.Second
	// udpSweepInterval is how often the sweeper scans the session table.
	udpSweepInterval = 30 * time.Second
	// udpReadDeadline is the per-read deadline of the upstream stream.
	udpReadDeadline = 60 * time.Second
	// udpBuildTimeout caps how long a packet waits for a session to be built.
	udpBuildTimeout = 5 * time.Second
)

// udpSession is one source-addr -> npc stream.
//
// Several goroutines may race to build the session for the same source address.
// The first one wins a LoadOrStore placeholder and dials; the losers block on
// ready and then reuse the same stream. That keeps one stream per source addr
// and makes sure only the winner consumes a connection slot.
type udpSession struct {
	target     io.ReadWriteCloser // read/write layer (may add crypt/compress)
	rawConn    net.Conn           // underlying conn, used for SetReadDeadline
	lastActive int64              // unix nano, accessed atomically
	ready      chan struct{}      // closed once the session is usable or failed
	err        error              // set before ready is closed
}

func (u *udpSession) touch() {
	atomic.StoreInt64(&u.lastActive, time.Now().UnixNano())
}

type UdpModeServer struct {
	BaseServer
	addrMap   sync.Map
	listener  *net.UDPConn
	guard     *common.AccessGuard
	closeOnce sync.Once
	closeCh   chan struct{}
}

func NewUdpModeServer(bridge *bridge.Bridge, task *file.Tunnel) *UdpModeServer {
	s := new(UdpModeServer)
	s.bridge = bridge
	s.task = task
	s.closeCh = make(chan struct{})
	return s
}

//开始
func (s *UdpModeServer) Start() error {
	if s.task.ServerIp == "" {
		s.task.ServerIp = "0.0.0.0"
	}
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP(s.task.ServerIp), Port: s.task.Port})
	if err != nil {
		return err
	}
	s.listener = listener
	s.guard = TaskGuard(s.task)
	if s.guard != nil && !s.guard.Disabled() {
		logs.Info(guardDescription(s.guard, s.task.Port))
	}
	go s.sweeper()
	for {
		buf := common.BufPoolUdp.Get().([]byte)
		n, addr, err := listener.ReadFromUDP(buf)
		if err != nil {
			common.PutBufPoolUdp(buf)
			if errors.Is(err, net.ErrClosed) {
				return nil
			}
			logs.Warn("read udp packet, task id %d, error %s", s.task.Id, err.Error())
			continue
		}
		// UDP 无连接，只能按报文来源过滤；这里不参与连接计数，避免正常流量被限速
		if ok, reason := s.guard.Filter(addr); !ok {
			logReject(addr, reason)
			common.PutBufPoolUdp(buf)
			continue
		}
		// process owns buf from now on and returns it to the pool on every path
		go s.process(addr, buf, n)
	}
}

// process handles one datagram and always returns buf to the pool.
func (s *UdpModeServer) process(addr *net.UDPAddr, buf []byte, n int) {
	key := addr.String()
	data := buf[:n]

	// fast path: session already there
	if v, ok := s.addrMap.Load(key); ok {
		s.dispatch(key, v.(*udpSession), data, n)
		common.PutBufPoolUdp(buf)
		return
	}

	// slow path: try to become the builder for this key
	placeholder := &udpSession{ready: make(chan struct{})}
	placeholder.touch()
	if existing, loaded := s.addrMap.LoadOrStore(key, placeholder); loaded {
		// lost the race, wait for the winner and reuse its session
		s.dispatch(key, existing.(*udpSession), data, n)
		common.PutBufPoolUdp(buf)
		return
	}
	s.runSession(addr, key, placeholder, buf, n)
}

// dispatch writes data into the session, waiting for a session under
// construction (bounded by udpBuildTimeout).
func (s *UdpModeServer) dispatch(key string, sess *udpSession, data []byte, n int) {
	if sess.ready != nil {
		select {
		case <-sess.ready:
			if sess.err != nil {
				logs.Trace("udp session build failed for %s, error %s", key, sess.err.Error())
				return
			}
		case <-time.After(udpBuildTimeout):
			logs.Warn("udp session build timeout for %s, drop packet", key)
			return
		case <-s.closeCh:
			return
		}
	}
	if _, err := sess.target.Write(data); err != nil {
		logs.Warn("write udp packet to session %s, error %s", key, err.Error())
		s.removeSession(key, sess)
		return
	}
	sess.touch()
	s.task.Flow.Add(int64(n), 0)
}

// runSession is executed by the placeholder winner: dial the npc stream, send
// the first datagram and run the downlink read loop. It owns buf.
func (s *UdpModeServer) runSession(addr *net.UDPAddr, key string, sess *udpSession, buf []byte, n int) {
	data := buf[:n]

	// single cleanup path for every build failure
	failBuild := func(err error) {
		sess.err = err
		close(sess.ready)
		s.addrMap.Delete(key)
		common.PutBufPoolUdp(buf)
	}

	if err := s.CheckFlowAndConnNum(s.task.Client); err != nil {
		logs.Warn("client id %d, task id %d,error %s, when udp connection", s.task.Client.Id, s.task.Id, err.Error())
		failBuild(err)
		return
	}
	// only the winner holds a slot, released when this function returns
	defer s.task.Client.AddConn()

	link := conn.NewLink(common.CONN_UDP, s.task.Target.TargetStr, s.task.Client.Cnf.Crypt, s.task.Client.Cnf.Compress, addr.String(), s.task.Target.LocalProxy)
	clientConn, err := s.bridge.SendLinkInfo(s.task.Client.Id, link, s.task)
	if err != nil {
		failBuild(err)
		return
	}

	target := conn.GetConn(clientConn, s.task.Client.Cnf.Crypt, s.task.Client.Cnf.Compress, nil, true)
	sess.target = target
	sess.rawConn = clientConn
	sess.touch()
	close(sess.ready) // wake up every waiter

	defer s.removeSession(key, sess)

	logs.Trace("New udp connection,client %d,remote address %s", s.task.Client.Id, addr)

	_, err = target.Write(data)
	common.PutBufPoolUdp(buf)
	if err != nil {
		logs.Warn("write first udp packet to client id %d, task id %d, error %s", s.task.Client.Id, s.task.Id, err.Error())
		return
	}
	s.task.Flow.Add(int64(n), 0)

	// downlink
	rbuf := common.BufPoolUdp.Get().([]byte)
	defer common.PutBufPoolUdp(rbuf)
	for {
		// deadline goes on the underlying conn, target is only a wrapper
		if err := clientConn.SetReadDeadline(time.Now().Add(udpReadDeadline)); err != nil {
			logs.Warn("set udp read deadline, task id %d, error %s", s.task.Id, err.Error())
			return
		}
		rn, err := target.Read(rbuf)
		if err != nil {
			// sweeper close, idle deadline or peer reset all land here
			if !errors.Is(err, net.ErrClosed) && !errors.Is(err, io.EOF) {
				logs.Warn("read udp packet from client id %d, task id %d, error %s", s.task.Client.Id, s.task.Id, err.Error())
			}
			return
		}
		sess.touch()
		if _, err := s.listener.WriteTo(rbuf[:rn], addr); err != nil {
			logs.Warn("write udp packet to remote address %s, task id %d, error %s", addr, s.task.Id, err.Error())
			return
		}
		s.task.Flow.Add(0, int64(rn))
	}
}

// removeSession deletes the entry (only if it is still this session) and
// closes the upstream stream, which unblocks its read loop.
func (s *UdpModeServer) removeSession(key string, sess *udpSession) {
	if v, ok := s.addrMap.Load(key); ok && v.(*udpSession) == sess {
		s.addrMap.Delete(key)
	}
	if sess.target != nil {
		sess.target.Close()
	}
}

// sweeper drops sessions that stayed idle for too long.
func (s *UdpModeServer) sweeper() {
	ticker := time.NewTicker(udpSweepInterval)
	defer ticker.Stop()
	idle := int64(udpSessionIdleTimeout)
	for {
		select {
		case <-s.closeCh:
			return
		case <-ticker.C:
			now := time.Now().UnixNano()
			s.addrMap.Range(func(k, v interface{}) bool {
				sess := v.(*udpSession)
				// placeholders still being built have no target yet
				if sess.target == nil {
					return true
				}
				if now-atomic.LoadInt64(&sess.lastActive) > idle {
					s.removeSession(k.(string), sess)
				}
				return true
			})
		}
	}
}

func (s *UdpModeServer) Close() error {
	s.closeOnce.Do(func() {
		close(s.closeCh)
	})
	s.addrMap.Range(func(k, v interface{}) bool {
		s.removeSession(k.(string), v.(*udpSession))
		return true
	})
	if s.listener == nil {
		return nil
	}
	return s.listener.Close()
}
