package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

func GeneratePassResetToken() (token string, hashedToken string, err error) {

	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}

	token = hex.EncodeToString(b)

	h := sha256.New()
	h.Write([]byte(token))
	hashedToken = hex.EncodeToString(h.Sum(nil))

	return token, hashedToken, nil
}
