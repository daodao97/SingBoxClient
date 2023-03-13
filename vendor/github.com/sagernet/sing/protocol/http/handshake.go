package http

import (
	std_bufio "bufio"
	"context"
	"encoding/base64"
	"net"
	"net/http"
	"strings"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/auth"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type Handler = N.TCPConnectionHandler

func HandleConnection(ctx context.Context, conn net.Conn, reader *std_bufio.Reader, authenticator auth.Authenticator, handler Handler, metadata M.Metadata) error {
	var httpClient *http.Client
	for {
		request, err := ReadRequest(reader)
		if err != nil {
			return E.Cause(err, "read http request")
		}

		if authenticator != nil {
			var authOk bool
			authorization := request.Header.Get("Proxy-Authorization")
			if strings.HasPrefix(authorization, "Basic ") {
				userPassword, _ := base64.URLEncoding.DecodeString(authorization[6:])
				userPswdArr := strings.SplitN(string(userPassword), ":", 2)
				authOk = authenticator.Verify(userPswdArr[0], userPswdArr[1])
				if authOk {
					ctx = auth.ContextWithUser(ctx, userPswdArr[0])
				}
			}
			if !authOk {
				err = responseWith(request, http.StatusProxyAuthRequired).Write(conn)
				if err != nil {
					return err
				}
			}
		}

		if sourceAddress := SourceAddress(request); sourceAddress.IsValid() {
			metadata.Source = sourceAddress
		}

		if request.Method == "CONNECT" {
			portStr := request.URL.Port()
			if portStr == "" {
				portStr = "80"
			}
			destination := M.ParseSocksaddrHostPortStr(request.URL.Hostname(), portStr)
			_, err = conn.Write([]byte(F.ToString("HTTP/", request.ProtoMajor, ".", request.ProtoMinor, " 200 Connection established\r\n\r\n")))
			if err != nil {
				return E.Cause(err, "write http response")
			}
			metadata.Protocol = "http"
			metadata.Destination = destination

			var requestConn net.Conn
			if reader.Buffered() > 0 {
				buffer := buf.NewSize(reader.Buffered())
				_, err = buffer.ReadFullFrom(reader, reader.Buffered())
				if err != nil {
					return err
				}
				requestConn = bufio.NewCachedConn(conn, buffer)
			} else {
				requestConn = conn
			}
			return handler.NewConnection(ctx, requestConn, metadata)
		}

		keepAlive := !(request.ProtoMajor == 1 && request.ProtoMinor == 0) && strings.TrimSpace(strings.ToLower(request.Header.Get("Proxy-Connection"))) == "keep-alive"
		request.RequestURI = ""

		removeHopByHopHeaders(request.Header)
		removeExtraHTTPHostPort(request)

		if request.URL.Scheme == "" || request.URL.Host == "" {
			return responseWith(request, http.StatusBadRequest).Write(conn)
		}

		var innerErr error
		if httpClient == nil {
			httpClient = &http.Client{
				Transport: &http.Transport{
					DisableCompression: true,
					DialContext: func(context context.Context, network, address string) (net.Conn, error) {
						metadata.Destination = M.ParseSocksaddr(address)
						metadata.Protocol = "http"
						input, output := net.Pipe()
						go func() {
							hErr := handler.NewConnection(ctx, output, metadata)
							if hErr != nil {
								innerErr = hErr
								common.Close(input, output)
							}
						}()
						return input, nil
					},
				},
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					return http.ErrUseLastResponse
				},
			}
		}

		response, err := httpClient.Do(request)
		if err != nil {
			return E.Errors(innerErr, err, responseWith(request, http.StatusBadGateway).Write(conn))
		}

		removeHopByHopHeaders(response.Header)

		if keepAlive {
			response.Header.Set("Proxy-Connection", "keep-alive")
			response.Header.Set("Connection", "keep-alive")
			response.Header.Set("Keep-Alive", "timeout=4")
		}

		response.Close = !keepAlive

		err = response.Write(conn)
		if err != nil {
			return E.Errors(innerErr, err)
		}

		if !keepAlive {
			return conn.Close()
		}
	}
}

func removeHopByHopHeaders(header http.Header) {
	// Strip hop-by-hop header based on RFC:
	// http://www.w3.org/Protocols/rfc2616/rfc2616-sec13.html#sec13.5.1
	// https://www.mnot.net/blog/2011/07/11/what_proxies_must_do

	header.Del("Proxy-Connection")
	header.Del("Proxy-Authenticate")
	header.Del("Proxy-Authorization")
	header.Del("TE")
	header.Del("Trailers")
	header.Del("Transfer-Encoding")
	header.Del("Upgrade")

	connections := header.Get("Connection")
	header.Del("Connection")
	if len(connections) == 0 {
		return
	}
	for _, h := range strings.Split(connections, ",") {
		header.Del(strings.TrimSpace(h))
	}
}

func removeExtraHTTPHostPort(req *http.Request) {
	host := req.Host
	if host == "" {
		host = req.URL.Host
	}

	if pHost, port, err := net.SplitHostPort(host); err == nil && port == "80" {
		host = pHost
	}

	req.Host = host
	req.URL.Host = host
}

func responseWith(request *http.Request, statusCode int) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Status:     http.StatusText(statusCode),
		Proto:      request.Proto,
		ProtoMajor: request.ProtoMajor,
		ProtoMinor: request.ProtoMinor,
		Header:     http.Header{},
	}
}
