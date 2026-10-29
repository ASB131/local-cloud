package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"strings"
)

var inviteBase32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func GenerateInviteCode() (plaintext string, hash string, hint string, err error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", "", "", err
	}
	plaintext = strings.ToUpper(inviteBase32.EncodeToString(buf))
	hash = HashInviteCode(plaintext)
	if len(plaintext) >= 4 {
		hint = plaintext[len(plaintext)-4:]
	} else {
		hint = plaintext
	}
	return plaintext, hash, hint, nil
}

func HashInviteCode(plaintext string) string {
	normalized := strings.ToUpper(strings.TrimSpace(plaintext))
	sum := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(sum[:])
}
