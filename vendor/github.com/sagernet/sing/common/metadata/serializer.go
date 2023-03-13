package metadata

import (
	"encoding/binary"
	"io"
	"net/netip"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
)

const (
	MaxSocksaddrLength   = 2 + 255 + 2
	MaxIPSocksaddrLength = 1 + 16 + 2
)

type SerializerOption func(*Serializer)

func AddressFamilyByte(b byte, f Family) SerializerOption {
	return func(s *Serializer) {
		s.familyMap[b] = f
		s.familyByteMap[f] = b
	}
}

func PortThenAddress() SerializerOption {
	return func(s *Serializer) {
		s.portFirst = true
	}
}

type Serializer struct {
	familyMap     map[byte]Family
	familyByteMap map[Family]byte
	portFirst     bool
}

func NewSerializer(options ...SerializerOption) *Serializer {
	s := &Serializer{
		familyMap:     make(map[byte]Family),
		familyByteMap: make(map[Family]byte),
	}
	for _, option := range options {
		option(s)
	}
	return s
}

func (s *Serializer) WriteAddress(buffer *buf.Buffer, addr Socksaddr) error {
	var af Family
	if addr.IsIPv4() {
		af = AddressFamilyIPv4
	} else if addr.IsIPv6() {
		af = AddressFamilyIPv6
	} else {
		af = AddressFamilyFqdn
	}
	err := buffer.WriteByte(s.familyByteMap[af])
	if err != nil {
		return err
	}
	if addr.Addr.IsValid() {
		_, err = buffer.Write(addr.Addr.AsSlice())
	} else {
		err = WriteSocksString(buffer, addr.Fqdn)
	}
	return err
}

func (s *Serializer) AddressLen(addr Socksaddr) int {
	if addr.IsIPv4() {
		return 5
	} else if addr.IsIPv6() {
		return 17
	} else {
		return 2 + len(addr.Fqdn)
	}
}

func (s *Serializer) WritePort(writer io.Writer, port uint16) error {
	return binary.Write(writer, binary.BigEndian, port)
}

func (s *Serializer) WriteAddrPort(writer io.Writer, destination Socksaddr) error {
	buffer, isBuffer := writer.(*buf.Buffer)
	if !isBuffer {
		buffer = buf.NewSize(s.AddrPortLen(destination))
		defer buffer.Release()
	}
	var err error
	if !s.portFirst {
		err = s.WriteAddress(buffer, destination)
	} else {
		err = s.WritePort(buffer, destination.Port)
	}
	if err != nil {
		return err
	}
	if s.portFirst {
		err = s.WriteAddress(buffer, destination)
	} else {
		err = s.WritePort(buffer, destination.Port)
	}
	if err != nil {
		return err
	}
	if !isBuffer {
		err = rw.WriteBytes(writer, buffer.Bytes())
	}
	return err
}

func (s *Serializer) AddrPortLen(destination Socksaddr) int {
	return s.AddressLen(destination) + 2
}

func (s *Serializer) ReadAddress(reader io.Reader) (Socksaddr, error) {
	af, err := rw.ReadByte(reader)
	if err != nil {
		return Socksaddr{}, err
	}
	family := s.familyMap[af]
	switch family {
	case AddressFamilyFqdn:
		fqdn, err := ReadSockString(reader)
		if err != nil {
			return Socksaddr{}, E.Cause(err, "read fqdn")
		}
		return ParseSocksaddrHostPort(fqdn, 0), nil
	default:
		switch family {
		case AddressFamilyIPv4:
			var addr [4]byte
			err = common.Error(reader.Read(addr[:]))
			if err != nil {
				return Socksaddr{}, E.Cause(err, "read ipv4 address")
			}
			return Socksaddr{Addr: netip.AddrFrom4(addr)}, nil
		case AddressFamilyIPv6:
			var addr [16]byte
			err = common.Error(reader.Read(addr[:]))
			if err != nil {
				return Socksaddr{}, E.Cause(err, "read ipv6 address")
			}

			return Socksaddr{Addr: netip.AddrFrom16(addr)}.Unwrap(), nil
		default:
			return Socksaddr{}, E.New("unknown address family: ", af)
		}
	}
}

func (s *Serializer) ReadPort(reader io.Reader) (uint16, error) {
	port, err := rw.ReadBytes(reader, 2)
	if err != nil {
		return 0, E.Cause(err, "read port")
	}
	return binary.BigEndian.Uint16(port), nil
}

func (s *Serializer) ReadAddrPort(reader io.Reader) (destination Socksaddr, err error) {
	var addr Socksaddr
	var port uint16
	if !s.portFirst {
		addr, err = s.ReadAddress(reader)
	} else {
		port, err = s.ReadPort(reader)
	}
	if err != nil {
		return
	}
	if s.portFirst {
		addr, err = s.ReadAddress(reader)
	} else {
		port, err = s.ReadPort(reader)
	}
	if err != nil {
		return
	}
	addr.Port = port
	return addr, nil
}

func ReadSockString(reader io.Reader) (string, error) {
	strLen, err := rw.ReadByte(reader)
	if err != nil {
		return "", err
	}
	return rw.ReadString(reader, int(strLen))
}

func WriteSocksString(buffer *buf.Buffer, str string) error {
	strLen := len(str)
	if strLen > 255 {
		return E.New("fqdn too long")
	}
	err := buffer.WriteByte(byte(strLen))
	if err != nil {
		return err
	}
	return common.Error(buffer.WriteString(str))
}
