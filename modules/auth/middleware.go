package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/limiter"
	"github.com/golang-jwt/jwt/v5"
)

func AccessTokenMiddleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")

		if authHeader == "" {
			return c.Status(401).JSON(fiber.Map{
				"message": "Missing authorization header",
			})
		}

		parts := strings.SplitN(authHeader, " ", 2)

		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return c.Status(401).JSON(fiber.Map{
				"message": "Invalid authorization header format",
			})
		}

		jwtStr := strings.TrimSpace(parts[1])

		if jwtStr == "" {
			return c.Status(401).JSON(fiber.Map{
				"message": "Missing token",
			})
		}

		claims, err := JwtParseAndValidate(jwtStr)

		if err != nil {
			if errors.Is(err, jwt.ErrTokenExpired) {
				return c.Status(401).JSON(fiber.Map{
					"message": "Token expired",
				})
			}
			return c.Status(401).JSON(fiber.Map{
				"message": "Invalid token",
			})
		}

		if claims.Type != "access" {
			return c.Status(401).JSON(fiber.Map{
				"message": "invalid token type",
			})
		}

		c.Locals("userId", claims.UserID)
		c.Locals("email", claims.Email)
		c.Locals("sessionId", claims.SessionId)

		return c.Next()
	}
}

func ValidateRefreshToken(refreshToken string) (res tokens) {

	now := time.Now()
	claims, err := JwtParseAndValidate(refreshToken)
	session := new(Session)

	if err != nil {
		res.err = err
		return
	}

	if claims.Type != "refresh" {
		res.err = jwt.ErrInvalidType
		return
	}

	if tx := database.FetchSessionByUserId(claims.UserID, &Session{}, session); tx.Error != nil {
		res.err = tx.Error
		return
	}

	if claims.SessionId != session.ID {
		res.err = jwt.ErrTokenInvalidId
		return
	}

	h := sha256.New()
	h.Write([]byte(refreshToken))
	hashedToken := hex.EncodeToString(h.Sum(nil))

	if hashedToken != session.TokenHash {
		res.err = jwt.ErrTokenInvalidId
		return
	}

	if claims.ExpiresAt.Before(now.Add(84 * time.Hour)) {
		res = GenerateRefreshToken(claims.UserID, claims.Email)
		return
	}

	accessToken, accessErr := GenerateAccessToken(claims.UserID, claims.Email, claims.SessionId)

	if accessErr != nil {
		res.err = accessErr
		return
	}

	res.accessToken = accessToken
	res.sessionId = claims.SessionId

	return

}

func LoginIPRateLimiter() fiber.Handler {

	return limiter.New(limiter.Config{
		Max:        5,
		Expiration: 1 * time.Minute,
		KeyGenerator: func(c fiber.Ctx) string {
			return c.IP()
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{
				"message": "Too Many Requests",
				"error":   "Too many login attempts. Please wait a while before trying again.",
			})
		},
	},
	)
}

func EmailRateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:                    10,
		Expiration:             15 * time.Minute,
		SkipSuccessfulRequests: true,
		KeyGenerator: func(c fiber.Ctx) string {
			var data struct {
				Email string `json:"email"`
			}

			_ = c.Bind().Body(&data)

			if data.Email == "" || data.Email == " " {
				c.Next()
			}

			data.Email = strings.TrimSpace(strings.ToLower(data.Email))

			return data.Email
		},
		LimitReached: func(c fiber.Ctx) error {
			return c.Status(429).JSON(fiber.Map{
				"message": "Too Many Requests",
				"error":   "Too many login attempts. Please wait 15 minutes before trying again.",
			})
		},
	})
}
