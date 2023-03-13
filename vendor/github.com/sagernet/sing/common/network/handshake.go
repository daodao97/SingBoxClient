package network

import (
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
)

type HandshakeConn interface {
	HandshakeFailure(err error) error
}

func HandshakeFailure(conn any, err error) error {
	if handshakeConn, isHandshakeConn := common.Cast[HandshakeConn](conn); isHandshakeConn {
		return E.Append(err, handshakeConn.HandshakeFailure(err), func(err error) error {
			return E.Cause(err, "write handshake failure")
		})
	}
	return err
}
