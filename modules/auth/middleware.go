package auth

import (
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
)

func JWTMiddleware() fiber.Handler {
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
				"message": "Missing authorization header format",
			})
		}

		JWTstr := strings.TrimSpace(parts[1])

		if JWTstr == "" {
			return c.Status(401).JSON(fiber.Map{
				"message": "Missing token",
			})
		}

		claims, err := JwtParseAndValidate(JWTstr)

		if err != nil {
			if strings.Contains(err.Error(), jwt.ErrTokenExpired.Error()) {
				return c.Status(401).JSON(fiber.Map{
					"message": "Token expired",
				})
			}
			return c.Status(401).JSON(fiber.Map{
				"message": "Invalid token",
			})
		}

		c.Locals("userID", claims.UserID)
		c.Locals("email", claims.Email)

		return c.Next()
	}
}
