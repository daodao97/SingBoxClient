//lint:file-ignore U1000 linkname magic brings a lot of unused unexported fields.

package tfo

import (
	"context"
	"net"
	"os"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const TCP_FASTOPEN = 15

func SetTFOListener(fd uintptr) error {
	return setTFO(windows.Handle(fd))
}

func SetTFODialer(fd uintptr) error {
	return setTFO(windows.Handle(fd))
}

func setTFO(fd windows.Handle) error {
	return windows.SetsockoptInt(fd, windows.IPPROTO_TCP, TCP_FASTOPEN, 1)
}

func setIPv6Only(fd windows.Handle, family int, ipv6only bool) error {
	if family == windows.AF_INET6 {
		// Allow both IP versions even if the OS default
		// is otherwise. Note that some operating systems
		// never admit this option.
		return windows.SetsockoptInt(fd, windows.IPPROTO_IPV6, windows.IPV6_V6ONLY, boolint(ipv6only))
	}
	return nil
}

func setNoDelay(fd windows.Handle, noDelay int) error {
	return windows.SetsockoptInt(fd, windows.IPPROTO_TCP, windows.TCP_NODELAY, noDelay)
}

func setUpdateConnectContext(fd windows.Handle) error {
	return windows.Setsockopt(fd, windows.SOL_SOCKET, windows.SO_UPDATE_CONNECT_CONTEXT, nil, 0)
}

//go:linkname sockaddrToTCP net.sockaddrToTCP
func sockaddrToTCP(sa syscall.Sockaddr) net.Addr

//go:linkname runtime_pollServerInit internal/poll.runtime_pollServerInit
func runtime_pollServerInit()

//go:linkname runtime_pollOpen internal/poll.runtime_pollOpen
func runtime_pollOpen(fd uintptr) (uintptr, int)

// Copied from src/internal/pool/fd_poll_runtime.go
var serverInit sync.Once

// operation contains superset of data necessary to perform all async IO.
//
// Copied from src/internal/pool/fd_windows.go
type operation struct {
	// Used by IOCP interface, it must be first field
	// of the struct, as our code rely on it.
	o syscall.Overlapped

	// fields used by runtime.netpoll
	runtimeCtx uintptr
	mode       int32
	errno      int32
	qty        uint32

	// fields used only by net package
	fd     *pFD
	buf    syscall.WSABuf
	msg    windows.WSAMsg
	sa     syscall.Sockaddr
	rsa    *syscall.RawSockaddrAny
	rsan   int32
	handle syscall.Handle
	flags  uint32
	bufs   []syscall.WSABuf
}

//go:linkname execIO internal/poll.execIO
func execIO(o *operation, submit func(o *operation) error) (int, error)

// pFD is a file descriptor. The net and os packages embed this type in
// a larger type representing a network connection or OS file.
//
// Copied from src/internal/pool/fd_windows.go
type pFD struct {
	fdmuS uint64
	fdmuR uint32
	fdmuW uint32

	// System file descriptor. Immutable until Close.
	Sysfd syscall.Handle

	// Read operation.
	rop operation
	// Write operation.
	wop operation

	// I/O poller.
	pd uintptr

	// Used to implement pread/pwrite.
	l sync.Mutex

	// For console I/O.
	lastbits       []byte   // first few bytes of the last incomplete rune in last write
	readuint16     []uint16 // buffer to hold uint16s obtained with ReadConsole
	readbyte       []byte   // buffer to hold decoding of readuint16 from utf16 to utf8
	readbyteOffset int      // readbyte[readOffset:] is yet to be consumed with file.Read

	// Semaphore signaled when file is closed.
	csema uint32

	skipSyncNotif bool

	// Whether this is a streaming descriptor, as opposed to a
	// packet-based descriptor like a UDP socket.
	IsStream bool

	// Whether a zero byte read indicates EOF. This is false for a
	// message based socket connection.
	ZeroReadIsEOF bool

	// Whether this is a file rather than a network socket.
	isFile bool

	// The kind of this file.
	kind byte
}

func (fd *pFD) init() error {
	serverInit.Do(runtime_pollServerInit)
	ctx, errno := runtime_pollOpen(uintptr(fd.Sysfd))
	if errno != 0 {
		return syscall.Errno(errno)
	}
	fd.pd = ctx
	fd.rop.mode = 'r'
	fd.wop.mode = 'w'
	fd.rop.fd = fd
	fd.wop.fd = fd
	fd.rop.runtimeCtx = fd.pd
	fd.wop.runtimeCtx = fd.pd
	return nil
}

func (fd *pFD) ConnectEx(ra syscall.Sockaddr, b []byte) (n int, err error) {
	fd.wop.sa = ra
	n, err = execIO(&fd.wop, func(o *operation) error {
		return syscall.ConnectEx(o.fd.Sysfd, o.sa, &b[0], uint32(len(b)), &o.qty, &o.o)
	})
	return
}

// Network file descriptor.
//
// Copied from src/net/fd_posix.go
type netFD struct {
	pfd pFD

	// immutable until Close
	family      int
	sotype      int
	isConnected bool // handshake completed or use of association with peer
	net         string
	laddr       net.Addr
	raddr       net.Addr
}

func (fd *netFD) ctrlNetwork() string {
	if fd.net == "tcp4" || fd.family == windows.AF_INET {
		return "tcp4"
	}
	return "tcp6"
}

//go:linkname newFD net.newFD
func newFD(sysfd syscall.Handle, family, sotype int, net string) (*netFD, error)

type rawConn netFD

func (c *rawConn) Control(f func(uintptr)) error {
	f(uintptr(c.pfd.Sysfd))
	return nil
}

func (c *rawConn) Read(f func(uintptr) bool) error {
	f(uintptr(c.pfd.Sysfd))
	return syscall.EWINDOWS
}

func (c *rawConn) Write(f func(uintptr) bool) error {
	f(uintptr(c.pfd.Sysfd))
	return syscall.EWINDOWS
}

func dialTFO(ctx context.Context, network string, laddr, raddr *net.TCPAddr, b []byte, ctrlCtxFn func(context.Context, string, string, syscall.RawConn) error) (*net.TCPConn, error) {
	ltsa := (*tcpSockaddr)(laddr)
	rtsa := (*tcpSockaddr)(raddr)
	family, ipv6only := favoriteAddrFamily(network, ltsa, rtsa, "dial")

	var (
		ip   net.IP
		port int
		zone string
	)

	if laddr != nil {
		ip = laddr.IP
		port = laddr.Port
		zone = laddr.Zone
	}

	lsa, err := ipToSockaddr(family, ip, port, zone)
	if err != nil {
		return nil, err
	}

	rsa, err := rtsa.sockaddr(family)
	if err != nil {
		return nil, err
	}

	handle, err := windows.WSASocket(int32(family), windows.SOCK_STREAM, windows.IPPROTO_TCP, nil, 0, windows.WSA_FLAG_OVERLAPPED|windows.WSA_FLAG_NO_HANDLE_INHERIT)
	if err != nil {
		return nil, os.NewSyscallError("WSASocket", err)
	}

	fd, err := newFD(syscall.Handle(handle), family, windows.SOCK_STREAM, network)
	if err != nil {
		windows.Closesocket(handle)
		return nil, err
	}

	tcpConn := (*net.TCPConn)(unsafe.Pointer(&fd))

	if err := setIPv6Only(handle, family, ipv6only); err != nil {
		tcpConn.Close()
		return nil, wrapSyscallError("setsockopt", err)
	}

	if err := setNoDelay(handle, 1); err != nil {
		tcpConn.Close()
		return nil, wrapSyscallError("setsockopt", err)
	}

	if err := setTFO(handle); err != nil {
		tcpConn.Close()
		return nil, wrapSyscallError("setsockopt", err)
	}

	if ctrlCtxFn != nil {
		if err := ctrlCtxFn(ctx, fd.ctrlNetwork(), raddr.String(), (*rawConn)(fd)); err != nil {
			tcpConn.Close()
			return nil, err
		}
	}

	if err := syscall.Bind(syscall.Handle(handle), lsa); err != nil {
		tcpConn.Close()
		return nil, wrapSyscallError("bind", err)
	}

	if err := fd.pfd.init(); err != nil {
		tcpConn.Close()
		return nil, err
	}

	n, err := fd.pfd.ConnectEx(rsa, b)
	if err != nil {
		tcpConn.Close()
		return nil, os.NewSyscallError("connectex", err)
	}

	if err := setUpdateConnectContext(handle); err != nil {
		tcpConn.Close()
		return nil, wrapSyscallError("setsockopt", err)
	}

	lsa, err = syscall.Getsockname(syscall.Handle(handle))
	if err != nil {
		tcpConn.Close()
		return nil, wrapSyscallError("getsockname", err)
	}
	fd.laddr = sockaddrToTCP(lsa)

	rsa, err = syscall.Getpeername(syscall.Handle(handle))
	if err != nil {
		tcpConn.Close()
		return nil, wrapSyscallError("getpeername", err)
	}
	fd.raddr = sockaddrToTCP(rsa)

	if n < len(b) {
		if _, err := tcpConn.Write(b[n:]); err != nil {
			tcpConn.Close()
			return nil, err
		}
	}

	return tcpConn, nil
}
