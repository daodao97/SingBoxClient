package bufio

import (
	"io"
	"net"
	"time"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type CachedConn struct {
	net.Conn
	buffer *buf.Buffer
}

func NewCachedConn(conn net.Conn, buffer *buf.Buffer) *CachedConn {
	return &CachedConn{
		Conn:   conn,
		buffer: buffer,
	}
}

func (c *CachedConn) ReadCached() *buf.Buffer {
	buffer := c.buffer
	c.buffer = nil
	return buffer
}

func (c *CachedConn) Read(p []byte) (n int, err error) {
	if c.buffer != nil {
		n, err = c.buffer.Read(p)
		if err == nil {
			return
		}
		c.buffer.Release()
		c.buffer = nil
	}
	return c.Conn.Read(p)
}

func (c *CachedConn) WriteTo(w io.Writer) (n int64, err error) {
	if c.buffer != nil {
		wn, wErr := w.Write(c.buffer.Bytes())
		if wErr != nil {
			c.buffer.Release()
			c.buffer = nil
		}
		n += int64(wn)
	}
	cn, err := Copy(w, c.Conn)
	n += cn
	return
}

func (c *CachedConn) SetReadDeadline(t time.Time) error {
	if c.buffer != nil && !c.buffer.IsEmpty() {
		return nil
	}
	return c.Conn.SetReadDeadline(t)
}

func (c *CachedConn) ReadFrom(r io.Reader) (n int64, err error) {
	return Copy(c.Conn, r)
}

func (c *CachedConn) Upstream() any {
	return c.Conn
}

func (c *CachedConn) ReaderReplaceable() bool {
	return c.buffer == nil
}

func (c *CachedConn) WriterReplaceable() bool {
	return true
}

func (c *CachedConn) Close() error {
	c.buffer.Release()
	return c.Conn.Close()
}

type CachedReader struct {
	upstream io.Reader
	buffer   *buf.Buffer
}

func NewCachedReader(upstream io.Reader, buffer *buf.Buffer) *CachedReader {
	return &CachedReader{
		upstream: upstream,
		buffer:   buffer,
	}
}

func (r *CachedReader) ReadCached() *buf.Buffer {
	buffer := r.buffer
	r.buffer = nil
	return buffer
}

func (r *CachedReader) Read(p []byte) (n int, err error) {
	if r.buffer != nil {
		n, err = r.buffer.Read(p)
		if err == nil {
			return
		}
		r.buffer.Release()
		r.buffer = nil
	}
	return r.upstream.Read(p)
}

func (r *CachedReader) WriteTo(w io.Writer) (n int64, err error) {
	if r.buffer != nil {
		wn, wErr := w.Write(r.buffer.Bytes())
		if wErr != nil {
			return 0, wErr
		}
		n += int64(wn)
	}
	cn, err := Copy(w, r.upstream)
	n += cn
	return
}

func (r *CachedReader) Upstream() any {
	return r.upstream
}

func (r *CachedReader) ReaderReplaceable() bool {
	return r.buffer == nil
}

func (r *CachedReader) Close() error {
	r.buffer.Release()
	return nil
}

type CachedPacketConn struct {
	N.PacketConn
	buffer      *buf.Buffer
	destination M.Socksaddr
}

func NewCachedPacketConn(conn N.PacketConn, buffer *buf.Buffer, destination M.Socksaddr) *CachedPacketConn {
	return &CachedPacketConn{
		PacketConn:  conn,
		buffer:      buffer,
		destination: destination,
	}
}

func (c *CachedPacketConn) ReadPacket(buffer *buf.Buffer) (destination M.Socksaddr, err error) {
	if c.buffer != nil {
		_, err = buffer.ReadOnceFrom(c.buffer)
		if err != nil {
			return M.Socksaddr{}, err
		}
		c.buffer = nil
		return c.destination, nil
	}
	return c.PacketConn.ReadPacket(buffer)
}

func (c *CachedPacketConn) ReadCachedPacket() (destination M.Socksaddr, buffer *buf.Buffer) {
	buffer = c.buffer
	c.buffer = nil
	return c.destination, buffer
}

func (c *CachedPacketConn) Upstream() any {
	return c.PacketConn
}

func (c *CachedPacketConn) ReaderReplaceable() bool {
	return c.buffer == nil
}

func (c *CachedPacketConn) WriterReplaceable() bool {
	return true
}
