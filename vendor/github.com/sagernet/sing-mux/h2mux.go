package mux

import (
	"context"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/sagernet/sing/common/atomic"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"

	"golang.org/x/net/http2"
)

const idleTimeout = 30 * time.Second

var _ abstractSession = (*h2MuxServerSession)(nil)

type h2MuxServerSession struct {
	server  http2.Server
	active  atomic.Int32
	conn    net.Conn
	inbound chan net.Conn
	done    chan struct{}
}

func newH2MuxServer(conn net.Conn) *h2MuxServerSession {
	session := &h2MuxServerSession{
		conn:    conn,
		inbound: make(chan net.Conn),
		done:    make(chan struct{}),
		server: http2.Server{
			IdleTimeout:      idleTimeout,
			MaxReadFrameSize: buf.BufferSize,
		},
	}
	go func() {
		session.server.ServeConn(conn, &http2.ServeConnOpts{
			Handler: session,
		})
		_ = session.Close()
	}()
	return session
}

func (s *h2MuxServerSession) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	s.active.Add(1)
	defer s.active.Add(-1)
	writer.WriteHeader(http.StatusOK)
	conn := newHTTP2Wrapper(newHTTPConn(request.Body, writer), writer.(http.Flusher))
	s.inbound <- conn
	select {
	case <-conn.done:
	case <-s.done:
		_ = conn.Close()
	}
}

func (s *h2MuxServerSession) Open() (net.Conn, error) {
	return nil, os.ErrInvalid
}

func (s *h2MuxServerSession) Accept() (net.Conn, error) {
	select {
	case conn := <-s.inbound:
		return conn, nil
	case <-s.done:
		return nil, os.ErrClosed
	}
}

func (s *h2MuxServerSession) NumStreams() int {
	return int(s.active.Load())
}

func (s *h2MuxServerSession) Close() error {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	return s.conn.Close()
}

func (s *h2MuxServerSession) IsClosed() bool {
	select {
	case <-s.done:
		return true
	default:
		return false
	}
}

func (s *h2MuxServerSession) CanTakeNewRequest() bool {
	return false
}

type h2MuxConnWrapper struct {
	N.ExtendedConn
	flusher http.Flusher
	done    chan struct{}
}

func newHTTP2Wrapper(conn net.Conn, flusher http.Flusher) *h2MuxConnWrapper {
	return &h2MuxConnWrapper{
		ExtendedConn: bufio.NewExtendedConn(conn),
		flusher:      flusher,
		done:         make(chan struct{}),
	}
}

func (w *h2MuxConnWrapper) Write(p []byte) (n int, err error) {
	select {
	case <-w.done:
		return 0, net.ErrClosed
	default:
	}
	n, err = w.ExtendedConn.Write(p)
	if err == nil {
		w.flusher.Flush()
	}
	return
}

func (w *h2MuxConnWrapper) WriteBuffer(buffer *buf.Buffer) error {
	select {
	case <-w.done:
		return net.ErrClosed
	default:
	}
	err := w.ExtendedConn.WriteBuffer(buffer)
	if err == nil {
		w.flusher.Flush()
	}
	return err
}

func (w *h2MuxConnWrapper) Close() error {
	select {
	case <-w.done:
	default:
		close(w.done)
	}
	return w.ExtendedConn.Close()
}

func (w *h2MuxConnWrapper) Upstream() any {
	return w.ExtendedConn
}

var _ abstractSession = (*h2MuxClientSession)(nil)

type h2MuxClientSession struct {
	transport  *http2.Transport
	clientConn *http2.ClientConn
	done       chan struct{}
}

func newH2MuxClient(conn net.Conn) (*h2MuxClientSession, error) {
	session := &h2MuxClientSession{
		transport: &http2.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.Config) (net.Conn, error) {
				return conn, nil
			},
			ReadIdleTimeout:  idleTimeout,
			MaxReadFrameSize: buf.BufferSize,
		},
		done: make(chan struct{}),
	}
	session.transport.ConnPool = session
	clientConn, err := session.transport.NewClientConn(conn)
	if err != nil {
		return nil, err
	}
	session.clientConn = clientConn
	return session, nil
}

func (s *h2MuxClientSession) GetClientConn(req *http.Request, addr string) (*http2.ClientConn, error) {
	return s.clientConn, nil
}

func (s *h2MuxClientSession) MarkDead(conn *http2.ClientConn) {
	s.Close()
}

func (s *h2MuxClientSession) Open() (net.Conn, error) {
	pipeInReader, pipeInWriter := io.Pipe()
	request := &http.Request{
		Method: http.MethodConnect,
		Body:   pipeInReader,
		URL:    &url.URL{Scheme: "https", Host: "localhost"},
	}
	conn := newLateHTTPConn(pipeInWriter)
	go func() {
		response, err := s.transport.RoundTrip(request)
		if err != nil {
			conn.setup(nil, err)
		} else if response.StatusCode != 200 {
			response.Body.Close()
			conn.setup(nil, E.New("unexpected status: ", response.StatusCode, " ", response.Status))
		} else {
			conn.setup(response.Body, nil)
		}
	}()
	return conn, nil
}

func (s *h2MuxClientSession) Accept() (net.Conn, error) {
	return nil, os.ErrInvalid
}

func (s *h2MuxClientSession) NumStreams() int {
	return s.clientConn.State().StreamsActive
}

func (s *h2MuxClientSession) Close() error {
	select {
	case <-s.done:
	default:
		close(s.done)
	}
	return s.clientConn.Close()
}

func (s *h2MuxClientSession) IsClosed() bool {
	select {
	case <-s.done:
		return true
	default:
	}
	return s.clientConn.State().Closed
}

func (s *h2MuxClientSession) CanTakeNewRequest() bool {
	return s.clientConn.CanTakeNewRequest()
}
