//go:build with_gvisor && darwin

package tun

import (
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"

	"gvisor.dev/gvisor/pkg/bufferv2"
	"gvisor.dev/gvisor/pkg/tcpip"
	"gvisor.dev/gvisor/pkg/tcpip/header"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

var _ GVisorTun = (*NativeTun)(nil)

func (t *NativeTun) NewEndpoint() (stack.LinkEndpoint, error) {
	return &DarwinEndpoint{tun: t}, nil
}

var _ stack.LinkEndpoint = (*DarwinEndpoint)(nil)

type DarwinEndpoint struct {
	tun        *NativeTun
	dispatcher stack.NetworkDispatcher
}

func (e *DarwinEndpoint) MTU() uint32 {
	return e.tun.mtu
}

func (e *DarwinEndpoint) MaxHeaderLength() uint16 {
	return 0
}

func (e *DarwinEndpoint) LinkAddress() tcpip.LinkAddress {
	return ""
}

func (e *DarwinEndpoint) Capabilities() stack.LinkEndpointCapabilities {
	return stack.CapabilityNone
}

func (e *DarwinEndpoint) Attach(dispatcher stack.NetworkDispatcher) {
	if dispatcher == nil && e.dispatcher != nil {
		e.dispatcher = nil
		return
	}
	if dispatcher != nil && e.dispatcher == nil {
		e.dispatcher = dispatcher
		go e.dispatchLoop()
	}
}

func (e *DarwinEndpoint) dispatchLoop() {
	_buffer := buf.StackNewSize(int(e.tun.mtu) + 4)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	data := buffer.FreeBytes()
	for {
		n, err := e.tun.tunFile.Read(data)
		if err != nil {
			break
		}
		packet := data[4:n]
		var networkProtocol tcpip.NetworkProtocolNumber
		switch header.IPVersion(packet) {
		case header.IPv4Version:
			networkProtocol = header.IPv4ProtocolNumber
			if header.IPv4(packet).DestinationAddress() == tcpip.Address(e.tun.inet4Address) {
				e.tun.tunFile.Write(data[:n])
				continue
			}
		case header.IPv6Version:
			networkProtocol = header.IPv6ProtocolNumber
			if header.IPv6(packet).DestinationAddress() == tcpip.Address(e.tun.inet6Address) {
				e.tun.tunFile.Write(data[:n])
				continue
			}
		default:
			e.tun.tunFile.Write(data[:n])
			continue
		}
		pkt := stack.NewPacketBuffer(stack.PacketBufferOptions{
			Payload:           bufferv2.MakeWithData(data[4:n]),
			IsForwardedPacket: true,
		})
		pkt.NetworkProtocolNumber = networkProtocol
		dispatcher := e.dispatcher
		if dispatcher == nil {
			pkt.DecRef()
			return
		}
		dispatcher.DeliverNetworkPacket(networkProtocol, pkt)
		pkt.DecRef()
	}
}

func (e *DarwinEndpoint) IsAttached() bool {
	return e.dispatcher != nil
}

func (e *DarwinEndpoint) Wait() {
}

func (e *DarwinEndpoint) ARPHardwareType() header.ARPHardwareType {
	return header.ARPHardwareNone
}

func (e *DarwinEndpoint) AddHeader(buffer *stack.PacketBuffer) {
}

func (e *DarwinEndpoint) WritePackets(packetBufferList stack.PacketBufferList) (int, tcpip.Error) {
	var n int
	for _, packet := range packetBufferList.AsSlice() {
		var packetHeader []byte
		switch packet.NetworkProtocolNumber {
		case header.IPv4ProtocolNumber:
			packetHeader = packetHeader4[:]
		case header.IPv6ProtocolNumber:
			packetHeader = packetHeader6[:]
		}
		_, err := bufio.WriteVectorised(e.tun.tunWriter, append([][]byte{packetHeader}, packet.AsSlices()...))
		if err != nil {
			return n, &tcpip.ErrAborted{}
		}
		n++
	}
	return n, nil
}
