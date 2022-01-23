package pool

import (
	"ehang.io/nps/lib/common"
	"github.com/panjf2000/ants/v2"
	"io"
	"net"
	"sync"
)

var connBp = NewBufferPool(MaxReadSize)
var packetBp = NewBufferPool(1500)

const MaxReadSize = 32 * 1024

var CopyConnGoroutinePool *ants.PoolWithFunc
var CopyPacketGoroutinePool *ants.PoolWithFunc

type CopyConnGpParams struct {
	Writer io.Writer
	Reader io.Reader
	Wg     *sync.WaitGroup
}

type CopyPacketGpParams struct {
	RPacket net.PacketConn
	WPacket net.PacketConn
	Wg      *sync.WaitGroup
}

func init() {
	var err error
	CopyConnGoroutinePool, err = ants.NewPoolWithFunc(1000000, func(i interface{}) {
		gpp, ok := i.(*CopyConnGpParams)
		if !ok {
			return
		}
		buf := connBp.Get()
		_, _ = common.CopyBuffer(gpp.Writer, gpp.Reader, buf)
		connBp.Put(buf)
		gpp.Wg.Done()
		if v, ok := gpp.Reader.(*net.TCPConn); ok {
			_ = v.CloseWrite()
		}
		if v, ok := gpp.Writer.(*net.TCPConn); ok {
			_ = v.CloseRead()
		}
	})
	if err != nil {
		panic(err)
	}
	CopyPacketGoroutinePool, err = ants.NewPoolWithFunc(1000000, func(i interface{}) {
		cpp, ok := i.(*CopyPacketGpParams)
		if !ok {
			return
		}
		buf := connBp.Get()
		for {
			n, addr, err := cpp.RPacket.ReadFrom(buf)
			if err != nil {
				break
			}
			_, err = cpp.WPacket.WriteTo(buf[:n], addr)
			if err != nil {
				break
			}
		}
		connBp.Put(buf)
		_ = cpp.RPacket.Close()
		_ = cpp.WPacket.Close()
		cpp.Wg.Done()
	})
	if err != nil {
		panic(err)
	}
}
