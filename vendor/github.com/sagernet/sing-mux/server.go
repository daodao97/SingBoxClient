package mux

import (
	"context"
	"net"

	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/task"
)

type ServerHandler interface {
	N.TCPConnectionHandler
	N.UDPConnectionHandler
	E.Handler
}

func HandleConnection(ctx context.Context, handler ServerHandler, logger logger.ContextLogger, conn net.Conn, metadata M.Metadata) error {
	request, err := ReadRequest(conn)
	if err != nil {
		return err
	}
	if request.Padding {
		conn = newPaddingConn(conn)
	}
	session, err := newServerSession(conn, request.Protocol)
	if err != nil {
		return err
	}
	var group task.Group
	group.Append0(func(_ context.Context) error {
		var stream net.Conn
		for {
			stream, err = session.Accept()
			if err != nil {
				return err
			}
			go newConnection(ctx, handler, logger, stream, metadata)
		}
	})
	group.Cleanup(func() {
		session.Close()
	})
	return group.Run(ctx)
}

func newConnection(ctx context.Context, handler ServerHandler, logger logger.ContextLogger, stream net.Conn, metadata M.Metadata) {
	stream = &wrapStream{stream}
	request, err := ReadStreamRequest(stream)
	if err != nil {
		logger.ErrorContext(ctx, err)
		return
	}
	metadata.Destination = request.Destination
	if request.Network == N.NetworkTCP {
		logger.InfoContext(ctx, "inbound multiplex connection to ", metadata.Destination)
		hErr := handler.NewConnection(ctx, &serverConn{ExtendedConn: bufio.NewExtendedConn(stream)}, metadata)
		stream.Close()
		if hErr != nil {
			handler.NewError(ctx, hErr)
		}
	} else {
		var packetConn N.PacketConn
		if !request.PacketAddr {
			logger.InfoContext(ctx, "inbound multiplex packet connection to ", metadata.Destination)
			packetConn = &serverPacketConn{ExtendedConn: bufio.NewExtendedConn(stream), destination: request.Destination}
		} else {
			logger.InfoContext(ctx, "inbound multiplex packet connection")
			packetConn = &serverPacketAddrConn{ExtendedConn: bufio.NewExtendedConn(stream)}
		}
		hErr := handler.NewPacketConnection(ctx, packetConn, metadata)
		stream.Close()
		if hErr != nil {
			handler.NewError(ctx, hErr)
		}
	}
}
