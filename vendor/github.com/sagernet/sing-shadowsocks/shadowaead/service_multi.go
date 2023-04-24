package shadowaead

import (
	"context"
	"crypto/cipher"
	"io"
	"net"
	"net/netip"

	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio/deadline"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/common/udpnat"
)

var _ shadowsocks.MultiService[int] = (*MultiService[int])(nil)

type MultiService[U comparable] struct {
	name      string
	methodMap map[U]*Method
	handler   shadowsocks.Handler
	udpNat    *udpnat.Service[netip.AddrPort]
}

func NewMultiService[U comparable](method string, udpTimeout int64, handler shadowsocks.Handler) (*MultiService[U], error) {
	s := &MultiService[U]{
		name:    method,
		handler: handler,
		udpNat:  udpnat.New[netip.AddrPort](udpTimeout, handler),
	}
	return s, nil
}

func (s *MultiService[U]) Name() string {
	return s.name
}

func (s *MultiService[U]) UpdateUsers(userList []U, keyList [][]byte) error {
	s.methodMap = make(map[U]*Method)
	for i, user := range userList {
		key := keyList[i]
		method, err := New(s.name, key, "")
		if err != nil {
			return err
		}
		s.methodMap[user] = method
	}
	return nil
}

func (s *MultiService[U]) UpdateUsersWithPasswords(userList []U, passwordList []string) error {
	s.methodMap = make(map[U]*Method)
	for i, user := range userList {
		password := passwordList[i]
		method, err := New(s.name, nil, password)
		if err != nil {
			return err
		}
		s.methodMap[user] = method
	}
	return nil
}

func (s *MultiService[U]) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	err := s.newConnection(ctx, conn, metadata)
	if err != nil {
		err = &shadowsocks.ServerConnError{Conn: conn, Source: metadata.Source, Cause: err}
	}
	return err
}

func (s *MultiService[U]) newConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	var user U
	var method *Method
	for u, m := range s.methodMap {
		user, method = u, m
		break
	}
	if method == nil {
		return shadowsocks.ErrNoUsers
	}
	_header := buf.StackNewSize(method.keySaltLength + PacketLengthBufferSize + Overhead)
	defer common.KeepAlive(_header)
	header := common.Dup(_header)
	defer header.Release()

	_, err := header.ReadFullFrom(conn, header.FreeLen())
	if err != nil {
		return E.Cause(err, "read header")
	} else if !header.IsFull() {
		return ErrBadHeader
	}

	var reader *Reader
	var readCipher cipher.AEAD
	for u, m := range s.methodMap {
		_key := buf.StackNewSize(method.keySaltLength)
		key := common.Dup(_key)
		Kdf(m.key, header.To(m.keySaltLength), key)
		readCipher, err = m.constructor(key.Bytes())
		key.Release()
		common.KeepAlive(_key)
		if err != nil {
			return err
		}
		reader = NewReader(conn, readCipher, MaxPacketSize)

		err = reader.ReadWithLengthChunk(header.From(method.keySaltLength))
		if err != nil {
			continue
		}

		user, method = u, m
		break
	}
	if err != nil {
		return err
	}

	destination, err := M.SocksaddrSerializer.ReadAddrPort(reader)
	if err != nil {
		return err
	}

	metadata.Protocol = "shadowsocks"
	metadata.Destination = destination

	return s.handler.NewConnection(auth.ContextWithUser(ctx, user), deadline.NewConn(&serverConn{
		Method: method,
		Conn:   conn,
		reader: reader,
	}), metadata)
}

func (s *MultiService[U]) WriteIsThreadUnsafe() {
}

func (s *MultiService[U]) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata M.Metadata) error {
	err := s.newPacket(ctx, conn, buffer, metadata)
	if err != nil {
		err = &shadowsocks.ServerPacketError{Source: metadata.Source, Cause: err}
	}
	return err
}

func (s *MultiService[U]) newPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata M.Metadata) error {
	var user U
	var method *Method
	for u, m := range s.methodMap {
		user, method = u, m
		break
	}
	if method == nil {
		return shadowsocks.ErrNoUsers
	}
	if buffer.Len() < method.keySaltLength {
		return io.ErrShortBuffer
	}
	var readCipher cipher.AEAD
	var err error
	for u, m := range s.methodMap {
		_key := buf.StackNewSize(m.keySaltLength)
		key := common.Dup(_key)
		Kdf(m.key, buffer.To(m.keySaltLength), key)
		readCipher, err = m.constructor(key.Bytes())
		key.Release()
		common.KeepAlive(_key)
		if err != nil {
			return err
		}
		var packet []byte
		packet, err = readCipher.Open(buffer.Index(m.keySaltLength), rw.ZeroBytes[:readCipher.NonceSize()], buffer.From(m.keySaltLength), nil)
		if err != nil {
			continue
		}

		buffer.Advance(m.keySaltLength)
		buffer.Truncate(len(packet))

		user, method = u, m
		break
	}
	if err != nil {
		return err
	}

	destination, err := M.SocksaddrSerializer.ReadAddrPort(buffer)
	if err != nil {
		return err
	}

	metadata.Protocol = "shadowsocks"
	metadata.Destination = destination
	s.udpNat.NewPacket(auth.ContextWithUser(ctx, user), metadata.Source.AddrPort(), buffer, metadata, func(natConn N.PacketConn) N.PacketWriter {
		return &serverPacketWriter{method, conn, natConn}
	})
	return nil
}

func (s *MultiService[U]) NewError(ctx context.Context, err error) {
	s.handler.NewError(ctx, err)
}
