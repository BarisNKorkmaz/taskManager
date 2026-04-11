package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

var secretKey = []byte("secretKey")

type CustomClaims struct {
	UserID    uint   `json:"userId"`
	Email     string `json:"email"`
	Type      string `json:"type"`
	SessionId string `json:"sessionId"`
	jwt.RegisteredClaims
}

type tokens struct {
	sessionId    string
	accessToken  string
	refreshToken string
	err          error
}

func GenerateAccessToken(userID uint, email string, sessionId string) (string, error) {

	now := time.Now()
	claims := CustomClaims{
		UserID:    userID,
		Email:     email,
		Type:      "access",
		SessionId: sessionId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(secretKey)

}

func GenerateRefreshToken(userID uint, email string, ipAddress string) tokens {

	var res tokens
	sessionId := uuid.New().String()
	now := time.Now()
	claims := CustomClaims{
		UserID:    userID,
		Email:     email,
		Type:      "refresh",
		SessionId: sessionId,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(168 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	refreshToken, refreshJwtErr := token.SignedString(secretKey)

	if refreshJwtErr != nil {
		res = tokens{
			accessToken:  "",
			refreshToken: "",
			err:          refreshJwtErr,
		}
		return res
	}

	h := sha256.New()
	h.Write([]byte(refreshToken))

	session := Session{
		UserID:    userID,
		ExpiresAt: claims.ExpiresAt.Time,
		CreatedAt: claims.IssuedAt.Time,
		ID:        sessionId,
		IpAddress: ipAddress,
		IsActive:  true,
		TokenHash: hex.EncodeToString(h.Sum(nil)),
	}

	atomicDB := database.DB.Begin()

	if atomicDB.Error != nil {
		res = tokens{
			accessToken:  "",
			refreshToken: "",
			err:          atomicDB.Error,
		}
		return res
	}

	if tx := database.DeleteSessionByUserId(atomicDB, userID, &Session{}); tx.Error != nil {
		atomicDB.Rollback()
		res = tokens{
			accessToken:  "",
			refreshToken: "",
			err:          tx.Error,
		}
		return res
	}

	if tx := database.Create(atomicDB, &session, &Session{}); tx.Error != nil {
		atomicDB.Rollback()
		res = tokens{
			accessToken:  "",
			refreshToken: "",
			err:          tx.Error,
		}
		return res
	}

	if tx := atomicDB.Commit(); tx.Error != nil {
		atomicDB.Rollback()
		res = tokens{
			accessToken:  "",
			refreshToken: "",
			err:          tx.Error,
		}
		return res
	}

	accessToken, jwtErr := GenerateAccessToken(userID, email, sessionId)

	if jwtErr != nil {
		res = tokens{
			accessToken:  "",
			refreshToken: "",
			err:          jwtErr,
		}
		return res
	}

	res = tokens{
		sessionId:    sessionId,
		accessToken:  accessToken,
		refreshToken: refreshToken,
		err:          nil,
	}
	return res

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
