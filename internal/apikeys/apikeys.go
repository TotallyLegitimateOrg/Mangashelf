package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

const tokenPrefix = "msak_"
const displayPrefixLength = 12

func Generate() (raw string, prefix string, hash string, err error) {
	secret := make([]byte, 32)
	if _, err = rand.Read(secret); err != nil {
		return "", "", "", err
	}

	raw = tokenPrefix + base64.RawURLEncoding.EncodeToString(secret)
	return raw, Prefix(raw), Hash(raw), nil
}

func Prefix(raw string) string {
	if len(raw) <= displayPrefixLength {
		return raw
	}
	return raw[:displayPrefixLength]
}

func Hash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
