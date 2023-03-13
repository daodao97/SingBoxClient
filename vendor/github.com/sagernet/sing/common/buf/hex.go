package buf

import "encoding/hex"

func EncodeHexString(src []byte) string {
	dst := Make(hex.EncodedLen(len(src)))
	hex.Encode(dst, src)
	return string(dst)
}
