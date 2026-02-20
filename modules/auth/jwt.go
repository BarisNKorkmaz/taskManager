package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var secretKey = []byte("secretKey")

type CustomClaims struct {
	UserID uint   `json:"userId"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

func GenerateJwtToken(userID uint, email string) (string, error) {

	claims := CustomClaims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(secretKey)

}

func JwtParseAndValidate(tokenStr string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, JwtKeyFunc)

	if err != nil {
		return nil, err
	}

	claims, ok := token.Claims.(*CustomClaims)

	if !ok || !token.Valid {
		return nil, errors.New("invalid token claims")
	}

	return claims, nil
}

func JwtKeyFunc(t *jwt.Token) (interface{}, error) {
	if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
		return nil, errors.New("unexpected signing method")
	}
	return secretKey, nil
}
