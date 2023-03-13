package tun

import (
	"github.com/sagernet/netlink"
	E "github.com/sagernet/sing/common/exceptions"
)

func (m *defaultInterfaceMonitor) checkUpdate() error {
	ruleList, err := netlink.RuleList(netlink.FAMILY_ALL)
	if err != nil {
		return err
	}

	oldVPNEnabled := m.androidVPNEnabled
	var defaultTableIndex int
	var vpnEnabled bool
	for _, rule := range ruleList {
		if rule.Mask == 0x20000 {
			if rule.UIDRange == nil {
				continue
			}
			vpnEnabled = true
			if m.options.OverrideAndroidVPN {
				defaultTableIndex = rule.Table
				break
			}
		}
		if rule.Mask == 0xFFFF {
			defaultTableIndex = rule.Table
			break
		}
	}
	m.androidVPNEnabled = vpnEnabled

	if defaultTableIndex == 0 {
		return ErrNoRoute
	}

	routes, err := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{Table: defaultTableIndex}, netlink.RT_FILTER_TABLE)
	if err != nil {
		return err
	}

	if len(routes) == 0 {
		return E.Extend(ErrNoRoute, "no route in default table ", defaultTableIndex)
	}

	var link netlink.Link
	link, err = netlink.LinkByIndex(routes[0].LinkIndex)
	if err != nil {
		return err
	}

	oldInterface := m.defaultInterfaceName
	oldIndex := m.defaultInterfaceIndex

	m.defaultInterfaceName = link.Attrs().Name
	m.defaultInterfaceIndex = link.Attrs().Index

	var event int
	if oldInterface != m.defaultInterfaceName || oldIndex != m.defaultInterfaceIndex {
		event |= EventInterfaceUpdate
	}
	if oldVPNEnabled != m.androidVPNEnabled {
		event |= EventAndroidVPNUpdate
	}
	if event != 0 {
		m.emit(event)
	}

	return nil
}
