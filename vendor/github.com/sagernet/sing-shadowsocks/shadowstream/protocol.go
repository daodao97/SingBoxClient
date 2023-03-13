package shadowstream

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"crypto/rand"
	"crypto/rc4"
	"io"
	"net"
	"os"

	"github.com/sagernet/sing-shadowsocks"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/crypto/chacha20"
)

var List = []string{
	"aes-128-ctr",
	"aes-192-ctr",
	"aes-256-ctr",
	"aes-128-cfb",
	"aes-192-cfb",
	"aes-256-cfb",
	"rc4-md5",
	"chacha20-ietf",
	"xchacha20",
}

type Method struct {
	name               string
	keyLength          int
	saltLength         int
	encryptConstructor func(key []byte, salt []byte) (cipher.Stream, error)
	decryptConstructor func(key []byte, salt []byte) (cipher.Stream, error)
	key                []byte
}

func New(method string, key []byte, password string) (shadowsocks.Method, error) {
	m := &Method{
		name: method,
	}
	switch method {
	case "aes-128-ctr":
		m.keyLength = 16
		m.saltLength = aes.BlockSize
		m.encryptConstructor = blockStream(aes.NewCipher, cipher.NewCTR)
		m.decryptConstructor = blockStream(aes.NewCipher, cipher.NewCTR)
	case "aes-192-ctr":
		m.keyLength = 24
		m.saltLength = aes.BlockSize
		m.encryptConstructor = blockStream(aes.NewCipher, cipher.NewCTR)
		m.decryptConstructor = blockStream(aes.NewCipher, cipher.NewCTR)
	case "aes-256-ctr":
		m.keyLength = 32
		m.saltLength = aes.BlockSize
		m.encryptConstructor = blockStream(aes.NewCipher, cipher.NewCTR)
		m.decryptConstructor = blockStream(aes.NewCipher, cipher.NewCTR)
	case "aes-128-cfb":
		m.keyLength = 16
		m.saltLength = aes.BlockSize
		m.encryptConstructor = blockStream(aes.NewCipher, cipher.NewCFBEncrypter)
		m.decryptConstructor = blockStream(aes.NewCipher, cipher.NewCFBDecrypter)
	case "aes-192-cfb":
		m.keyLength = 24
		m.saltLength = aes.BlockSize
		m.encryptConstructor = blockStream(aes.NewCipher, cipher.NewCFBEncrypter)
		m.decryptConstructor = blockStream(aes.NewCipher, cipher.NewCFBDecrypter)
	case "aes-256-cfb":
		m.keyLength = 32
		m.saltLength = aes.BlockSize
		m.encryptConstructor = blockStream(aes.NewCipher, cipher.NewCFBEncrypter)
		m.decryptConstructor = blockStream(aes.NewCipher, cipher.NewCFBDecrypter)
	case "rc4-md5":
		m.keyLength = 16
		m.saltLength = 16
		m.encryptConstructor = func(key []byte, salt []byte) (cipher.Stream, error) {
			h := md5.New()
			h.Write(key)
			h.Write(salt)
			return rc4.NewCipher(h.Sum(nil))
		}
		m.decryptConstructor = func(key []byte, salt []byte) (cipher.Stream, error) {
			h := md5.New()
			h.Write(key)
			h.Write(salt)
			return rc4.NewCipher(h.Sum(nil))
		}
	case "chacha20-ietf":
		m.keyLength = chacha20.KeySize
		m.saltLength = chacha20.NonceSize
		m.encryptConstructor = func(key []byte, salt []byte) (cipher.Stream, error) {
			return chacha20.NewUnauthenticatedCipher(key, salt)
		}
		m.decryptConstructor = func(key []byte, salt []byte) (cipher.Stream, error) {
			return chacha20.NewUnauthenticatedCipher(key, salt)
		}
	case "xchacha20":
		m.keyLength = chacha20.KeySize
		m.saltLength = chacha20.NonceSizeX
		m.encryptConstructor = func(key []byte, salt []byte) (cipher.Stream, error) {
			return chacha20.NewUnauthenticatedCipher(key, salt)
		}
		m.decryptConstructor = func(key []byte, salt []byte) (cipher.Stream, error) {
			return chacha20.NewUnauthenticatedCipher(key, salt)
		}
	default:
		return nil, os.ErrInvalid
	}
	if len(key) == m.keyLength {
		m.key = key
	} else if len(key) > 0 {
		return nil, shadowsocks.ErrBadKey
	} else if password != "" {
		m.key = shadowsocks.Key([]byte(password), m.keyLength)
	} else {
		return nil, shadowsocks.ErrMissingPassword
	}
	return m, nil
}

func blockStream(blockCreator func(key []byte) (cipher.Block, error), streamCreator func(block cipher.Block, iv []byte) cipher.Stream) func([]byte, []byte) (cipher.Stream, error) {
	return func(key []byte, iv []byte) (cipher.Stream, error) {
		block, err := blockCreator(key)
		if err != nil {
			return nil, err
		}
		return streamCreator(block, iv), err
	}
}

func (m *Method) Name() string {
	return m.name
}

func (m *Method) DialConn(conn net.Conn, destination M.Socksaddr) (net.Conn, error) {
	shadowsocksConn := &clientConn{
		Method:      m,
		Conn:        conn,
		destination: destination,
	}
	return shadowsocksConn, shadowsocksConn.writeRequest()
}

func (m *Method) DialEarlyConn(conn net.Conn, destination M.Socksaddr) net.Conn {
	return &clientConn{
		Method:      m,
		Conn:        conn,
		destination: destination,
	}
}

func (m *Method) DialPacketConn(conn net.Conn) N.NetPacketConn {
	return &clientPacketConn{m, conn}
}

type clientConn struct {
	*Method
	net.Conn
	destination M.Socksaddr
	readStream  cipher.Stream
	writeStream cipher.Stream
}

func (c *clientConn) writeRequest() error {
	_buffer := buf.StackNewSize(c.saltLength + M.SocksaddrSerializer.AddrPortLen(c.destination))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()

	salt := buffer.Extend(c.saltLength)
	common.Must1(io.ReadFull(rand.Reader, salt))

	stream, err := c.encryptConstructor(c.key, salt)
	if err != nil {
		return err
	}

	err = M.SocksaddrSerializer.WriteAddrPort(buffer, c.destination)
	if err != nil {
		return err
	}

	stream.XORKeyStream(buffer.From(c.saltLength), buffer.From(c.saltLength))

	_, err = c.Conn.Write(buffer.Bytes())
	if err != nil {
		return err
	}

	c.writeStream = stream
	return nil
}

func (c *clientConn) readResponse() error {
	if c.readStream != nil {
		return nil
	}
	_salt := buf.Make(c.saltLength)
	defer common.KeepAlive(_salt)
	salt := common.Dup(_salt)
	_, err := io.ReadFull(c.Conn, salt)
	if err != nil {
		return err
	}
	c.readStream, err = c.decryptConstructor(c.key, salt)
	return err
}

func (c *clientConn) Read(p []byte) (n int, err error) {
	if c.readStream == nil {
		err = c.readResponse()
		if err != nil {
			return
		}
	}
	n, err = c.Conn.Read(p)
	if err != nil {
		return
	}
	c.readStream.XORKeyStream(p[:n], p[:n])
	return
}

func (c *clientConn) Write(p []byte) (n int, err error) {
	if c.writeStream == nil {
		err = c.writeRequest()
		if err != nil {
			return
		}
	}

	c.writeStream.XORKeyStream(p, p)
	return c.Conn.Write(p)
}

func (c *clientConn) Upstream() any {
	return c.Conn
}

type clientPacketConn struct {
	*Method
	net.Conn
}

func (c *clientPacketConn) WritePacket(buffer *buf.Buffer, destination M.Socksaddr) error {
	defer buffer.Release()
	header := buf.With(buffer.ExtendHeader(c.saltLength + M.SocksaddrSerializer.AddrPortLen(destination)))
	common.Must1(header.ReadFullFrom(rand.Reader, c.saltLength))
	err := M.SocksaddrSerializer.WriteAddrPort(header, destination)
	if err != nil {
		return err
	}
	stream, err := c.encryptConstructor(c.key, buffer.To(c.saltLength))
	if err != nil {
		return err
	}
	stream.XORKeyStream(buffer.From(c.saltLength), buffer.From(c.saltLength))
	return common.Error(c.Write(buffer.Bytes()))
}

func (c *clientPacketConn) ReadPacket(buffer *buf.Buffer) (M.Socksaddr, error) {
	n, err := c.Read(buffer.FreeBytes())
	if err != nil {
		return M.Socksaddr{}, err
	}
	buffer.Truncate(n)
	stream, err := c.decryptConstructor(c.key, buffer.To(c.saltLength))
	if err != nil {
		return M.Socksaddr{}, err
	}
	stream.XORKeyStream(buffer.From(c.saltLength), buffer.From(c.saltLength))
	buffer.Advance(c.saltLength)
	return M.SocksaddrSerializer.ReadAddrPort(buffer)
}

func (c *clientPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = c.Read(p)
	if err != nil {
		return
	}
	stream, err := c.decryptConstructor(c.key, p[:c.saltLength])
	if err != nil {
		return
	}
	buffer := buf.As(p[c.saltLength:n])
	stream.XORKeyStream(buffer.Bytes(), buffer.Bytes())
	destination, err := M.SocksaddrSerializer.ReadAddrPort(buffer)
	if err != nil {
		return
	}
	addr = destination.UDPAddr()
	n = copy(p, buffer.Bytes())
	return
}

func (c *clientPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	destination := M.SocksaddrFromNet(addr)
	_buffer := buf.StackNewSize(c.saltLength + M.SocksaddrSerializer.AddrPortLen(destination) + len(p))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must1(buffer.ReadFullFrom(rand.Reader, c.saltLength))
	err = M.SocksaddrSerializer.WriteAddrPort(buffer, M.SocksaddrFromNet(addr))
	if err != nil {
		return
	}
	_, err = buffer.Write(p)
	if err != nil {
		return
	}
	stream, err := c.encryptConstructor(c.key, buffer.To(c.saltLength))
	if err != nil {
		return
	}
	stream.XORKeyStream(buffer.From(c.saltLength), buffer.From(c.saltLength))
	_, err = c.Write(buffer.Bytes())
	if err != nil {
		return
	}
	return len(p), nil
}

func (c *clientPacketConn) FrontHeadroom() int {
	return c.saltLength + M.MaxSocksaddrLength
}

func (c *clientPacketConn) Upstream() any {
	return c.Conn
}
