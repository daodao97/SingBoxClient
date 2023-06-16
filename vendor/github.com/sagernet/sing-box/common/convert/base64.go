package convert

import (
	"encoding/base64"
	"fmt"
	"strings"
)

var (
	encRaw = base64.RawStdEncoding
	enc    = base64.StdEncoding
)

func Base64Decode(encodedString string) ([]byte, error) {
	decodedData, err := base64.URLEncoding.DecodeString(encodedString) // 使用 base64.URLEncoding 解码
	if err != nil {
		// 如果使用 base64.URLEncoding 仍然无法解码，尝试使用 base64.RawURLEncoding
		decodedData, err = base64.RawURLEncoding.DecodeString(encodedString)
		if err != nil {
			return nil, err
		}
	}
	return decodedData, nil
}

// DecodeBase64 try to decode content from the given bytes,
// which can be in base64.RawStdEncoding, base64.StdEncoding or just plaintext.
func DecodeBase64(buf []byte) []byte {
	result, err := tryDecodeBase64(buf)
	if err != nil {
		fmt.Println("DecodeBase64 Err", err)
		return buf
	}
	return result
}

func tryDecodeBase64(buf []byte) ([]byte, error) {
	dBuf := make([]byte, encRaw.DecodedLen(len(buf)))
	n, err := encRaw.Decode(dBuf, buf)
	if err != nil {
		n, err = enc.Decode(dBuf, buf)
		if err != nil {
			return nil, err
		}
	}
	return dBuf[:n], nil
}

func urlSafe(data string) string {
	return strings.NewReplacer("+", "-", "/", "_").Replace(data)
}

func decodeUrlSafe(data string) string {
	dcBuf, err := base64.RawURLEncoding.DecodeString(data)
	if err != nil {
		return ""
	}
	return string(dcBuf)
}
