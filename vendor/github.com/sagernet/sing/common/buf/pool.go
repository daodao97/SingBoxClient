package buf

func Get(size int) []byte {
	return DefaultAllocator.Get(size)
}

func Put(buf []byte) error {
	return DefaultAllocator.Put(buf)
}

func Make(size int) []byte {
	var buffer []byte
	switch {
	case size <= 2:
		buffer = make([]byte, 2)
	case size <= 4:
		buffer = make([]byte, 4)
	case size <= 8:
		buffer = make([]byte, 8)
	case size <= 16:
		buffer = make([]byte, 16)
	case size <= 32:
		buffer = make([]byte, 32)
	case size <= 64:
		buffer = make([]byte, 64)
	case size <= 128:
		buffer = make([]byte, 128)
	case size <= 256:
		buffer = make([]byte, 256)
	case size <= 512:
		buffer = make([]byte, 512)
	case size <= 1024:
		buffer = make([]byte, 1024)
	case size <= 2048:
		buffer = make([]byte, 2048)
	case size <= 4096:
		buffer = make([]byte, 4096)
	case size <= 8192:
		buffer = make([]byte, 8192)
	case size <= 16384:
		buffer = make([]byte, 16384)
	case size <= 32768:
		buffer = make([]byte, 32768)
	case size <= 65535:
		buffer = make([]byte, 65535)
	default:
		return make([]byte, size)
	}
	return buffer[:size]
}
