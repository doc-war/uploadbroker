package hash

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/blake2b"
)

func SignToken(salt, key string, expire int64) string {
	data := fmt.Sprintf("%s/%s/%s", salt, formatInt64(expire), key)
	h := blake2b.Sum256([]byte(data))
	return hex.EncodeToString(h[:])[:16]
}

func VerifyToken(salts []string, key string, expire int64, token string) bool {
	for _, salt := range salts {
		if SignToken(salt, key, expire) == token {
			return true
		}
	}
	return false
}

func BuildURL(baseURL, urlPrefix, key, ext string, expire int64, salt string) string {
	shard := key[:2]
	token := SignToken(salt, key, expire)
	return fmt.Sprintf("%s/%s/%d/%s/%s/%s%s", baseURL, urlPrefix, expire, shard, key, token, ext)
}

func HMACSign(secret, msg string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

func VerifyHMAC(secret, msg, sign string) bool {
	return hmac.Equal([]byte(HMACSign(secret, msg)), []byte(sign))
}

func formatInt64(n int64) string {
	t := time.Unix(n, 0).UTC()
	return t.Format("20060102150405")
}
