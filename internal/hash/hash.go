package hash

import (
	"golang.org/x/crypto/blake2b"
)

func Sum(data []byte) string {
	h := blake2b.Sum256(data)
	return hexEncode(h[:])
}

func hexEncode(src []byte) string {
	const hextable = "0123456789abcdef"
	dst := make([]byte, len(src)*2)
	for i, v := range src {
		dst[i*2] = hextable[v>>4]
		dst[i*2+1] = hextable[v&0x0f]
	}
	return string(dst)
}
