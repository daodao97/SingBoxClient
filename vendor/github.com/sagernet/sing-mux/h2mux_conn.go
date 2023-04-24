package mux

import (
	"io"
	"net"
	"os"
	"time"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/baderror"
)

type httpConn struct {
	reader io.Reader
	writer io.Writer
	create chan struct{}
	err    error
}

func newHTTPConn(reader io.Reader, writer io.Writer) *httpConn {
	return &httpConn{
		reader: reader,
		writer: writer,
	}
}

func newLateHTTPConn(writer io.Writer) *httpConn {
	return &httpConn{
		create: make(chan struct{}),
		writer: writer,
	}
}

func (c *httpConn) setup(reader io.Reader, err error) {
	c.reader = reader
	c.err = err
	close(c.create)
}

func (c *httpConn) Read(b []byte) (n int, err error) {
	if c.reader == nil {
		<-c.create
		if c.err != nil {
			return 0, c.err
		}
	}
	n, err = c.reader.Read(b)
	return n, baderror.WrapH2(err)
}

func (c *httpConn) Write(b []byte) (n int, err error) {
	n, err = c.writer.Write(b)
	return n, baderror.WrapH2(err)
}

func (c *httpConn) Close() error {
	return common.Close(c.reader, c.writer)
}

func (c *httpConn) LocalAddr() net.Addr {
	return nil
}

func (c *httpConn) RemoteAddr() net.Addr {
	return nil
}

func (c *httpConn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *httpConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *httpConn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *httpConn) NeedAdditionalReadDeadline() bool {
	return true
}
