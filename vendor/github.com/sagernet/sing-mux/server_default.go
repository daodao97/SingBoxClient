package mux

import (
	"context"
	"net"

	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

func HandleConnectionDefault(ctx context.Context, conn net.Conn) error {
	return HandleConnection(ctx, (*defaultServerHandler)(nil), logger.NOP(), conn, M.Metadata{})
}

type defaultServerHandler struct{}

func (h *defaultServerHandler) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	remoteConn, err := N.SystemDialer.DialContext(ctx, N.NetworkTCP, metadata.Destination)
	if err != nil {
		return err
	}
	return bufio.CopyConn(ctx, conn, remoteConn)
}

func (h *defaultServerHandler) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata M.Metadata) error {
	remoteConn, err := N.SystemDialer.ListenPacket(ctx, metadata.Destination)
	if err != nil {
		return err
	}
	return bufio.CopyPacketConn(ctx, conn, bufio.NewPacketConn(remoteConn))
}

func (h *defaultServerHandler) NewError(ctx context.Context, err error) {
}
