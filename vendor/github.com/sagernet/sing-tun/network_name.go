package tun

import (
	"strconv"

	"github.com/sagernet/sing-tun/internal/clashtcpip"
	F "github.com/sagernet/sing/common/format"
	N "github.com/sagernet/sing/common/network"
)

func NetworkName(network uint8) string {
	switch network {
	case clashtcpip.TCP:
		return N.NetworkTCP
	case clashtcpip.UDP:
		return N.NetworkUDP
	case clashtcpip.ICMP:
		return N.NetworkICMPv4
	case clashtcpip.ICMPv6:
		return N.NetworkICMPv6
	}
	return F.ToString(network)
}

func NetworkFromName(name string) uint8 {
	switch name {
	case N.NetworkTCP:
		return clashtcpip.TCP
	case N.NetworkUDP:
		return clashtcpip.UDP
	case N.NetworkICMPv4:
		return clashtcpip.ICMP
	case N.NetworkICMPv6:
		return clashtcpip.ICMPv6
	}
	parseNetwork, err := strconv.ParseUint(name, 10, 8)
	if err != nil {
		return 0
	}
	return uint8(parseNetwork)
}
