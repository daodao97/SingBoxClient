package tun

import (
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"
)

type ActionType = uint8

const (
	ActionTypeReturn ActionType = iota
	ActionTypeReject
	ActionTypeDirect
)

func ParseActionType(action string) (ActionType, error) {
	switch action {
	case "return":
		return ActionTypeReturn, nil
	case "reject":
		return ActionTypeReject, nil
	case "direct":
		return ActionTypeDirect, nil
	default:
		return 0, E.New("unknown action: ", action)
	}
}

func ActionTypeName(actionType ActionType) string {
	switch actionType {
	case ActionTypeReturn:
		return "return"
	case ActionTypeReject:
		return "reject"
	case ActionTypeDirect:
		return "direct"
	default:
		return "unknown"
	}
}

type RouteSession struct {
	IPVersion   uint8
	Network     uint8
	Source      netip.AddrPort
	Destination netip.AddrPort
}

type RouteContext interface {
	WritePacket(packet []byte) error
}

type Router interface {
	RouteConnection(session RouteSession, context RouteContext) RouteAction
}

type RouteAction interface {
	ActionType() ActionType
	Timeout() bool
}

type ActionReturn struct{}

func (r *ActionReturn) ActionType() ActionType {
	return ActionTypeReturn
}

func (r *ActionReturn) Timeout() bool {
	return false
}

type ActionReject struct{}

func (r *ActionReject) ActionType() ActionType {
	return ActionTypeReject
}

func (r *ActionReject) Timeout() bool {
	return false
}

type ActionDirect struct {
	DirectDestination
}

func (r *ActionDirect) ActionType() ActionType {
	return ActionTypeDirect
}
