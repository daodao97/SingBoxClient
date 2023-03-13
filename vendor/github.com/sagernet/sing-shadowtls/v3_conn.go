package shadowtls

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"hash"
	"io"
	"net"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

type verifiedConn struct {
	net.Conn
	writer     N.VectorisedWriter
	hmacAdd    hash.Hash
	hmacVerify hash.Hash
	hmacIgnore hash.Hash

	buffer *buf.Buffer
}

func newVerifiedConn(
	conn net.Conn,
	hmacAdd hash.Hash,
	hmacVerify hash.Hash,
	hmacIgnore hash.Hash,
) *verifiedConn {
	return &verifiedConn{
		Conn:       conn,
		writer:     bufio.NewVectorisedWriter(conn),
		hmacAdd:    hmacAdd,
		hmacVerify: hmacVerify,
		hmacIgnore: hmacIgnore,
	}
}

func (c *verifiedConn) Read(b []byte) (n int, err error) {
	if c.buffer != nil {
		if !c.buffer.IsEmpty() {
			return c.buffer.Read(b)
		}
		c.buffer.Release()
		c.buffer = nil
	}
	for {
		var tlsHeader [tlsHeaderSize]byte
		_, err = io.ReadFull(c.Conn, tlsHeader[:])
		if err != nil {
			sendAlert(c.Conn)
			return
		}
		length := int(binary.BigEndian.Uint16(tlsHeader[3:tlsHeaderSize]))
		c.buffer = buf.NewSize(tlsHeaderSize + length)
		common.Must1(c.buffer.Write(tlsHeader[:]))
		_, err = c.buffer.ReadFullFrom(c.Conn, length)
		if err != nil {
			return
		}
		buffer := c.buffer.Bytes()
		switch buffer[0] {
		case alert:
			err = E.Cause(net.ErrClosed, "remote alert")
			return
		case applicationData:
			if c.hmacIgnore != nil {
				if verifyApplicationData(buffer, c.hmacIgnore, false) {
					c.buffer.Release()
					c.buffer = nil
					continue
				} else {
					c.hmacIgnore = nil
				}
			}
			if !verifyApplicationData(buffer, c.hmacVerify, true) {
				sendAlert(c.Conn)
				err = E.New("application data verification failed")
				return
			}
			c.buffer.Advance(tlsHmacHeaderSize)
		default:
			sendAlert(c.Conn)
			err = E.New("unexpected TLS record type: ", buffer[0])
			return
		}
		return c.buffer.Read(b)
	}
}

func (c *verifiedConn) Write(p []byte) (n int, err error) {
	pTotal := len(p)
	for len(p) > 0 {
		var pWrite []byte
		if len(p) > 16384 {
			pWrite = p[:16384]
			p = p[16384:]
		} else {
			pWrite = p
			p = nil
		}
		_, err = c.write(pWrite)
	}
	if err == nil {
		n = pTotal
	}
	return
}

func (c *verifiedConn) write(p []byte) (n int, err error) {
	var header [tlsHmacHeaderSize]byte
	header[0] = applicationData
	header[1] = 3
	header[2] = 3
	binary.BigEndian.PutUint16(header[3:tlsHeaderSize], hmacSize+uint16(len(p)))
	c.hmacAdd.Write(p)
	hmacHash := c.hmacAdd.Sum(nil)[:hmacSize]
	c.hmacAdd.Write(hmacHash)
	copy(header[tlsHeaderSize:], hmacHash)
	_, err = bufio.WriteVectorised(c.writer, [][]byte{common.Dup(header[:]), p})
	if err == nil {
		n = len(p)
	}
	return
}

func (c *verifiedConn) WriteVectorised(buffers []*buf.Buffer) error {
	var header [tlsHmacHeaderSize]byte
	header[0] = applicationData
	header[1] = 3
	header[2] = 3
	binary.BigEndian.PutUint16(header[3:tlsHeaderSize], hmacSize+uint16(buf.LenMulti(buffers)))
	for _, buffer := range buffers {
		c.hmacAdd.Write(buffer.Bytes())
	}
	c.hmacAdd.Write(c.hmacAdd.Sum(nil)[:hmacSize])
	copy(header[tlsHeaderSize:], c.hmacAdd.Sum(nil)[:hmacSize])
	return c.writer.WriteVectorised(append([]*buf.Buffer{buf.As(header[:])}, buffers...))
}

func verifyApplicationData(frame []byte, hmac hash.Hash, update bool) bool {
	if frame[1] != 3 || frame[2] != 3 || len(frame) < tlsHmacHeaderSize {
		return false
	}
	hmac.Write(frame[tlsHmacHeaderSize:])
	hmacHash := hmac.Sum(nil)[:hmacSize]
	if update {
		hmac.Write(hmacHash)
	}
	return bytes.Equal(frame[tlsHeaderSize:tlsHeaderSize+hmacSize], hmacHash)
}

func sendAlert(writer io.Writer) {
	const recordSize = 31
	record := [recordSize]byte{
		alert,
		3,
		3,
		0,
		recordSize - tlsHeaderSize,
	}
	_, err := rand.Read(record[tlsHeaderSize:])
	if err != nil {
		return
	}
	writer.Write(record[:])
}
