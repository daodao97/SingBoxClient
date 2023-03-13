package network

import (
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
)

var ErrUnknownNetwork = E.New("unknown network")

//goland:noinspection GoNameStartsWithPackageName
const (
	NetworkTCP = "tcp"
	NetworkUDP = "udp"
)

//goland:noinspection GoNameStartsWithPackageName
func NetworkName(network string) string {
	if strings.HasPrefix(network, "tcp") {
		return NetworkTCP
	} else if strings.HasPrefix(network, "udp") {
		return NetworkUDP
	} else {
		return network
	}
}
