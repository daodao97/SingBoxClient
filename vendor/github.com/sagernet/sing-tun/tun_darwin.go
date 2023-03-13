package tun

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/route"
	"golang.org/x/sys/unix"
)

const PacketOffset = 4

type NativeTun struct {
	tunFile      *os.File
	tunWriter    N.VectorisedWriter
	mtu          uint32
	inet4Address string
	inet6Address string
}

func Open(options Options) (Tun, error) {
	ifIndex := -1
	_, err := fmt.Sscanf(options.Name, "utun%d", &ifIndex)
	if err != nil {
		return nil, E.New("bad tun name: ", options.Name)
	}

	tunFd, err := unix.Socket(unix.AF_SYSTEM, unix.SOCK_DGRAM, 2)
	if err != nil {
		return nil, err
	}

	err = configure(tunFd, ifIndex, options.Name, options)
	if err != nil {
		unix.Close(tunFd)
		return nil, err
	}
	nativeTun := &NativeTun{
		tunFile: os.NewFile(uintptr(tunFd), "utun"),
		mtu:     options.MTU,
	}
	if len(options.Inet4Address) > 0 {
		nativeTun.inet4Address = string(options.Inet4Address[0].Addr().AsSlice())
	}
	if len(options.Inet6Address) > 0 {
		nativeTun.inet6Address = string(options.Inet6Address[0].Addr().AsSlice())
	}
	var ok bool
	nativeTun.tunWriter, ok = bufio.CreateVectorisedWriter(nativeTun.tunFile)
	if !ok {
		panic("create vectorised writer")
	}
	runtime.SetFinalizer(nativeTun.tunFile, nil)
	return nativeTun, nil
}

func (t *NativeTun) Read(p []byte) (n int, err error) {
	/*n, err = t.tunFile.Read(p)
	if n < 4 {
		return 0, err
	}

	copy(p[:], p[4:])
	return n - 4, err*/
	return t.tunFile.Read(p)
}

var (
	packetHeader4 = [4]byte{0x00, 0x00, 0x00, unix.AF_INET}
	packetHeader6 = [4]byte{0x00, 0x00, 0x00, unix.AF_INET6}
)

func (t *NativeTun) Write(p []byte) (n int, err error) {
	var packetHeader []byte
	if p[0]>>4 == 4 {
		packetHeader = packetHeader4[:]
	} else {
		packetHeader = packetHeader6[:]
	}
	_, err = bufio.WriteVectorised(t.tunWriter, [][]byte{packetHeader, p})
	if err == nil {
		n = len(p)
	}
	return
}

func (t *NativeTun) Close() error {
	return t.tunFile.Close()
}

const utunControlName = "com.apple.net.utun_control"

const (
	SIOCAIFADDR_IN6       = 2155899162 // netinet6/in6_var.h
	IN6_IFF_NODAD         = 0x0020     // netinet6/in6_var.h
	IN6_IFF_SECURED       = 0x0400     // netinet6/in6_var.h
	ND6_INFINITE_LIFETIME = 0xFFFFFFFF // netinet6/nd6.h
)

type ifAliasReq struct {
	Name    [unix.IFNAMSIZ]byte
	Addr    unix.RawSockaddrInet4
	Dstaddr unix.RawSockaddrInet4
	Mask    unix.RawSockaddrInet4
}

type ifAliasReq6 struct {
	Name     [16]byte
	Addr     unix.RawSockaddrInet6
	Dstaddr  unix.RawSockaddrInet6
	Mask     unix.RawSockaddrInet6
	Flags    uint32
	Lifetime addrLifetime6
}

type addrLifetime6 struct {
	Expire    float64
	Preferred float64
	Vltime    uint32
	Pltime    uint32
}

func configure(tunFd int, ifIndex int, name string, options Options) error {
	ctlInfo := &unix.CtlInfo{}
	copy(ctlInfo.Name[:], utunControlName)
	err := unix.IoctlCtlInfo(tunFd, ctlInfo)
	if err != nil {
		return os.NewSyscallError("IoctlCtlInfo", err)
	}

	err = unix.Connect(tunFd, &unix.SockaddrCtl{
		ID:   ctlInfo.Id,
		Unit: uint32(ifIndex) + 1,
	})
	if err != nil {
		return os.NewSyscallError("Connect", err)
	}

	err = unix.SetNonblock(tunFd, true)
	if err != nil {
		return os.NewSyscallError("SetNonblock", err)
	}

	err = useSocket(unix.AF_INET, unix.SOCK_DGRAM, 0, func(socketFd int) error {
		var ifr unix.IfreqMTU
		copy(ifr.Name[:], name)
		ifr.MTU = int32(options.MTU)
		return unix.IoctlSetIfreqMTU(socketFd, &ifr)
	})
	if err != nil {
		return os.NewSyscallError("IoctlSetIfreqMTU", err)
	}
	if len(options.Inet4Address) > 0 {
		for _, address := range options.Inet4Address {
			ifReq := ifAliasReq{
				Addr: unix.RawSockaddrInet4{
					Len:    unix.SizeofSockaddrInet4,
					Family: unix.AF_INET,
					Addr:   address.Addr().As4(),
				},
				Dstaddr: unix.RawSockaddrInet4{
					Len:    unix.SizeofSockaddrInet4,
					Family: unix.AF_INET,
					Addr:   address.Addr().As4(),
				},
				Mask: unix.RawSockaddrInet4{
					Len:    unix.SizeofSockaddrInet4,
					Family: unix.AF_INET,
					Addr:   netip.MustParseAddr(net.IP(net.CIDRMask(address.Bits(), 32)).String()).As4(),
				},
			}
			copy(ifReq.Name[:], name)
			err = useSocket(unix.AF_INET, unix.SOCK_DGRAM, 0, func(socketFd int) error {
				if _, _, errno := unix.Syscall(
					syscall.SYS_IOCTL,
					uintptr(socketFd),
					uintptr(unix.SIOCAIFADDR),
					uintptr(unsafe.Pointer(&ifReq)),
				); errno != 0 {
					return os.NewSyscallError("SIOCAIFADDR", errno)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	if len(options.Inet6Address) > 0 {
		for _, address := range options.Inet6Address {
			ifReq6 := ifAliasReq6{
				Addr: unix.RawSockaddrInet6{
					Len:    unix.SizeofSockaddrInet6,
					Family: unix.AF_INET6,
					Addr:   address.Addr().As16(),
				},
				Mask: unix.RawSockaddrInet6{
					Len:    unix.SizeofSockaddrInet6,
					Family: unix.AF_INET6,
					Addr:   netip.MustParseAddr(net.IP(net.CIDRMask(address.Bits(), 128)).String()).As16(),
				},
				Flags: IN6_IFF_NODAD | IN6_IFF_SECURED,
				Lifetime: addrLifetime6{
					Vltime: ND6_INFINITE_LIFETIME,
					Pltime: ND6_INFINITE_LIFETIME,
				},
			}
			if address.Bits() == 128 {
				ifReq6.Dstaddr = unix.RawSockaddrInet6{
					Len:    unix.SizeofSockaddrInet6,
					Family: unix.AF_INET6,
					Addr:   address.Addr().Next().As16(),
				}
			}
			copy(ifReq6.Name[:], name)
			err = useSocket(unix.AF_INET6, unix.SOCK_DGRAM, 0, func(socketFd int) error {
				if _, _, errno := unix.Syscall(
					syscall.SYS_IOCTL,
					uintptr(socketFd),
					uintptr(SIOCAIFADDR_IN6),
					uintptr(unsafe.Pointer(&ifReq6)),
				); errno != 0 {
					return os.NewSyscallError("SIOCAIFADDR_IN6", errno)
				}
				return nil
			})
			if err != nil {
				return err
			}
		}
	}
	if options.AutoRoute {
		if len(options.Inet4Address) > 0 {
			var routes []netip.Prefix
			if len(options.Inet4RouteAddress) > 0 {
				routes = append(options.Inet4RouteAddress, netip.PrefixFrom(options.Inet4Address[0].Addr().Next(), 32))
			} else {
				routes = []netip.Prefix{
					netip.PrefixFrom(netip.AddrFrom4([4]byte{1, 0, 0, 0}), 8),
					netip.PrefixFrom(netip.AddrFrom4([4]byte{2, 0, 0, 0}), 7),
					netip.PrefixFrom(netip.AddrFrom4([4]byte{4, 0, 0, 0}), 6),
					netip.PrefixFrom(netip.AddrFrom4([4]byte{8, 0, 0, 0}), 5),
					netip.PrefixFrom(netip.AddrFrom4([4]byte{16, 0, 0, 0}), 4),
					netip.PrefixFrom(netip.AddrFrom4([4]byte{32, 0, 0, 0}), 3),
					netip.PrefixFrom(netip.AddrFrom4([4]byte{64, 0, 0, 0}), 2),
					netip.PrefixFrom(netip.AddrFrom4([4]byte{128, 0, 0, 0}), 1),
				}
			}
			for _, subnet := range routes {
				err = addRoute(subnet, options.Inet4Address[0].Addr())
				if err != nil {
					return E.Cause(err, "add ipv4 route ", subnet)
				}
			}
		}
		if len(options.Inet6Address) > 0 {
			var routes []netip.Prefix
			if len(options.Inet6RouteAddress) > 0 {
				routes = append(options.Inet6RouteAddress, netip.PrefixFrom(options.Inet6Address[0].Addr().Next(), 128))
			} else {
				routes = []netip.Prefix{
					netip.PrefixFrom(netip.AddrFrom16([16]byte{32, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}), 3),
				}
			}
			for _, subnet := range routes {
				err = addRoute(subnet, options.Inet6Address[0].Addr())
				if err != nil {
					return E.Cause(err, "add ipv6 route ", subnet)
				}
			}
		}
	}
	return nil
}

func useSocket(domain, typ, proto int, block func(socketFd int) error) error {
	socketFd, err := unix.Socket(domain, typ, proto)
	if err != nil {
		return err
	}
	defer unix.Close(socketFd)
	return block(socketFd)
}

func addRoute(destination netip.Prefix, gateway netip.Addr) error {
	routeMessage := route.RouteMessage{
		Type:    unix.RTM_ADD,
		Flags:   unix.RTF_UP | unix.RTF_STATIC | unix.RTF_GATEWAY,
		Version: unix.RTM_VERSION,
		Seq:     1,
	}
	if gateway.Is4() {
		routeMessage.Addrs = []route.Addr{
			syscall.RTAX_DST:     &route.Inet4Addr{IP: destination.Addr().As4()},
			syscall.RTAX_NETMASK: &route.Inet4Addr{IP: netip.MustParseAddr(net.IP(net.CIDRMask(destination.Bits(), 32)).String()).As4()},
			syscall.RTAX_GATEWAY: &route.Inet4Addr{IP: gateway.As4()},
		}
	} else {
		routeMessage.Addrs = []route.Addr{
			syscall.RTAX_DST:     &route.Inet6Addr{IP: destination.Addr().As16()},
			syscall.RTAX_NETMASK: &route.Inet6Addr{IP: netip.MustParseAddr(net.IP(net.CIDRMask(destination.Bits(), 128)).String()).As16()},
			syscall.RTAX_GATEWAY: &route.Inet6Addr{IP: gateway.As16()},
		}
	}
	request, err := routeMessage.Marshal()
	if err != nil {
		return err
	}
	return useSocket(unix.AF_ROUTE, unix.SOCK_RAW, 0, func(socketFd int) error {
		return common.Error(unix.Write(socketFd, request))
	})
}
