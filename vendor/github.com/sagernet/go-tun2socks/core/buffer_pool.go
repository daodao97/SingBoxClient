package core

import "github.com/sagernet/sing/common/buf"

func NewBytes(size int) []byte {
	return buf.Get(size)
}

func FreeBytes(b []byte) {
	buf.Put(b)
}
