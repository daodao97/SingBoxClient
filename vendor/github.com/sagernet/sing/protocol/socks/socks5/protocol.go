package socks5

import (
	"io"
	"net/netip"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/rw"
)

const (
	Version byte = 5

	AuthTypeNotRequired       byte = 0x00
	AuthTypeGSSAPI            byte = 0x01
	AuthTypeUsernamePassword  byte = 0x02
	AuthTypeNoAcceptedMethods byte = 0xFF

	UsernamePasswordStatusSuccess byte = 0x00
	UsernamePasswordStatusFailure byte = 0x01

	CommandConnect      byte = 0x01
	CommandBind         byte = 0x02
	CommandUDPAssociate byte = 0x03

	ReplyCodeSuccess                byte = 0
	ReplyCodeFailure                byte = 1
	ReplyCodeNotAllowed             byte = 2
	ReplyCodeNetworkUnreachable     byte = 3
	ReplyCodeHostUnreachable        byte = 4
	ReplyCodeConnectionRefused      byte = 5
	ReplyCodeTTLExpired             byte = 6
	ReplyCodeUnsupported            byte = 7
	ReplyCodeAddressTypeUnsupported byte = 8
)

// +----+----------+----------+
// |VER | NMETHODS | METHODS  |
// +----+----------+----------+
// | 1  |    1     | 1 to 255 |
// +----+----------+----------+

type AuthRequest struct {
	Methods []byte
}

func WriteAuthRequest(writer io.Writer, request AuthRequest) error {
	_buffer := buf.StackNewSize(len(request.Methods) + 2)
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(
		buffer.WriteByte(Version),
		buffer.WriteByte(byte(len(request.Methods))),
		common.Error(buffer.Write(request.Methods)),
	)
	return rw.WriteBytes(writer, buffer.Bytes())
}

func ReadAuthRequest(reader io.Reader) (request AuthRequest, err error) {
	version, err := rw.ReadByte(reader)
	if err != nil {
		return
	}
	if version != Version {
		err = E.New("expected socks version 5, got ", version)
		return
	}
	return ReadAuthRequest0(reader)
}

func ReadAuthRequest0(reader io.Reader) (request AuthRequest, err error) {
	methodLen, err := rw.ReadByte(reader)
	if err != nil {
		return
	}
	request.Methods, err = rw.ReadBytes(reader, int(methodLen))
	return
}

// +----+--------+
// |VER | METHOD |
// +----+--------+
// | 1  |   1    |
// +----+--------+

type AuthResponse struct {
	Method byte
}

func WriteAuthResponse(writer io.Writer, response AuthResponse) error {
	return rw.WriteBytes(writer, []byte{Version, response.Method})
}

func ReadAuthResponse(reader io.Reader) (response AuthResponse, err error) {
	version, err := rw.ReadByte(reader)
	if err != nil {
		return
	}
	if version != Version {
		err = E.New("expected socks version 5, got ", version)
		return
	}
	response.Method, err = rw.ReadByte(reader)
	return
}

// +----+------+----------+------+----------+
// |VER | ULEN |  UNAME   | PLEN |  PASSWD  |
// +----+------+----------+------+----------+
// | 1  |  1   | 1 to 255 |  1   | 1 to 255 |
// +----+------+----------+------+----------+

type UsernamePasswordAuthRequest struct {
	Username string
	Password string
}

func WriteUsernamePasswordAuthRequest(writer io.Writer, request UsernamePasswordAuthRequest) error {
	_buffer := buf.StackNewSize(3 + len(request.Username) + len(request.Password))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(
		buffer.WriteByte(1),
		M.WriteSocksString(buffer, request.Username),
		M.WriteSocksString(buffer, request.Password),
	)
	return rw.WriteBytes(writer, buffer.Bytes())
}

func ReadUsernamePasswordAuthRequest(reader io.Reader) (request UsernamePasswordAuthRequest, err error) {
	version, err := rw.ReadByte(reader)
	if err != nil {
		return
	}
	if version != 1 {
		err = E.New("excepted password request version 1, got ", version)
		return
	}
	request.Username, err = M.ReadSockString(reader)
	if err != nil {
		return
	}
	request.Password, err = M.ReadSockString(reader)
	if err != nil {
		return
	}
	return
}

// +----+--------+
// |VER | STATUS |
// +----+--------+
// | 1  |   1    |
// +----+--------+

type UsernamePasswordAuthResponse struct {
	Status byte
}

func WriteUsernamePasswordAuthResponse(writer io.Writer, response UsernamePasswordAuthResponse) error {
	return rw.WriteBytes(writer, []byte{1, response.Status})
}

func ReadUsernamePasswordAuthResponse(reader io.Reader) (response UsernamePasswordAuthResponse, err error) {
	version, err := rw.ReadByte(reader)
	if err != nil {
		return
	}
	if version != 1 {
		err = E.New("excepted password request version 1, got ", version)
		return
	}
	response.Status, err = rw.ReadByte(reader)
	return
}

// +----+-----+-------+------+----------+----------+
// |VER | CMD |  RSV  | ATYP | DST.ADDR | DST.PORT |
// +----+-----+-------+------+----------+----------+
// | 1  |  1  | X'00' |  1   | Variable |    2     |
// +----+-----+-------+------+----------+----------+

type Request struct {
	Command     byte
	Destination M.Socksaddr
}

func WriteRequest(writer io.Writer, request Request) error {
	_buffer := buf.StackNewSize(3 + M.SocksaddrSerializer.AddrPortLen(request.Destination))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(
		buffer.WriteByte(Version),
		buffer.WriteByte(request.Command),
		buffer.WriteZero(),
		M.SocksaddrSerializer.WriteAddrPort(buffer, request.Destination),
	)
	return rw.WriteBytes(writer, buffer.Bytes())
}

func ReadRequest(reader io.Reader) (request Request, err error) {
	version, err := rw.ReadByte(reader)
	if err != nil {
		return
	}
	if version != Version {
		err = E.New("expected socks version 5, got ", version)
		return
	}
	request.Command, err = rw.ReadByte(reader)
	if err != nil {
		return
	}
	err = rw.Skip(reader)
	if err != nil {
		return
	}
	request.Destination, err = M.SocksaddrSerializer.ReadAddrPort(reader)
	return
}

// +----+-----+-------+------+----------+----------+
// |VER | REP |  RSV  | ATYP | BND.ADDR | BND.PORT |
// +----+-----+-------+------+----------+----------+
// | 1  |  1  | X'00' |  1   | Variable |    2     |
// +----+-----+-------+------+----------+----------+

type Response struct {
	ReplyCode byte
	Bind      M.Socksaddr
}

func WriteResponse(writer io.Writer, response Response) error {
	var bind M.Socksaddr
	if response.Bind.IsValid() {
		bind = response.Bind
	} else {
		bind.Addr = netip.IPv4Unspecified()
	}

	_buffer := buf.StackNewSize(3 + M.SocksaddrSerializer.AddrPortLen(bind))
	defer common.KeepAlive(_buffer)
	buffer := common.Dup(_buffer)
	defer buffer.Release()
	common.Must(
		buffer.WriteByte(Version),
		buffer.WriteByte(response.ReplyCode),
		buffer.WriteZero(),
		M.SocksaddrSerializer.WriteAddrPort(buffer, bind),
	)
	return rw.WriteBytes(writer, buffer.Bytes())
}

func ReadResponse(reader io.Reader) (response Response, err error) {
	version, err := rw.ReadByte(reader)
	if err != nil {
		return
	}
	if version != Version {
		err = E.New("expected socks version 5, got ", version)
		return
	}
	response.ReplyCode, err = rw.ReadByte(reader)
	if err != nil {
		return
	}
	err = rw.Skip(reader)
	if err != nil {
		return
	}
	response.Bind, err = M.SocksaddrSerializer.ReadAddrPort(reader)
	return
}
