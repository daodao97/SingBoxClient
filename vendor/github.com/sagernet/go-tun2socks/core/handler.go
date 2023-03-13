package core

import (
	M "github.com/sagernet/sing/common/metadata"
	"net"
)

// TCPConnHandler handles TCP connections comming from TUN.
type TCPConnHandler interface {
	// Handle handles the conn for target.
	Handle(conn net.Conn) error
}

// UDPConnHandler handles UDP connections comming from TUN.
type UDPConnHandler interface {
	// ReceiveTo will be called when data arrives from TUN.
	ReceiveTo(conn UDPConn, data []byte, addr M.Socksaddr) error
}

var tcpConnHandler TCPConnHandler
var udpConnHandler UDPConnHandler

func RegisterTCPConnHandler(h TCPConnHandler) {
	tcpConnHandler = h
}

func RegisterUDPConnHandler(h UDPConnHandler) {
	udpConnHandler = h
}
