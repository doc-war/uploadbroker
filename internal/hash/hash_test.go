package hash

import (
	"testing"
	"time"
)

func TestSum(t *testing.T) {
	h := Sum([]byte("hello"))
	if len(h) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(h))
	}

	h2 := Sum([]byte("hello"))
	if h != h2 {
		t.Fatal("same input must produce same hash")
	}

	h3 := Sum([]byte("world"))
	if h == h3 {
		t.Fatal("different input must produce different hash")
	}
}

func TestSumDeterministic(t *testing.T) {
	data := []byte("the quick brown fox jumps over the lazy dog")
	a := Sum(data)
	b := Sum(data)
	if a != b {
		t.Fatal("not deterministic")
	}
}

func TestSignToken(t *testing.T) {
	salt := "test-salt"
	key := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	expire := int64(1782650347)

	token := SignToken(salt, key, expire)
	if len(token) != 16 {
		t.Fatalf("expected 16 char token, got %d", len(token))
	}

	token2 := SignToken(salt, key, expire)
	if token != token2 {
		t.Fatal("same inputs must produce same token")
	}
}

func TestVerifyToken(t *testing.T) {
	salts := []string{"salt1", "salt2"}
	key := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	expire := int64(1782650347)

	token := SignToken(salts[0], key, expire)

	if !VerifyToken(salts, key, expire, token) {
		t.Fatal("must verify with correct salt")
	}

	if VerifyToken(salts, key, expire, "bad-token") {
		t.Fatal("must reject bad token")
	}

	badSalts := []string{"wrong-salt"}
	if VerifyToken(badSalts, key, expire, token) {
		t.Fatal("must reject token signed with different salt")
	}
}

func TestVerifyTokenFallback(t *testing.T) {
	salts := []string{"current", "previous"}
	key := "test-key-1234"
	expire := int64(1782650347)

	tokenOld := SignToken(salts[1], key, expire)

	if !VerifyToken(salts, key, expire, tokenOld) {
		t.Fatal("must verify with previous salt")
	}
}

func TestBuildURL(t *testing.T) {
	salt := "test-salt"
	baseURL := "https://upload.example.com"
	urlPrefix := "tmp"
	key := "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890"
	ext := ".png"
	expire := int64(1782650347)

	url := BuildURL(baseURL, urlPrefix, key, ext, expire, salt)
	expectedPrefix := "https://upload.example.com/tmp/1782650347/ab/abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890/"
	if len(url) <= len(expectedPrefix) {
		t.Fatalf("url too short: %s", url)
	}
	if url[:len(expectedPrefix)] != expectedPrefix {
		t.Fatalf("expected prefix %s, got %s", expectedPrefix, url[:len(expectedPrefix)])
	}
	if url[len(url)-len(ext):] != ext {
		t.Fatalf("expected suffix %s, got %s", ext, url[len(url)-len(ext):])
	}
}

func TestHMACSign(t *testing.T) {
	secret := "my-secret"
	msg := "hello"

	sign := HMACSign(secret, msg)
	if len(sign) != 64 {
		t.Fatalf("expected 64 hex chars, got %d", len(sign))
	}

	sign2 := HMACSign(secret, msg)
	if sign != sign2 {
		t.Fatal("must be deterministic")
	}

	if !VerifyHMAC(secret, msg, sign) {
		t.Fatal("must verify")
	}

	if VerifyHMAC(secret, msg, sign+"x") {
		t.Fatal("must reject modified signature")
	}

	if VerifyHMAC("wrong-secret", msg, sign) {
		t.Fatal("must reject wrong secret")
	}
}

func TestFormatInt64(t *testing.T) {
	now := time.Now().Unix()
	got := formatInt64(now)
	expected := time.Unix(now, 0).UTC().Format("20060102150405")
	if got != expected {
		t.Errorf("formatInt64(%d) = %s, want %s", now, got, expected)
	}

	gotZero := formatInt64(0)
	expectedZero := time.Unix(0, 0).UTC().Format("20060102150405")
	if gotZero != expectedZero {
		t.Errorf("formatInt64(0) = %s, want %s", gotZero, expectedZero)
	}
}
