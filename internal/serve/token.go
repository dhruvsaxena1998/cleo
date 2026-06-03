package serve

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
)

// URL is the address the QR encodes: the LAN page with the access token in the
// query string. The phone scans it and the page forwards the token on every
// poll.
func URL(ip string, port int, token string) string {
	return fmt.Sprintf("http://%s:%d/?token=%s", ip, port, token)
}

// NewToken returns a per-run access token: 16 bytes of crypto-random entropy,
// hex-encoded so it is always URL-safe (it travels in the QR'd query string).
func NewToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// tokenOK reports whether got matches want using a constant-time comparison,
// so a network observer cannot learn the token byte-by-byte from timing.
func tokenOK(want, got string) bool {
	return subtle.ConstantTimeCompare([]byte(want), []byte(got)) == 1
}
