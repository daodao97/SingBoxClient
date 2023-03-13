package socks

import (
	"context"
	"io"
	"net"
	"net/netip"
	"os"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/protocol/socks/socks4"
	"github.com/sagernet/sing/protocol/socks/socks5"
)

type Handler interface {
	N.TCPConnectionHandler
	N.UDPConnectionHandler
}

func ClientHandshake4(conn io.ReadWriter, command byte, destination M.Socksaddr, username string) (socks4.Response, error) {
	err := socks4.WriteRequest(conn, socks4.Request{
		Command:     command,
		Destination: destination,
		Username:    username,
	})
	if err != nil {
		return socks4.Response{}, err
	}
	response, err := socks4.ReadResponse(conn)
	if err != nil {
		return socks4.Response{}, err
	}
	if response.ReplyCode != socks4.ReplyCodeGranted {
		err = E.New("socks4: request rejected, code= ", response.ReplyCode)
	}
	return response, err
}

func ClientHandshake5(conn io.ReadWriter, command byte, destination M.Socksaddr, username string, password string) (socks5.Response, error) {
	var method byte
	if username == "" {
		method = socks5.AuthTypeNotRequired
	} else {
		method = socks5.AuthTypeUsernamePassword
	}
	err := socks5.WriteAuthRequest(conn, socks5.AuthRequest{
		Methods: []byte{method},
	})
	if err != nil {
		return socks5.Response{}, err
	}
	authResponse, err := socks5.ReadAuthResponse(conn)
	if err != nil {
		return socks5.Response{}, err
	}
	if authResponse.Method == socks5.AuthTypeUsernamePassword {
		err = socks5.WriteUsernamePasswordAuthRequest(conn, socks5.UsernamePasswordAuthRequest{
			Username: username,
			Password: password,
		})
		if err != nil {
			return socks5.Response{}, err
		}
		usernamePasswordResponse, err := socks5.ReadUsernamePasswordAuthResponse(conn)
		if err != nil {
			return socks5.Response{}, err
		}
		if usernamePasswordResponse.Status != socks5.UsernamePasswordStatusSuccess {
			return socks5.Response{}, E.New("socks5: incorrect user name or password")
		}
	} else if authResponse.Method != socks5.AuthTypeNotRequired {
		return socks5.Response{}, E.New("socks5: unsupported auth method: ", authResponse.Method)
	}
	err = socks5.WriteRequest(conn, socks5.Request{
		Command:     command,
		Destination: destination,
	})
	if err != nil {
		return socks5.Response{}, err
	}
	response, err := socks5.ReadResponse(conn)
	if err != nil {
		return socks5.Response{}, err
	}
	if response.ReplyCode != socks5.ReplyCodeSuccess {
		err = E.New("socks5: request rejected, code=", response.ReplyCode)
	}
	return response, err
}

func HandleConnection(ctx context.Context, conn net.Conn, authenticator auth.Authenticator, handler Handler, metadata M.Metadata) error {
	version, err := rw.ReadByte(conn)
	if err != nil {
		return err
	}
	return HandleConnection0(ctx, conn, version, authenticator, handler, metadata)
}

func HandleConnection0(ctx context.Context, conn net.Conn, version byte, authenticator auth.Authenticator, handler Handler, metadata M.Metadata) error {
	switch version {
	case socks4.Version:
		request, err := socks4.ReadRequest0(conn)
		if err != nil {
			return err
		}
		switch request.Command {
		case socks4.CommandConnect:
			err = socks4.WriteResponse(conn, socks4.Response{
				ReplyCode:   socks4.ReplyCodeGranted,
				Destination: M.SocksaddrFromNet(conn.LocalAddr()),
			})
			if err != nil {
				return err
			}
			metadata.Protocol = "socks4"
			metadata.Destination = request.Destination
			return handler.NewConnection(auth.ContextWithUser(ctx, request.Username), conn, metadata)
		default:
			err = socks4.WriteResponse(conn, socks4.Response{
				ReplyCode:   socks4.ReplyCodeRejectedOrFailed,
				Destination: request.Destination,
			})
			if err != nil {
				return err
			}
			return E.New("socks4: unsupported command ", request.Command)
		}
	case socks5.Version:
		authRequest, err := socks5.ReadAuthRequest0(conn)
		if err != nil {
			return err
		}
		var authMethod byte
		if authenticator != nil && !common.Contains(authRequest.Methods, socks5.AuthTypeUsernamePassword) {
			err = socks5.WriteAuthResponse(conn, socks5.AuthResponse{
				Method: socks5.AuthTypeNoAcceptedMethods,
			})
			if err != nil {
				return err
			}
		}
		if authenticator != nil {
			authMethod = socks5.AuthTypeUsernamePassword
		} else {
			authMethod = socks5.AuthTypeNotRequired
		}
		err = socks5.WriteAuthResponse(conn, socks5.AuthResponse{
			Method: authMethod,
		})
		if err != nil {
			return err
		}
		if authMethod == socks5.AuthTypeUsernamePassword {
			usernamePasswordAuthRequest, err := socks5.ReadUsernamePasswordAuthRequest(conn)
			if err != nil {
				return err
			}
			ctx = auth.ContextWithUser(ctx, usernamePasswordAuthRequest.Username)
			response := socks5.UsernamePasswordAuthResponse{}
			if authenticator.Verify(usernamePasswordAuthRequest.Username, usernamePasswordAuthRequest.Password) {
				response.Status = socks5.UsernamePasswordStatusSuccess
			} else {
				response.Status = socks5.UsernamePasswordStatusFailure
			}
			err = socks5.WriteUsernamePasswordAuthResponse(conn, response)
			if err != nil {
				return err
			}
		}
		request, err := socks5.ReadRequest(conn)
		if err != nil {
			return err
		}
		switch request.Command {
		case socks5.CommandConnect:
			err = socks5.WriteResponse(conn, socks5.Response{
				ReplyCode: socks5.ReplyCodeSuccess,
				Bind:      M.SocksaddrFromNet(conn.LocalAddr()),
			})
			if err != nil {
				return err
			}
			metadata.Protocol = "socks5"
			metadata.Destination = request.Destination
			return handler.NewConnection(ctx, conn, metadata)
		case socks5.CommandUDPAssociate:
			var udpConn *net.UDPConn
			udpConn, err = net.ListenUDP(M.NetworkFromNetAddr("udp", M.AddrFromNetAddr(conn.LocalAddr())), net.UDPAddrFromAddrPort(netip.AddrPortFrom(M.AddrFromNetAddr(conn.LocalAddr()), 0)))
			if err != nil {
				return err
			}
			defer udpConn.Close()
			err = socks5.WriteResponse(conn, socks5.Response{
				ReplyCode: socks5.ReplyCodeSuccess,
				Bind:      M.SocksaddrFromNet(udpConn.LocalAddr()),
			})
			if err != nil {
				return err
			}
			metadata.Protocol = "socks5"
			metadata.Destination = request.Destination
			var innerError error
			done := make(chan struct{})
			go func() {
				defer conn.Close()
				innerError = handler.NewPacketConnection(ctx, NewAssociatePacketConn(udpConn, request.Destination, conn), metadata)
				close(done)
			}()
			err = common.Error(io.Copy(io.Discard, conn))
			<-done
			return E.Errors(innerError, err)
		default:
			err = socks5.WriteResponse(conn, socks5.Response{
				ReplyCode: socks5.ReplyCodeUnsupported,
			})
			if err != nil {
				return err
			}
			return E.New("socks5: unsupported command ", request.Command)
		}
	}
	return os.ErrInvalid
}
