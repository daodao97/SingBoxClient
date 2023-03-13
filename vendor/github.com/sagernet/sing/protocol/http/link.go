package http

import (
	"bufio"
	"net/http"
	_ "unsafe" // for linkname
)

//go:linkname ReadRequest net/http.readRequest
func ReadRequest(b *bufio.Reader) (req *http.Request, err error)
