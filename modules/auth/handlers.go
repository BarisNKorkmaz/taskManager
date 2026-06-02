package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
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

type deviceTokenInterface struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	SessionID string `gorm:"not null;index"`
	IsActive  bool   `gorm:"not null;default:true"`
}

func (deviceTokenInterface) TableName() string { return "device_tokens" }

func RegisterHandler(c fiber.Ctx) error {
	data := new(RegisterDTO)
	now := time.Now()
	if err := c.Bind().Body(data); err != nil {
		middleware.Log.Info("Body parsing error in register handler", "err", err.Error())
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   err.Error(),
		})
	}

	data.Timezone = strings.TrimSpace(data.Timezone)
	data.Email = strings.ToLower(strings.TrimSpace(data.Email))

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
		middleware.Log.Error("error on hashing password", "err", passHashErr)
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
		Name:          data.Name,
		Surname:       data.Surname,
		PasswordHash:  string(hashedPass),
		Email:         data.Email,
		LastLoginAt:   now,
		Timezone:      loc.String(),
		PassChangedAt: now,
	}

	tx := database.Create(database.DB, &user, &User{})
	if tx.Error != nil {
		if strings.Contains(tx.Error.Error(), "SQLSTATE 23505") {
			return c.Status(409).JSON(fiber.Map{
				"message": "Email already used",
			})
		}
		middleware.Log.Error("error on creating user", "err", tx.Error)
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	tokens := GenerateRefreshToken(user.UserID, user.Email, c.IP())

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
		middleware.Log.Info("Body parsing error", "err", err.Error())
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   err.Error(),
		})
	}

	data.Email = strings.ToLower(strings.TrimSpace(data.Email))

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

	tokens := GenerateRefreshToken(user.UserID, user.Email, c.IP())

	if tokens.err != nil {
		middleware.Log.Error("error on generate refresh token", "err", tokens.err.Error())
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tokens.err.Error(),
		})
	}

	cookie := SetCookieHelper("refresh_token", tokens.refreshToken, time.Now().Add(168*time.Hour))
	c.Cookie(&cookie)

	dbErr := database.UpdateLastLogin(&User{}, "user_id = ?", user.UserID)
	if dbErr.Error != nil {
		middleware.Log.Warn("failed on update last login time operation:", "err", dbErr.Error, "userID", user.UserID)
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
		middleware.Log.Error("error on fetch user wtih userId", "err", tx.Error.Error(), "userId", uid)
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
		Timezone:    user.Timezone,
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

	res := ValidateRefreshToken(refreshToken, c.IP())

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
	sessionId := c.Locals("sessionId").(string)
	uid := c.Locals("userId").(uint)
	if refreshToken == "" {
		return c.Status(401).JSON(fiber.Map{
			"message": "Unauthorized",
			"error":   "Missing refresh token",
		})
	}

	if tx := database.DeactivateDeviceTokenBySessionId(sessionId, &deviceTokenInterface{}); tx.Error != nil {
		middleware.Log.Error("error on deactivate device token with session ID", "err", tx.Error.Error(), "sessionId", sessionId, "userId", uid)
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	if tx := database.DeleteSessionByUserId(database.DB, uid, &Session{}); tx.Error != nil {
		middleware.Log.Error("error on delete session with userId", "err", tx.Error.Error(), "userId", uid)
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

func ForgotPasswordHandler(c fiber.Ctx) error { // TODO rate limit eklenecek
	data := new(ForgotPassDTO)
	user := new(User)
	now := time.Now()

	if err := c.Bind().Body(data); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   err.Error(),
		})
	}

	data.Email = strings.ToLower(strings.TrimSpace(data.Email))

	if err := utils.Validate.Struct(data); err != nil {
		var messages []string
		valErrs := err.(validator.ValidationErrors)

		for _, valErr := range valErrs {
			messages = append(messages, fmt.Sprintf("Field: %s, Failed on: %s On your value: %v", valErr.Field(), valErr.Tag(), valErr.Value()))
		}

		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   messages,
		})
	}

	if tx := database.FetchUserByEmail(data.Email, user); tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return c.Status(200).JSON(fiber.Map{
				"message": "If the email exists, a password reset link has been sent.",
			})
		}
		middleware.Log.Error("error on fetch user with email", "err", tx.Error.Error())
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	token, hashedToken, generateErr := utils.GeneratePassResetToken()

	if generateErr != nil {
		middleware.Log.Error("error on generate pass reset token", "err", generateErr)
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   generateErr.Error(),
		})
	}

	resetToken := PasswordResetToken{
		UserID:    user.UserID,
		CreatedAt: now,
		ExpiresAt: now.Add(15 * time.Minute),
		TokenHash: hashedToken,
	}

	atomicDb := database.DB.Begin()
	if atomicDb.Error != nil {
		middleware.Log.Error("error on starting atomic transaction in forgot pass handler", "err", atomicDb.Error)
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   atomicDb.Error.Error(),
		})
	}

	if tx := database.DeletePassResetTokenByUserId(atomicDb, user.UserID, &PasswordResetToken{}); tx.Error != nil {
		atomicDb.Rollback()
		middleware.Log.Error("error on delete pass reset token with userId", "err", tx.Error.Error(), "userId", user.UserID)

		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	if tx := database.Create(atomicDb, &resetToken, &PasswordResetToken{}); tx.Error != nil {
		atomicDb.Rollback()
		middleware.Log.Error("error on create pass reset token on db", "err", tx.Error.Error(), "token", resetToken)
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	if tx := atomicDb.Commit(); tx.Error != nil {
		atomicDb.Rollback()
		middleware.Log.Error("error on commiting atomic transaction in forgot pass handler", "err", tx.Error.Error())
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	mailService, err := utils.LoadMailConfig()

	if err != nil {
		middleware.Log.Error("error on set mail config", "err", err)
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   err.Error(),
		})
	}
	frontendDomain := os.Getenv("FRONTEND_URL")
	if frontendDomain == "" {
		middleware.Log.Error("error on read FRONTEND_URL from env file in forgot pass handler")
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   "FRONTEND_URL env not set",
		})
	}
	resetUrl := fmt.Sprintf("sevenplanner://reset-password?token=%s", token)
	uid := user.UserID
	userMail := user.Email
	go func() {
		if err := mailService.SendForgotPasswordEmail(userMail, resetUrl); err != nil {

			middleware.Log.Error("Failed to send reset email",
				"email", userMail,
				"error", err.Error(),
			)

			if tx := database.DeletePassResetTokenByUserId(database.DB, uid, &PasswordResetToken{}); tx.Error != nil {
				middleware.Log.Error("Failed to delete passResetToken",
					"tokenid", resetToken.ID,
					"err", tx.Error.Error())
			}
		}
	}()

	return c.Status(200).JSON(fiber.Map{
		"message": "If the email exists, a password reset link has been sent.",
	})
}

func ResetPasswordHandler(c fiber.Ctx) error {
	data := new(ResetPassDTO)
	resetToken := new(PasswordResetToken)
	user := new(User)
	now := time.Now()

	if err := c.Bind().Body(data); err != nil {
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   err.Error(),
		})
	}

	if err := utils.Validate.Struct(data); err != nil {
		var messages []string
		valErrs := err.(validator.ValidationErrors)

		for _, valErr := range valErrs {
			messages = append(messages, fmt.Sprintf("Field: %s, Failed on: %s, On your value: %v", valErr.Field(), valErr.Tag(), valErr.Value()))
		}
		return c.Status(400).JSON(fiber.Map{
			"message": "Bad request",
			"error":   messages,
		})
	}

	h := sha256.New()
	h.Write([]byte(data.Token))
	hashedToken := hex.EncodeToString(h.Sum(nil))

	if tx := database.FetchPassResetTokenByToken(hashedToken, &PasswordResetToken{}, resetToken); tx.Error != nil {
		if errors.Is(tx.Error, gorm.ErrRecordNotFound) {
			return c.Status(400).JSON(fiber.Map{
				"message": "Invalid or expired token",
			})
		}
		middleware.Log.Error("error on fetch pass reset token with token", "err", tx.Error.Error())
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	if resetToken.ExpiresAt.Before(now) {
		return c.Status(400).JSON(fiber.Map{
			"message": "Invalid or expired token",
		})
	}

	if tx := database.FetchUserByUID(resetToken.UserID, user); tx.Error != nil {
		middleware.Log.Error("error on fetch user with userId in reset pass handler", "err", tx.Error.Error(), "userId", resetToken.UserID)
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	updates := make(map[string]any)

	if hashedNewPass, err := bcrypt.GenerateFromPassword([]byte(data.NewPassword), bcrypt.DefaultCost); err != nil {
		middleware.Log.Error("error on hashing pass in reset pass handler", "err", err)
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   err.Error(),
		})
	} else {
		updates["password_hash"] = string(hashedNewPass)
		updates["pass_changed_at"] = now
	}

	atomicDb := database.DB.Begin()
	if atomicDb.Error != nil {
		middleware.Log.Error("error on starting atomic transaction in reset pass handler", "err", atomicDb.Error.Error())
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   atomicDb.Error.Error(),
		})
	}

	if tx := database.UpdateUserPass(atomicDb, &User{}, user.UserID, updates); tx.Error != nil {
		atomicDb.Rollback()
		middleware.Log.Error("error on update user pass in reset pass handler", "err", tx.Error.Error())
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	if tx := database.DeleteSessionByUserId(atomicDb, user.UserID, &Session{}); tx.Error != nil {
		atomicDb.Rollback()
		middleware.Log.Error("error on delete sessin with userId in reset pass handler", "err", tx.Error.Error())
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	if tx := database.DeletePassResetToken(atomicDb, resetToken.ID, &PasswordResetToken{}); tx.Error != nil {
		atomicDb.Rollback()
		middleware.Log.Error("error on delete pass reset token in reset pass handler", "err", tx.Error.Error(), "userId", resetToken.UserID)
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	if tx := atomicDb.Commit(); tx.Error != nil {
		atomicDb.Rollback()
		middleware.Log.Error("error on commiting atomic transaction in reset pass handler", "err", tx.Error.Error())
		return c.Status(500).JSON(fiber.Map{
			"message": "Server error",
			"error":   tx.Error.Error(),
		})
	}

	return c.Status(200).JSON(fiber.Map{
		"message": "Password successfully changed",
	})

}
