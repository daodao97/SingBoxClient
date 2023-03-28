package tun

import (
	"net/netip"

	E "github.com/sagernet/sing/common/exceptions"
)

type ActionType = uint8

const (
	ActionTypeUnknown ActionType = iota
	ActionTypeReturn
	ActionTypeBlock
	ActionTypeDirect
)

func ParseActionType(action string) (ActionType, error) {
	switch action {
	case "return":
		return ActionTypeReturn, nil
	case "block":
		return ActionTypeBlock, nil
	case "direct":
		return ActionTypeDirect, nil
	default:
		return 0, E.New("unknown action: ", action)
	}
}

func ActionTypeName(actionType ActionType) (string, error) {
	switch actionType {
	case ActionTypeUnknown:
		return "", nil
	case ActionTypeReturn:
		return "return", nil
	case ActionTypeBlock:
		return "block", nil
	case ActionTypeDirect:
		return "direct", nil
	default:
		return "", E.New("unknown action: ", actionType)
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

type ActionBlock struct{}

func (r *ActionBlock) ActionType() ActionType {
	return ActionTypeBlock
}

func (r *ActionBlock) Timeout() bool {
	return false
}

type ActionDirect struct {
	DirectDestination
}

func (r *ActionDirect) ActionType() ActionType {
	return ActionTypeDirect
}
