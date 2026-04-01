package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/BarisNKorkmaz/taskManager/database"
	"github.com/BarisNKorkmaz/taskManager/middleware"
	"github.com/BarisNKorkmaz/taskManager/utils"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v3"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func RegisterHandler(c fiber.Ctx) error {
	data := new(RegisterDTO)
	if err := c.Bind().Body(data); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   err.Error(),
		})
	}

	data.Timezone = strings.TrimSpace(data.Timezone)

	if err := utils.Validate.Struct(data); err != nil {
		var messages []string
		valErrs := err.(validator.ValidationErrors)

		for _, valErr := range valErrs {
			messages = append(messages, fmt.Sprintf("Field: %s, failed on: %s, on your value: %s", valErr.Field(), valErr.Tag(), valErr.Value()))
		}

		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   messages,
		})
	}

	hashedPass, passHashErr := bcrypt.GenerateFromPassword([]byte(data.Password), bcrypt.DefaultCost)

	if passHashErr != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   passHashErr.Error(),
		})
	}

	loc, err := time.LoadLocation(data.Timezone)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid timezone",
		})
	}

	user := User{
		Name:         data.Name,
		Surname:      data.Surname,
		PasswordHash: string(hashedPass),
		Email:        data.Email,
		LastLoginAt:  time.Now(),
		Timezone:     loc.String(),
	}

	tx := database.Create(database.DB, &user, &User{})
	if tx.Error != nil {
		if strings.Contains(tx.Error.Error(), "SQLSTATE 23505") {
			return c.Status(409).JSON(fiber.Map{
				"message": "Email already used",
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	tokens := GenerateRefreshToken(user.UserID, user.Email)

	if tokens.err != nil {
		return c.Status(201).JSON(fiber.Map{
			"message":       "User successfully created. Please login.",
			"requiresLogin": true,
		})
	}

	cookie := SetCookieHelper("refresh_token", tokens.refreshToken, time.Now().Add(168*time.Hour))
	c.Cookie(&cookie)

	return c.Status(201).JSON(fiber.Map{
		"message":     "User successfully created",
		"accessToken": tokens.accessToken,
	})

}

func LoginHandler(c fiber.Ctx) error {
	data := new(LoginDTO)

	if err := c.Bind().Body(data); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   err.Error(),
		})
	}

	if errs := utils.Validate.Struct(data); errs != nil {
		var messages []string
		var valErrs = errs.(validator.ValidationErrors)

		for _, valErr := range valErrs {
			messages = append(messages, fmt.Sprintf("Field: %s, failed on: %s, on your value: %s", valErr.Field(), valErr.Tag(), valErr.Value()))
		}

		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   messages,
		})
	}

	user := new(User)

	if tx := database.FetchUserByEmail(data.Email, user); tx.Error != nil {

		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return c.Status(401).JSON(fiber.Map{
				"message": "Wrong password or email",
			})
		}

		middleware.Log.Error("failed on fetch user operation:", "err", tx.Error)
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	if !user.IsActive && user.Email != "" {
		return c.Status(401).JSON(fiber.Map{
			"message": "User account is deactivated",
		})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(data.Password)); err != nil {
		return c.Status(401).JSON(fiber.Map{
			"message": "Wrong password or email",
		})
	}

	tokens := GenerateRefreshToken(user.UserID, user.Email)

	if tokens.err != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tokens.err.Error(),
		})
	}

	cookie := SetCookieHelper("refresh_token", tokens.refreshToken, time.Now().Add(168*time.Hour))
	c.Cookie(&cookie)

	dbErr := database.UpdateLastLogin(&User{}, "user_id = ?", user.UserID)
	if dbErr.Error != nil {
		middleware.Log.Error("failed on update last login time operation:", "err", dbErr.Error, "userID", user.UserID)
	}

	return c.Status(200).JSON(fiber.Map{
		"message":     "Successfully logged in",
		"accessToken": tokens.accessToken,
	})
}

func MeHandler(c fiber.Ctx) error {

	uid := c.Locals("userId").(uint)
	user := new(User)
	if tx := database.FetchUserByUID(uid, user); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	me := MeDTO{
		UserID:      user.UserID,
		Name:        user.Name,
		Surname:     user.Surname,
		Email:       user.Email,
		IsActive:    user.IsActive,
		LastLoginAt: user.LastLoginAt,
		CreatedAt:   user.CreatedAt,
	}

	return c.Status(200).JSON(fiber.Map{
		"message": "User successfully fetched",
		"user":    me,
	})
}

func RefreshHandler(c fiber.Ctx) error {
	refreshToken := c.Cookies("refresh_token")

	if refreshToken == "" {
		return c.Status(401).JSON(fiber.Map{
			"message": "Unauthorized",
			"error":   "Missing refresh token",
		})
	}

	res := ValidateRefreshToken(refreshToken)

	if res.err != nil {
		c.ClearCookie("refresh_token")
		return c.Status(401).JSON(fiber.Map{
			"message": "Unauthorized",
			"error":   res.err,
		})
	}

	if res.refreshToken != "" {
		cookie := SetCookieHelper("refresh_token", res.refreshToken, time.Now().Add(168*time.Hour))
		c.Cookie(&cookie)
	}

	return c.Status(200).JSON(fiber.Map{
		"message":     "Token refreshed successfully",
		"accessToken": res.accessToken,
	})

}

func LogoutHandler(c fiber.Ctx) error {
	refreshToken := c.Cookies("refresh_token")
	if refreshToken == "" {
		return c.Status(401).JSON(fiber.Map{
			"message": "Unauthorized",
			"error":   "Missing refresh token",
		})
	}

	uid := c.Locals("userId").(uint)

	if tx := database.DeleteSessionByUserId(database.DB, uid, &Session{}); tx.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error,
		})
	}

	cookie := SetCookieHelper("refresh_token", "", time.Now().Add(-1*time.Hour))
	c.Cookie(&cookie)

	return c.Status(200).JSON(fiber.Map{
		"message": "Successfully logged out",
	})
}

func SetCookieHelper(name string, value string, expireTime time.Time) fiber.Cookie {
	return fiber.Cookie{
		Name:     name,
		Value:    value,
		HTTPOnly: true,
		Secure:   false,
		SameSite: "Lax",
		Path:     "/",
		Expires:  expireTime,
	}
}
