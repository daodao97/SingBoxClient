package rw

import (
	"encoding/binary"
	"io"

	"github.com/sagernet/sing/common"
)

type stubByteReader struct {
	io.Reader
}

func (r stubByteReader) ReadByte() (byte, error) {
	return ReadByte(r.Reader)
}

func ToByteReader(reader io.Reader) io.ByteReader {
	if byteReader, ok := reader.(io.ByteReader); ok {
		return byteReader
	}
	return &stubByteReader{reader}
}

func ReadUVariant(reader io.Reader) (uint64, error) {
	return binary.ReadUvarint(ToByteReader(reader))
}

func UVariantLen(x uint64) int {
	switch {
	case x < 1<<(7*1):
		return 1
	case x < 1<<(7*2):
		return 2
	case x < 1<<(7*3):
		return 3
	case x < 1<<(7*4):
		return 4
	case x < 1<<(7*5):
		return 5
	case x < 1<<(7*6):
		return 6
	case x < 1<<(7*7):
		return 7
	case x < 1<<(7*8):
		return 8
	case x < 1<<(7*9):
		return 9
	default:
		return 10
	}
}

func WriteUVariant(writer io.Writer, value uint64) error {
	var b [8]byte
	return common.Error(writer.Write(b[:binary.PutUvarint(b[:], value)]))
}

func WriteVString(writer io.Writer, value string) error {
	err := WriteUVariant(writer, uint64(len(value)))
	if err != nil {
		return err
	}
	return WriteString(writer, value)
}

func ReadVString(reader io.Reader) (string, error) {
	length, err := binary.ReadUvarint(ToByteReader(reader))
	if err != nil {
		return "", err
	}
	value, err := ReadBytes(reader, int(length))
	if err != nil {
		return "", err
	}
	return string(value), nil
}
