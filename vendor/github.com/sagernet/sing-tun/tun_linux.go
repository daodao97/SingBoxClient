package tun

import (
	"math/rand"
	"net"
	"net/netip"
	"os"
	"runtime"
	"unsafe"

	"github.com/sagernet/netlink"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/common/x/list"

	"golang.org/x/sys/unix"
)

type NativeTun struct {
	tunFd             int
	tunFile           *os.File
	interfaceCallback *list.Element[DefaultInterfaceUpdateCallback]
	options           Options
	ruleIndex6        []int
}

func Open(options Options) (Tun, error) {
	tunFd, err := open(options.Name)
	if err != nil {
		return nil, err
	}
	tunLink, err := netlink.LinkByName(options.Name)
	if err != nil {
		return nil, E.Errors(err, unix.Close(tunFd))
	}
	nativeTun := &NativeTun{
		tunFd:   tunFd,
		tunFile: os.NewFile(uintptr(tunFd), "tun"),
		options: options,
	}
	runtime.SetFinalizer(nativeTun.tunFile, nil)
	err = nativeTun.configure(tunLink)
	if err != nil {
		return nil, E.Errors(err, unix.Close(tunFd))
	}
	return nativeTun, nil
}

func (t *NativeTun) Read(p []byte) (n int, err error) {
	return t.tunFile.Read(p)
}

func (t *NativeTun) Write(p []byte) (n int, err error) {
	return t.tunFile.Write(p)
}

var controlPath string

func init() {
	const defaultTunPath = "/dev/net/tun"
	const androidTunPath = "/dev/tun"
	if rw.FileExists(androidTunPath) {
		controlPath = androidTunPath
	} else {
		controlPath = defaultTunPath
	}
}

func open(name string) (int, error) {
	fd, err := unix.Open(controlPath, unix.O_RDWR, 0)
	if err != nil {
		return -1, err
	}

	var ifr struct {
		name  [16]byte
		flags uint16
		_     [22]byte
	}

	copy(ifr.name[:], name)
	ifr.flags = unix.IFF_TUN | unix.IFF_NO_PI
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.TUNSETIFF, uintptr(unsafe.Pointer(&ifr)))
	if errno != 0 {
		unix.Close(fd)
		return -1, errno
	}

	if err = unix.SetNonblock(fd, true); err != nil {
		unix.Close(fd)
		return -1, err
	}

	return fd, nil
}

func (t *NativeTun) configure(tunLink netlink.Link) error {
	err := netlink.LinkSetMTU(tunLink, int(t.options.MTU))
	if err == unix.EPERM {
		// unprivileged
		return nil
	} else if err != nil {
		return err
	}

	if len(t.options.Inet4Address) > 0 {
		for _, address := range t.options.Inet4Address {
			addr4, _ := netlink.ParseAddr(address.String())
			err = netlink.AddrAdd(tunLink, addr4)
			if err != nil {
				return err
			}
		}
	}
	if len(t.options.Inet6Address) > 0 {
		for _, address := range t.options.Inet6Address {
			addr6, _ := netlink.ParseAddr(address.String())
			err = netlink.AddrAdd(tunLink, addr6)
			if err != nil {
				return err
			}
		}
	}

	err = netlink.LinkSetUp(tunLink)
	if err != nil {
		return err
	}

	if t.options.TableIndex == 0 {
		for {
			t.options.TableIndex = int(rand.Uint32())
			routeList, fErr := netlink.RouteListFiltered(netlink.FAMILY_ALL, &netlink.Route{Table: t.options.TableIndex}, netlink.RT_FILTER_TABLE)
			if len(routeList) == 0 || fErr != nil {
				break
			}
		}
	}

	err = t.setRoute(tunLink)
	if err != nil {
		_ = t.unsetRoute0(tunLink)
		return err
	}

	err = t.unsetRules()
	if err != nil {
		return E.Cause(err, "cleanup rules")
	}
	err = t.setRules()
	if err != nil {
		_ = t.unsetRules()
		return err
	}

	if t.options.AutoRoute && runtime.GOOS == "android" {
		t.interfaceCallback = t.options.InterfaceMonitor.RegisterCallback(t.routeUpdate)
	}
	return nil
}

func (t *NativeTun) Close() error {
	if t.interfaceCallback != nil {
		t.options.InterfaceMonitor.UnregisterCallback(t.interfaceCallback)
	}
	return E.Errors(t.unsetRoute(), t.unsetRules(), common.Close(common.PtrOrNil(t.tunFile)))
}

func (t *NativeTun) routes(tunLink netlink.Link) []netlink.Route {
	var routes []netlink.Route
	if len(t.options.Inet4Address) > 0 {
		if t.options.AutoRoute {
			if len(t.options.Inet4RouteAddress) > 0 {
				for _, addr := range t.options.Inet4RouteAddress {
					routes = append(routes, netlink.Route{
						Dst: &net.IPNet{
							IP:   addr.Addr().AsSlice(),
							Mask: net.CIDRMask(addr.Bits(), 32),
						},
						LinkIndex: tunLink.Attrs().Index,
						Table:     t.options.TableIndex,
					})
				}
			} else {
				routes = append(routes, netlink.Route{
					Dst: &net.IPNet{
						IP:   net.IPv4zero,
						Mask: net.CIDRMask(0, 32),
					},
					LinkIndex: tunLink.Attrs().Index,
					Table:     t.options.TableIndex,
				})
			}
		}
	}
	if len(t.options.Inet6Address) > 0 {
		if len(t.options.Inet6RouteAddress) > 0 {
			for _, addr := range t.options.Inet6RouteAddress {
				routes = append(routes, netlink.Route{
					Dst: &net.IPNet{
						IP:   addr.Addr().AsSlice(),
						Mask: net.CIDRMask(addr.Bits(), 128),
					},
					LinkIndex: tunLink.Attrs().Index,
					Table:     t.options.TableIndex,
				})
			}
		} else {
			routes = append(routes, netlink.Route{
				Dst: &net.IPNet{
					IP:   net.IPv6zero,
					Mask: net.CIDRMask(0, 128),
				},
				LinkIndex: tunLink.Attrs().Index,
				Table:     t.options.TableIndex,
			})
		}
	}
	return routes
}

const (
	ruleStart = 9000
	ruleEnd   = ruleStart + 10
)

func (t *NativeTun) nextIndex6() int {
	ruleList, err := netlink.RuleList(netlink.FAMILY_V6)
	if err != nil {
		return -1
	}
	var minIndex int
	for _, rule := range ruleList {
		if rule.Priority > 0 && (minIndex == 0 || rule.Priority < minIndex) {
			minIndex = rule.Priority
		}
	}
	minIndex--
	t.ruleIndex6 = append(t.ruleIndex6, minIndex)
	return minIndex
}

func (t *NativeTun) rules() []*netlink.Rule {
	if !t.options.AutoRoute {
		if len(t.options.Inet6Address) > 0 {
			it := netlink.NewRule()
			it.Priority = t.nextIndex6()
			it.Table = t.options.TableIndex
			it.Family = unix.AF_INET6
			it.OifName = t.options.Name
			return []*netlink.Rule{it}
		}
		return nil
	}

	var p4, p6 bool
	var pRule int
	if len(t.options.Inet4Address) > 0 {
		p4 = true
		pRule += 1
	}
	if len(t.options.Inet6Address) > 0 {
		p6 = true
		pRule += 1
	}
	if pRule == 0 {
		return []*netlink.Rule{}
	}

	var rules []*netlink.Rule
	var it *netlink.Rule

	excludeRanges := t.options.ExcludedRanges()
	priority := ruleStart
	priority6 := priority
	nopPriority := ruleEnd

	for _, excludeRange := range excludeRanges {
		if p4 {
			it = netlink.NewRule()
			it.Priority = priority
			it.UIDRange = netlink.NewRuleUIDRange(excludeRange.Start, excludeRange.End)
			it.Goto = nopPriority
			it.Family = unix.AF_INET
			rules = append(rules, it)
		}
		if p6 {
			it = netlink.NewRule()
			it.Priority = priority6
			it.UIDRange = netlink.NewRuleUIDRange(excludeRange.Start, excludeRange.End)
			it.Goto = nopPriority
			it.Family = unix.AF_INET6
			rules = append(rules, it)
		}
	}
	if len(excludeRanges) > 0 {
		if p4 {
			priority++
		}
		if p6 {
			priority6++
		}
	}

	if runtime.GOOS == "android" && t.options.InterfaceMonitor.AndroidVPNEnabled() {
		const protectedFromVPN = 0x20000
		if p4 || t.options.StrictRoute {
			it = netlink.NewRule()
			if t.options.InterfaceMonitor.OverrideAndroidVPN() {
				it.Mark = protectedFromVPN
			}
			it.Mask = protectedFromVPN
			it.Priority = priority
			it.Family = unix.AF_INET
			it.Goto = nopPriority
			rules = append(rules, it)
			priority++
		}
		if p6 || t.options.StrictRoute {
			it = netlink.NewRule()
			if t.options.InterfaceMonitor.OverrideAndroidVPN() {
				it.Mark = protectedFromVPN
			}
			it.Mask = protectedFromVPN
			it.Family = unix.AF_INET6
			it.Priority = priority6
			it.Goto = nopPriority
			rules = append(rules, it)
			priority6++
		}
	}

	if t.options.StrictRoute {
		if !p4 {
			it = netlink.NewRule()
			it.Priority = priority
			it.Family = unix.AF_INET
			it.Type = unix.FR_ACT_UNREACHABLE
			rules = append(rules, it)
			priority++
		}
		if !p6 {
			it = netlink.NewRule()
			it.Priority = priority6
			it.Family = unix.AF_INET6
			it.Type = unix.FR_ACT_UNREACHABLE
			rules = append(rules, it)
			priority6++
		}
	}

	if runtime.GOOS != "android" {
		if p4 {
			for _, address := range t.options.Inet4Address {
				it = netlink.NewRule()
				it.Priority = priority
				it.Dst = address.Masked()
				it.Table = t.options.TableIndex
				it.Family = unix.AF_INET
				rules = append(rules, it)
			}
			priority++
		}
		/*if p6 {
			it = netlink.NewRule()
			it.Priority = priority
			it.Dst = t.options.Inet6Address.Masked()
			it.Table = tunTableIndex
			it.Family = unix.AF_INET6
			rules = append(rules, it)
		}*/
		if p4 {
			it = netlink.NewRule()
			it.Priority = priority
			it.IPProto = unix.IPPROTO_ICMP
			it.Goto = nopPriority
			it.Family = unix.AF_INET
			rules = append(rules, it)
			priority++
		}
		if p6 {
			it = netlink.NewRule()
			it.Priority = priority6
			it.IPProto = unix.IPPROTO_ICMPV6
			it.Goto = nopPriority
			it.Family = unix.AF_INET6
			rules = append(rules, it)
			priority6++
		}
		if p4 {
			it = netlink.NewRule()
			it.Priority = priority
			it.Invert = true
			it.Dport = netlink.NewRulePortRange(53, 53)
			it.Table = unix.RT_TABLE_MAIN
			it.SuppressPrefixlen = 0
			it.Family = unix.AF_INET
			rules = append(rules, it)
		}
		if p6 {
			it = netlink.NewRule()
			it.Priority = priority6
			it.Invert = true
			it.Dport = netlink.NewRulePortRange(53, 53)
			it.Table = unix.RT_TABLE_MAIN
			it.SuppressPrefixlen = 0
			it.Family = unix.AF_INET6
			rules = append(rules, it)
		}
	}

	if p4 {
		if t.options.StrictRoute {
			it = netlink.NewRule()
			it.Priority = priority
			it.Table = t.options.TableIndex
			it.Family = unix.AF_INET
			rules = append(rules, it)
		} else {
			it = netlink.NewRule()
			it.Priority = priority
			it.Invert = true
			it.IifName = "lo"
			it.Table = t.options.TableIndex
			it.Family = unix.AF_INET
			rules = append(rules, it)

			it = netlink.NewRule()
			it.Priority = priority
			it.IifName = "lo"
			it.Src = netip.PrefixFrom(netip.IPv4Unspecified(), 32)
			it.Table = t.options.TableIndex
			it.Family = unix.AF_INET
			rules = append(rules, it)

			for _, address := range t.options.Inet4Address {
				it = netlink.NewRule()
				it.Priority = priority
				it.IifName = "lo"
				it.Src = address.Masked()
				it.Table = t.options.TableIndex
				it.Family = unix.AF_INET
				rules = append(rules, it)
			}
		}
		priority++
	}
	if p6 {
		// FIXME: this match connections from public address
		it = netlink.NewRule()
		it.Priority = priority6
		it.Table = t.options.TableIndex
		it.Family = unix.AF_INET6
		rules = append(rules, it)

		/*it = netlink.NewRule()
		it.Priority = priority
		it.Invert = true
		it.IifName = "lo"
		it.Table = tunTableIndex
		it.Family = unix.AF_INET6
		rules = append(rules, it)

		it = netlink.NewRule()
		it.Priority = priority
		it.IifName = "lo"
		it.Src = netip.PrefixFrom(netip.IPv6Unspecified(), 128) // not working
		it.Table = tunTableIndex
		it.Family = unix.AF_INET6
		rules = append(rules, it)

		it = netlink.NewRule()
		it.Priority = priority
		it.IifName = "lo"
		it.Src = t.options.Inet6Address.Masked()
		it.Table = tunTableIndex
		it.Family = unix.AF_INET6
		rules = append(rules, it)*/
		priority6++
	}
	if p4 {
		it = netlink.NewRule()
		it.Priority = nopPriority
		it.Family = unix.AF_INET
		rules = append(rules, it)
	}
	if p6 {
		it = netlink.NewRule()
		it.Priority = nopPriority
		it.Family = unix.AF_INET6
		rules = append(rules, it)
	}
	return rules
}

func (t *NativeTun) setRoute(tunLink netlink.Link) error {
	for i, route := range t.routes(tunLink) {
		err := netlink.RouteAdd(&route)
		if err != nil {
			return E.Cause(err, "add route ", i)
		}
	}
	return nil
}

func (t *NativeTun) setRules() error {
	for i, rule := range t.rules() {
		err := netlink.RuleAdd(rule)
		if err != nil {
			return E.Cause(err, "add rule ", i, "/", len(t.rules()))
		}
	}
	return nil
}

func (t *NativeTun) unsetRoute() error {
	tunLink, err := netlink.LinkByName(t.options.Name)
	if err != nil {
		return err
	}
	return t.unsetRoute0(tunLink)
}

func (t *NativeTun) unsetRoute0(tunLink netlink.Link) error {
	for _, route := range t.routes(tunLink) {
		_ = netlink.RouteDel(&route)
	}
	return nil
}

func (t *NativeTun) unsetRules() error {
	if len(t.ruleIndex6) > 0 {
		for _, index := range t.ruleIndex6 {
			ruleToDel := netlink.NewRule()
			ruleToDel.Family = unix.AF_INET6
			ruleToDel.Priority = index
			err := netlink.RuleDel(ruleToDel)
			if err != nil {
				return E.Cause(err, "unset rule6 ", index)
			}
		}
		t.ruleIndex6 = nil
	}
	if t.options.AutoRoute {
		ruleList, err := netlink.RuleList(netlink.FAMILY_ALL)
		if err != nil {
			return err
		}
		for _, rule := range ruleList {
			if rule.Priority >= ruleStart && rule.Priority <= ruleEnd {
				ruleToDel := netlink.NewRule()
				ruleToDel.Family = rule.Family
				ruleToDel.Priority = rule.Priority
				err = netlink.RuleDel(ruleToDel)
				if err != nil {
					return E.Cause(err, "unset rule ", rule.Priority, " for ", rule.Family)
				}
			}
		}
	}
	return nil
}

func (t *NativeTun) resetRules() error {
	t.unsetRules()
	return t.setRules()
}

func (t *NativeTun) routeUpdate(event int) error {
	if event&EventAndroidVPNUpdate == 0 {
		return nil
	}
	err := t.resetRules()
	if err != nil {
		return E.Cause(err, "reset route")
	}
	return nil
}
