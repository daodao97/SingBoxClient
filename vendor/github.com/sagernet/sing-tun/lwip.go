//go:build with_lwip

package tun

import (
	"context"
	"net"
	"net/netip"
	"os"

	lwip "github.com/sagernet/go-tun2socks/core"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/udpnat"
)

type LWIP struct {
	ctx        context.Context
	tun        Tun
	tunMtu     uint32
	udpTimeout int64
	handler    Handler
	stack      lwip.LWIPStack
	udpNat     *udpnat.Service[netip.AddrPort]
}

func NewLWIP(
	options StackOptions,
) (Stack, error) {
	return &LWIP{
		ctx:     options.Context,
		tun:     options.Tun,
		tunMtu:  options.MTU,
		handler: options.Handler,
		stack:   lwip.NewLWIPStack(),
		udpNat:  udpnat.New[netip.AddrPort](options.UDPTimeout, options.Handler),
	}, nil
}

func (l *LWIP) Start() error {
	lwip.RegisterTCPConnHandler(l)
	lwip.RegisterUDPConnHandler(l)
	lwip.RegisterOutputFn(l.tun.Write)
	go l.loopIn()
	return nil
}

func (l *LWIP) loopIn() {
	if winTun, isWintun := l.tun.(WinTun); isWintun {
		l.loopInWintun(winTun)
		return
	}
	mtu := int(l.tunMtu) + PacketOffset
	_buffer := buf.StackNewSize(mtu)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	data := buffer.FreeBytes()
	for {
		n, err := l.tun.Read(data)
		if err != nil {
			return
		}
		_, err = l.stack.Write(data[PacketOffset:n])
		if err != nil {
			if err.Error() == "stack closed" {
				return
			}
			l.handler.NewError(context.Background(), err)
		}
	}
}

func (l *LWIP) loopInWintun(tun WinTun) {
	for {
		packet, release, err := tun.ReadPacket()
		if err != nil {
			return
		}
		_, err = l.stack.Write(packet)
		release()
		if err != nil {
			if err.Error() == "stack closed" {
				return
			}
			l.handler.NewError(context.Background(), err)
		}
	}
}

func (l *LWIP) Close() error {
	lwip.RegisterTCPConnHandler(nil)
	lwip.RegisterUDPConnHandler(nil)
	lwip.RegisterOutputFn(func(bytes []byte) (int, error) {
		return 0, os.ErrClosed
	})
	return l.stack.Close()
}

func (l *LWIP) Handle(conn net.Conn) error {
	lAddr := conn.LocalAddr()
	rAddr := conn.RemoteAddr()
	if lAddr == nil || rAddr == nil {
		conn.Close()
		return nil
	}
	go func() {
		var metadata M.Metadata
		metadata.Source = M.SocksaddrFromNet(lAddr)
		metadata.Destination = M.SocksaddrFromNet(rAddr)
		hErr := l.handler.NewConnection(l.ctx, conn, metadata)
		if hErr != nil {
			conn.(lwip.TCPConn).Abort()
		}
	}()
	return nil
}

func (l *LWIP) ReceiveTo(conn lwip.UDPConn, data []byte, addr M.Socksaddr) error {
	var upstreamMetadata M.Metadata
	upstreamMetadata.Source = conn.LocalAddr()
	upstreamMetadata.Destination = addr

	l.udpNat.NewPacket(
		l.ctx,
		upstreamMetadata.Source.AddrPort(),
		buf.As(data).ToOwned(),
		upstreamMetadata,
		func(natConn N.PacketConn) N.PacketWriter {
			return &LWIPUDPBackWriter{conn}
		},
	)
	return nil
}

type LWIPUDPBackWriter struct {
	conn lwip.UDPConn
}

func (w *LWIPUDPBackWriter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	return common.Error(w.conn.WriteFrom(buffer.Bytes(), destination))
}

func (w *LWIPUDPBackWriter) Close() error {
	return w.conn.Close()
}
