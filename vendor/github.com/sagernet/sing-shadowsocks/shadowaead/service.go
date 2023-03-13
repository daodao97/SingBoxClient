package shadowaead

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"io"
	"net"
	"net/netip"
	"sync"

	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/common/udpnat"

	"golang.org/x/crypto/chacha20poly1305"
)

var ErrBadHeader = E.New("bad header")

var _ shadowsocks.Service = (*Service)(nil)

type Service struct {
	name          string
	keySaltLength int
	constructor   func(key []byte) (cipher.AEAD, error)
	key           []byte
	password      string
	handler       shadowsocks.Handler
	udpNat        *udpnat.Service[netip.AddrPort]
}

func NewService(method string, key []byte, password string, udpTimeout int64, handler shadowsocks.Handler) (*Service, error) {
	s := &Service{
		name:    method,
		handler: handler,
		udpNat:  udpnat.New[netip.AddrPort](udpTimeout, handler),
	}
	switch method {
	case "aes-128-gcm":
		s.keySaltLength = 16
		s.constructor = aeadCipher(aes.NewCipher, cipher.NewGCM)
	case "aes-192-gcm":
		s.keySaltLength = 24
		s.constructor = aeadCipher(aes.NewCipher, cipher.NewGCM)
	case "aes-256-gcm":
		s.keySaltLength = 32
		s.constructor = aeadCipher(aes.NewCipher, cipher.NewGCM)
	case "chacha20-ietf-poly1305":
		s.keySaltLength = 32
		s.constructor = chacha20poly1305.New
	case "xchacha20-ietf-poly1305":
		s.keySaltLength = 32
		s.constructor = chacha20poly1305.NewX
	}
	if len(key) == s.keySaltLength {
		s.key = key
	} else if len(key) > 0 {
		return nil, shadowsocks.ErrBadKey
	} else if password != "" {
		s.key = shadowsocks.Key([]byte(password), s.keySaltLength)
	} else {
		return nil, shadowsocks.ErrMissingPassword
	}
	return s, nil
}

func (s *Service) Name() string {
	return s.name
}

func (s *Service) Password() string {
	return s.password
}

func (s *Service) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	err := s.newConnection(ctx, conn, metadata)
	if err != nil {
		err = &shadowsocks.ServerConnError{Conn: conn, Source: metadata.Source, Cause: err}
	}
	return err
}

func (s *Service) newConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	_header := buf.StackNewSize(s.keySaltLength + PacketLengthBufferSize + Overhead)
	defer common.KeepAlive(_header)
	header := common.Dup(_header)
	defer header.Release()

	_, err := header.ReadOnceFrom(conn)
	if err != nil {
		return E.Cause(err, "read header")
	} else if !header.IsFull() {
		return ErrBadHeader
	}

	_key := buf.StackNewSize(s.keySaltLength)
	key := common.Dup(_key)
	Kdf(s.key, header.To(s.keySaltLength), key)
	readCipher, err := s.constructor(key.Bytes())
	key.Release()
	common.KeepAlive(_key)
	if err != nil {
		return err
	}
	reader := NewReader(conn, readCipher, MaxPacketSize)

	err = reader.ReadWithLengthChunk(header.From(s.keySaltLength))
	if err != nil {
		return err
	}

	destination, err := M.SocksaddrSerializer.ReadAddrPort(reader)
	if err != nil {
		return err
	}

	metadata.Protocol = "shadowsocks"
	metadata.Destination = destination

	return s.handler.NewConnection(ctx, &serverConn{
		Service: s,
		Conn:    conn,
		reader:  reader,
	}, metadata)
}

func (s *Service) NewError(ctx context.Context, err error) {
	s.handler.NewError(ctx, err)
}

type serverConn struct {
	*Service
	net.Conn
	access sync.Mutex
	reader *Reader
	writer *Writer
}

func (c *serverConn) writeResponse(payload []byte) (n int, err error) {
	_salt := buf.StackNewSize(c.keySaltLength)
	salt := common.Dup(_salt)
	salt.WriteRandom(c.keySaltLength)

	_key := buf.StackNewSize(c.keySaltLength)
	key := common.Dup(_key)

	Kdf(c.key, salt.Bytes(), key)
	writeCipher, err := c.constructor(key.Bytes())
	key.Release()
	common.KeepAlive(_key)
	if err != nil {
		salt.Release()
		common.KeepAlive(_salt)
		return
	}
	writer := NewWriter(c.Conn, writeCipher, MaxPacketSize)

	header := writer.Buffer()
	common.Must1(header.Write(salt.Bytes()))
	salt.Release()
	common.KeepAlive(_salt)

	bufferedWriter := writer.BufferedWriter(header.Len())
	if len(payload) > 0 {
		n, err = bufferedWriter.Write(payload)
		if err != nil {
			return
		}
	}

	err = bufferedWriter.Flush()
	if err != nil {
		return
	}

	c.writer = writer
	return
}

func (c *serverConn) Read(b []byte) (n int, err error) {
	return c.reader.Read(b)
}

func (c *serverConn) Write(p []byte) (n int, err error) {
	if c.writer != nil {
		return c.writer.Write(p)
	}
	c.access.Lock()
	if c.writer != nil {
		c.access.Unlock()
		return c.writer.Write(p)
	}
	defer c.access.Unlock()
	return c.writeResponse(p)
}

func (c *serverConn) ReadFrom(r io.Reader) (n int64, err error) {
	if c.writer == nil {
		return bufio.ReadFrom0(c, r)
	}
	return c.writer.ReadFrom(r)
}

func (c *serverConn) WriteTo(w io.Writer) (n int64, err error) {
	return c.reader.WriteTo(w)
}

func (c *serverConn) Upstream() any {
	return c.Conn
}

func (s *Service) ReaderMTU() int {
	return MaxPacketSize
}

func (s *Service) WriteIsThreadUnsafe() {
}

func (s *Service) NewPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata M.Metadata) error {
	err := s.newPacket(ctx, conn, buffer, metadata)
	if err != nil {
		err = &shadowsocks.ServerPacketError{Source: metadata.Source, Cause: err}
	}
	return err
}

func (s *Service) newPacket(ctx context.Context, conn N.PacketConn, buffer *buf.Buffer, metadata M.Metadata) error {
	if buffer.Len() < s.keySaltLength {
		return io.ErrShortBuffer
	}
	_key := buf.StackNewSize(s.keySaltLength)
	key := common.Dup(_key)
	Kdf(s.key, buffer.To(s.keySaltLength), key)
	readCipher, err := s.constructor(key.Bytes())
	key.Release()
	common.KeepAlive(_key)
	if err != nil {
		return err
	}
	packet, err := readCipher.Open(buffer.Index(s.keySaltLength), rw.ZeroBytes[:readCipher.NonceSize()], buffer.From(s.keySaltLength), nil)
	if err != nil {
		return err
	}
	buffer.Advance(s.keySaltLength)
	buffer.Truncate(len(packet))

	destination, err := M.SocksaddrSerializer.ReadAddrPort(buffer)
	if err != nil {
		return err
	}

	metadata.Protocol = "shadowsocks"
	metadata.Destination = destination
	s.udpNat.NewPacket(ctx, metadata.Source.AddrPort(), buffer, metadata, func(natConn N.PacketConn) N.PacketWriter {
		return &serverPacketWriter{s, conn, natConn}
	})
	return nil
}

type serverPacketWriter struct {
	*Service
	source N.PacketConn
	nat    N.PacketConn
}

func (w *serverPacketWriter) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	header := buffer.ExtendHeader(w.keySaltLength + M.SocksaddrSerializer.AddrPortLen(destination))
	common.Must1(io.ReadFull(rand.Reader, header[:w.keySaltLength]))
	err := M.SocksaddrSerializer.WriteAddrPort(buf.With(header[w.keySaltLength:]), destination)
	if err != nil {
		buffer.Release()
		return err
	}
	_key := buf.StackNewSize(w.keySaltLength)
	key := common.Dup(_key)
	Kdf(w.key, buffer.To(w.keySaltLength), key)
	writeCipher, err := w.constructor(key.Bytes())
	key.Release()
	common.KeepAlive(_key)
	if err != nil {
		return err
	}
	writeCipher.Seal(buffer.From(w.keySaltLength)[:0], rw.ZeroBytes[:writeCipher.NonceSize()], buffer.From(w.keySaltLength), nil)
	buffer.Extend(Overhead)
	return w.source.WritePacket(buffer, M.SocksaddrFromNet(w.nat.LocalAddr()))
}

func (w *serverPacketWriter) FrontHeadroom() int {
	return w.keySaltLength + M.MaxSocksaddrLength
}

func (w *serverPacketWriter) RearHeadroom() int {
	return Overhead
}

func (w *serverPacketWriter) WriterMTU() int {
	return MaxPacketSize
}

func (w *serverPacketWriter) Upstream() any {
	return w.source
}
